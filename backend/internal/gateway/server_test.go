package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestProfileGatewaySwitchDrainsOldConnections(t *testing.T) {
	serverA := startTaggedEchoServer(t, "A:")
	serverB := startTaggedEchoServer(t, "B:")
	var releasedA atomic.Int32
	gateway, err := StartProfileGateway("profile-a", routeTo(serverA, "route-a", func() { releasedA.Add(1) }), RoutingConfig{Mode: ModeProxy})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })

	oldConn := dialThroughGateway(t, gateway.ProxyURL(), "example.com", 443)
	assertEcho(t, oldConn, "one", "A:one")
	if _, err := gateway.Switch(routeTo(serverB, "route-b", nil), RoutingConfig{Mode: ModeProxy}, false); err != nil {
		t.Fatal(err)
	}
	assertEcho(t, oldConn, "two", "A:two")
	newConn := dialThroughGateway(t, gateway.ProxyURL(), "example.com", 443)
	assertEcho(t, newConn, "three", "B:three")
	_ = newConn.Close()
	if releasedA.Load() != 0 {
		t.Fatal("old route released while an old connection was still active")
	}
	_ = oldConn.Close()
	waitFor(t, func() bool { return releasedA.Load() == 1 })
}

func TestProfileGatewayForceSwitchClosesOldConnections(t *testing.T) {
	serverA := startTaggedEchoServer(t, "A:")
	serverB := startTaggedEchoServer(t, "B:")
	gateway, err := StartProfileGateway("profile-a", routeTo(serverA, "route-a", nil), RoutingConfig{Mode: ModeProxy})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	oldConn := dialThroughGateway(t, gateway.ProxyURL(), "example.com", 443)
	assertEcho(t, oldConn, "one", "A:one")
	if _, err := gateway.Switch(routeTo(serverB, "route-b", nil), RoutingConfig{Mode: ModeProxy}, true); err != nil {
		t.Fatal(err)
	}
	_ = oldConn.SetDeadline(time.Now().Add(time.Second))
	if _, err := oldConn.Write([]byte("closed")); err == nil {
		buffer := make([]byte, 32)
		if _, readErr := oldConn.Read(buffer); readErr == nil {
			t.Fatal("old connection remained usable after force switch")
		}
	}
}

func TestProfileGatewayForceSwitchRejectsInFlightOldDial(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	oldRoute := RouteSpec{ID: "route-a", Dial: func(ctx context.Context, network string, address string) (net.Conn, error) {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	target := startTaggedEchoServer(t, "A:")
	gateway, err := StartProfileGateway("profile-a", oldRoute, RoutingConfig{Mode: ModeProxy})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })

	result := make(chan byte, 1)
	go func() {
		result <- gatewayConnectReplyCode(gateway.ProxyURL(), target)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("old route dial did not start")
	}

	serverB := startTaggedEchoServer(t, "B:")
	if _, err := gateway.Switch(routeTo(serverB, "route-b", nil), RoutingConfig{Mode: ModeProxy}, true); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case code := <-result:
		if code == 0x00 {
			t.Fatal("in-flight old dial survived force switch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight old dial was not closed")
	}
}

func TestProfileGatewayFailsClosed(t *testing.T) {
	failing := RouteSpec{ID: "broken", Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("unavailable")
	}}
	gateway, err := StartProfileGateway("profile-a", failing, RoutingConfig{Mode: ModeProxy})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	proxyURL, _ := url.Parse(gateway.ProxyURL())
	conn, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil {
		t.Fatal(err)
	}
	request, _ := appendSOCKSAddress([]byte{0x05, 0x01, 0x00}, "127.0.0.1", 9)
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if reply[1] == 0x00 {
		t.Fatal("failed proxy route unexpectedly fell back to direct")
	}
}

func TestProfileGatewayDirectRule(t *testing.T) {
	target := startTaggedEchoServer(t, "D:")
	failing := RouteSpec{ID: "broken", Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("proxy route must not be used")
	}}
	gateway, err := StartProfileGateway("profile-a", failing, RoutingConfig{Mode: ModeRule, Rules: []Rule{{Enabled: true, MatchType: MatchDomain, Pattern: "127.0.0.1", Action: ActionDirect}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	_, portText, _ := net.SplitHostPort(target)
	port, _ := strconv.Atoi(portText)
	conn := dialThroughGateway(t, gateway.ProxyURL(), "127.0.0.1", port)
	defer conn.Close()
	assertEcho(t, conn, "hello", "D:hello")
}

func routeTo(address string, id string, release func()) RouteSpec {
	return RouteSpec{ID: id, Release: release, Dial: func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
}

func startTaggedEchoServer(t *testing.T, prefix string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buffer := make([]byte, 1024)
				for {
					n, err := conn.Read(buffer)
					if n > 0 {
						_, _ = conn.Write(append([]byte(prefix), buffer[:n]...))
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func dialThroughGateway(t *testing.T, proxyRaw string, host string, port int) net.Conn {
	t.Helper()
	proxyURL, err := url.Parse(proxyRaw)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil || method[1] != 0x00 {
		t.Fatalf("SOCKS method negotiation failed: %v, %v", method, err)
	}
	request, err := appendSOCKSAddress([]byte{0x05, 0x01, 0x00}, host, port)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if reply[1] != 0x00 {
		t.Fatalf("SOCKS CONNECT failed with code %d", reply[1])
	}
	if err := discardSOCKSAddress(conn, reply[3]); err != nil {
		t.Fatal(err)
	}
	return conn
}

func gatewayConnectReplyCode(proxyRaw string, target string) byte {
	proxyURL, err := url.Parse(proxyRaw)
	if err != nil {
		return 0xff
	}
	conn, err := net.DialTimeout("tcp", proxyURL.Host, time.Second)
	if err != nil {
		return 0xff
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return 0xff
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil || method[1] != 0x00 {
		return 0xff
	}
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return 0xff
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0xff
	}
	request, err := appendSOCKSAddress([]byte{0x05, 0x01, 0x00}, host, port)
	if err != nil {
		return 0xff
	}
	if _, err := conn.Write(request); err != nil {
		return 0xff
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return 0xff
	}
	return reply[1]
}

func assertEcho(t *testing.T, conn net.Conn, input string, want string) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(want))
	if _, err := io.ReadFull(conn, buffer); err != nil {
		t.Fatal(err)
	}
	if string(buffer) != want {
		t.Fatalf("echo = %q, want %q", buffer, want)
	}
	_ = conn.SetDeadline(time.Time{})
}

func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not reached")
}
