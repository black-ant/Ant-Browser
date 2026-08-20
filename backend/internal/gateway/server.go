package gateway

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const handshakeTimeout = 15 * time.Second

type routeState struct {
	spec     RouteSpec
	active   int
	retired  bool
	released bool
}

type tunnel struct {
	generation uint64
	route      *routeState
	client     net.Conn
	upstream   net.Conn
	closeOnce  sync.Once
}

func (t *tunnel) close() {
	t.closeOnce.Do(func() {
		if t.client != nil {
			_ = t.client.Close()
		}
		if t.upstream != nil {
			_ = t.upstream.Close()
		}
	})
}

type ProfileGateway struct {
	profileID string
	listener  net.Listener
	proxyURL  string

	mu          sync.Mutex
	routing     RoutingConfig
	current     *routeState
	routes      map[*routeState]struct{}
	tunnels     map[*tunnel]struct{}
	generation  uint64
	forceCutoff uint64
	closed      bool
	closeOnce   sync.Once
	done        chan struct{}
}

func StartProfileGateway(profileID string, route RouteSpec, routing RoutingConfig) (*ProfileGateway, error) {
	if route.Dial == nil {
		return nil, fmt.Errorf("gateway route dialer is required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	state := &routeState{spec: route}
	gateway := &ProfileGateway{
		profileID:  profileID,
		listener:   listener,
		proxyURL:   "socks5://" + listener.Addr().String(),
		routing:    NormalizeRoutingConfig(routing),
		current:    state,
		routes:     map[*routeState]struct{}{state: {}},
		tunnels:    make(map[*tunnel]struct{}),
		generation: 1,
		done:       make(chan struct{}),
	}
	go gateway.acceptLoop()
	return gateway, nil
}

func (g *ProfileGateway) ProxyURL() string {
	if g == nil {
		return ""
	}
	return g.proxyURL
}

func (g *ProfileGateway) Done() <-chan struct{} {
	if g == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return g.done
}

func (g *ProfileGateway) Switch(route RouteSpec, routing RoutingConfig, force bool) (Status, error) {
	if g == nil || route.Dial == nil {
		return Status{}, fmt.Errorf("gateway route dialer is required")
	}
	newRoute := &routeState{spec: route}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return Status{}, fmt.Errorf("gateway is closed")
	}
	oldRoute := g.current
	g.generation++
	if force {
		g.forceCutoff = g.generation
	}
	g.current = newRoute
	g.routes[newRoute] = struct{}{}
	g.routing = NormalizeRoutingConfig(routing)
	if oldRoute != nil {
		oldRoute.retired = true
	}
	toClose := g.oldTunnelsLocked(force)
	toRelease := g.collectReleasableRoutesLocked()
	status := g.statusLocked()
	g.mu.Unlock()
	closeTunnels(toClose)
	releaseRoutes(toRelease)
	return status, nil
}

func (g *ProfileGateway) UpdateRouting(routing RoutingConfig, force bool) (Status, error) {
	if g == nil {
		return Status{}, fmt.Errorf("gateway is unavailable")
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return Status{}, fmt.Errorf("gateway is closed")
	}
	g.generation++
	if force {
		g.forceCutoff = g.generation
	}
	g.routing = NormalizeRoutingConfig(routing)
	toClose := g.oldTunnelsLocked(force)
	status := g.statusLocked()
	g.mu.Unlock()
	closeTunnels(toClose)
	return status, nil
}

func (g *ProfileGateway) Status() Status {
	if g == nil {
		return Status{}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.statusLocked()
}

func (g *ProfileGateway) statusLocked() Status {
	status := Status{ProfileID: g.profileID, ProxyURL: g.proxyURL, Mode: g.routing.Mode}
	if g.current != nil {
		status.CurrentRouteID = g.current.spec.ID
	}
	for item := range g.tunnels {
		if item.generation < g.generation {
			status.DrainingConnections++
		} else {
			status.ActiveConnections++
		}
	}
	return status
}

func (g *ProfileGateway) Close() error {
	if g == nil {
		return nil
	}
	var closeErr error
	g.closeOnce.Do(func() {
		closeErr = g.listener.Close()
		g.mu.Lock()
		g.closed = true
		toClose := make([]*tunnel, 0, len(g.tunnels))
		for item := range g.tunnels {
			toClose = append(toClose, item)
		}
		toRelease := make([]*routeState, 0, len(g.routes))
		for route := range g.routes {
			if !route.released {
				route.released = true
				toRelease = append(toRelease, route)
			}
		}
		g.mu.Unlock()
		closeTunnels(toClose)
		releaseRoutes(toRelease)
		close(g.done)
	})
	return closeErr
}

func (g *ProfileGateway) acceptLoop() {
	for {
		conn, err := g.listener.Accept()
		if err != nil {
			return
		}
		go g.handleSOCKS5(conn)
	}
}

func (g *ProfileGateway) handleSOCKS5(client net.Conn) {
	_ = client.SetDeadline(time.Now().Add(handshakeTimeout))
	host, port, err := readSOCKS5Request(client)
	if err != nil {
		_ = client.Close()
		return
	}
	target := net.JoinHostPort(host, strconv.Itoa(port))

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		_ = writeSOCKS5Reply(client, 0x01, nil)
		_ = client.Close()
		return
	}
	action := ResolveAction(g.routing, host)
	generation := g.generation
	var route *routeState
	var dial DialContextFunc
	switch action {
	case ActionBlock:
		g.mu.Unlock()
		_ = writeSOCKS5Reply(client, 0x02, nil)
		_ = client.Close()
		return
	case ActionDirect:
		dial = DirectDialer()
	default:
		route = g.current
		if route != nil && !route.released {
			route.active++
			dial = route.spec.Dial
		}
	}
	g.mu.Unlock()

	if dial == nil {
		g.releaseRouteConnection(route)
		_ = writeSOCKS5Reply(client, 0x01, nil)
		_ = client.Close()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), upstreamDialTimeout)
	upstream, err := dial(ctx, "tcp", target)
	cancel()
	if err != nil {
		g.releaseRouteConnection(route)
		_ = writeSOCKS5Reply(client, 0x05, nil)
		_ = client.Close()
		return
	}
	item := &tunnel{generation: generation, route: route, client: client, upstream: upstream}
	g.mu.Lock()
	if g.closed || generation < g.forceCutoff {
		g.mu.Unlock()
		item.close()
		g.releaseRouteConnection(route)
		return
	}
	g.tunnels[item] = struct{}{}
	g.mu.Unlock()

	_ = client.SetDeadline(time.Time{})
	if err := writeSOCKS5Reply(client, 0x00, upstream.LocalAddr()); err != nil {
		item.close()
		g.finishTunnel(item)
		return
	}
	relayConnections(client, upstream)
	item.close()
	g.finishTunnel(item)
}

func (g *ProfileGateway) finishTunnel(item *tunnel) {
	if item == nil {
		return
	}
	g.mu.Lock()
	delete(g.tunnels, item)
	if item.route != nil && item.route.active > 0 {
		item.route.active--
	}
	toRelease := g.collectReleasableRoutesLocked()
	g.mu.Unlock()
	releaseRoutes(toRelease)
}

func (g *ProfileGateway) releaseRouteConnection(route *routeState) {
	if route == nil {
		return
	}
	g.mu.Lock()
	if route.active > 0 {
		route.active--
	}
	toRelease := g.collectReleasableRoutesLocked()
	g.mu.Unlock()
	releaseRoutes(toRelease)
}

func (g *ProfileGateway) oldTunnelsLocked(force bool) []*tunnel {
	if !force {
		return nil
	}
	items := make([]*tunnel, 0)
	for item := range g.tunnels {
		if item.generation < g.generation {
			items = append(items, item)
		}
	}
	return items
}

func (g *ProfileGateway) collectReleasableRoutesLocked() []*routeState {
	items := make([]*routeState, 0)
	for route := range g.routes {
		if route.retired && route.active == 0 && !route.released {
			route.released = true
			delete(g.routes, route)
			items = append(items, route)
		}
	}
	return items
}

func closeTunnels(items []*tunnel) {
	for _, item := range items {
		item.close()
	}
}

func releaseRoutes(items []*routeState) {
	for _, route := range items {
		if route != nil && route.spec.Release != nil {
			route.spec.Release()
		}
	}
}

func readSOCKS5Request(conn net.Conn) (string, int, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", 0, err
	}
	if header[0] != 0x05 || header[1] == 0 {
		return "", 0, fmt.Errorf("invalid SOCKS greeting")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", 0, err
	}
	supportsNoAuth := false
	for _, method := range methods {
		if method == 0x00 {
			supportsNoAuth = true
			break
		}
	}
	if !supportsNoAuth {
		_, _ = conn.Write([]byte{0x05, 0xff})
		return "", 0, fmt.Errorf("SOCKS client does not support no-auth")
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return "", 0, err
	}
	request := make([]byte, 4)
	if _, err := io.ReadFull(conn, request); err != nil {
		return "", 0, err
	}
	if request[0] != 0x05 || request[1] != 0x01 {
		_ = writeSOCKS5Reply(conn, 0x07, nil)
		return "", 0, fmt.Errorf("SOCKS command is not CONNECT")
	}
	host, err := readSOCKSHost(conn, request[3])
	if err != nil {
		return "", 0, err
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", 0, err
	}
	port := int(binary.BigEndian.Uint16(portBytes))
	if port == 0 {
		return "", 0, fmt.Errorf("invalid SOCKS target port")
	}
	return host, port, nil
}

func readSOCKSHost(reader io.Reader, addressType byte) (string, error) {
	switch addressType {
	case 0x01:
		value := make([]byte, 4)
		_, err := io.ReadFull(reader, value)
		return net.IP(value).String(), err
	case 0x04:
		value := make([]byte, 16)
		_, err := io.ReadFull(reader, value)
		return net.IP(value).String(), err
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(reader, length); err != nil {
			return "", err
		}
		if length[0] == 0 {
			return "", fmt.Errorf("empty SOCKS domain")
		}
		value := make([]byte, int(length[0]))
		_, err := io.ReadFull(reader, value)
		return string(value), err
	default:
		return "", fmt.Errorf("unsupported SOCKS address type")
	}
}

func writeSOCKS5Reply(conn net.Conn, code byte, address net.Addr) error {
	ip := net.IPv4zero
	port := 0
	if tcpAddress, ok := address.(*net.TCPAddr); ok && tcpAddress != nil {
		ip = tcpAddress.IP
		port = tcpAddress.Port
	}
	response := []byte{0x05, code, 0x00}
	if ipv4 := ip.To4(); ipv4 != nil {
		response = append(response, 0x01)
		response = append(response, ipv4...)
	} else {
		response = append(response, 0x04)
		response = append(response, ip.To16()...)
	}
	response = append(response, byte(port>>8), byte(port))
	_, err := conn.Write(response)
	return err
}

func relayConnections(client net.Conn, upstream net.Conn) {
	var wait sync.WaitGroup
	wait.Add(2)
	copyOneWay := func(dst net.Conn, src net.Conn) {
		defer wait.Done()
		_, _ = io.Copy(dst, src)
		if writer, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = writer.CloseWrite()
		} else {
			_ = dst.SetDeadline(time.Now().Add(2 * time.Second))
		}
	}
	go copyOneWay(upstream, client)
	go copyOneWay(client, upstream)
	wait.Wait()
}
