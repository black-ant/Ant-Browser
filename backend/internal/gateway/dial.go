package gateway

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const upstreamDialTimeout = 15 * time.Second

func DirectDialer() DialContextFunc {
	dialer := &net.Dialer{Timeout: upstreamDialTimeout, KeepAlive: 30 * time.Second}
	return dialer.DialContext
}

func URLDialer(raw string) (DialContextFunc, error) {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "direct://") {
		return DirectDialer(), nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse upstream proxy: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme == "socks" {
		scheme = "socks5"
	}
	if parsed.Hostname() == "" || parsed.Port() == "" {
		return nil, fmt.Errorf("upstream proxy must include host and port")
	}
	switch scheme {
	case "socks5":
		return socks5URLDialer(parsed), nil
	case "http", "https":
		return httpURLDialer(parsed, scheme == "https"), nil
	default:
		return nil, fmt.Errorf("unsupported upstream proxy scheme: %s", parsed.Scheme)
	}
}

func socks5URLDialer(proxyURL *url.URL) DialContextFunc {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		if network != "tcp" {
			return nil, fmt.Errorf("SOCKS upstream only supports TCP")
		}
		conn, err := (&net.Dialer{Timeout: upstreamDialTimeout, KeepAlive: 30 * time.Second}).DialContext(ctx, "tcp", proxyURL.Host)
		if err != nil {
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		} else {
			_ = conn.SetDeadline(time.Now().Add(upstreamDialTimeout))
		}
		if err := negotiateSOCKS5(conn, proxyURL, address); err != nil {
			_ = conn.Close()
			return nil, err
		}
		_ = conn.SetDeadline(time.Time{})
		return conn, nil
	}
}

func negotiateSOCKS5(conn net.Conn, proxyURL *url.URL, address string) error {
	username := ""
	password := ""
	if proxyURL.User != nil {
		username = proxyURL.User.Username()
		password, _ = proxyURL.User.Password()
	}
	methods := []byte{0x00}
	if username != "" {
		methods = append(methods, 0x02)
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		return err
	}
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}
	if response[0] != 0x05 || response[1] == 0xff {
		return fmt.Errorf("SOCKS upstream rejected authentication methods")
	}
	if response[1] == 0x02 {
		if len(username) > 255 || len(password) > 255 {
			return fmt.Errorf("SOCKS upstream credentials are too long")
		}
		auth := []byte{0x01, byte(len(username))}
		auth = append(auth, username...)
		auth = append(auth, byte(len(password)))
		auth = append(auth, password...)
		if _, err := conn.Write(auth); err != nil {
			return err
		}
		if _, err := io.ReadFull(conn, response); err != nil {
			return err
		}
		if response[1] != 0x00 {
			return fmt.Errorf("SOCKS upstream authentication failed")
		}
	} else if response[1] != 0x00 {
		return fmt.Errorf("SOCKS upstream selected unsupported authentication method")
	}

	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid target port")
	}
	request := []byte{0x05, 0x01, 0x00}
	request, err = appendSOCKSAddress(request, host, port)
	if err != nil {
		return err
	}
	if _, err := conn.Write(request); err != nil {
		return err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != 0x05 || header[1] != 0x00 {
		return fmt.Errorf("SOCKS upstream connect failed with code %d", header[1])
	}
	return discardSOCKSAddress(conn, header[3])
}

func httpURLDialer(proxyURL *url.URL, useTLS bool) DialContextFunc {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		if network != "tcp" {
			return nil, fmt.Errorf("HTTP upstream only supports TCP")
		}
		var conn net.Conn
		var err error
		dialer := &net.Dialer{Timeout: upstreamDialTimeout, KeepAlive: 30 * time.Second}
		if useTLS {
			tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: proxyURL.Hostname(), MinVersion: tls.VersionTLS12}}
			conn, err = tlsDialer.DialContext(ctx, "tcp", proxyURL.Host)
		} else {
			conn, err = dialer.DialContext(ctx, "tcp", proxyURL.Host)
		}
		if err != nil {
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		} else {
			_ = conn.SetDeadline(time.Now().Add(upstreamDialTimeout))
		}
		req := &http.Request{
			Method: http.MethodConnect,
			URL:    &url.URL{Opaque: address},
			Host:   address,
			Header: make(http.Header),
		}
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			credentials := proxyURL.User.Username() + ":" + password
			req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
		}
		if err := req.Write(conn); err != nil {
			_ = conn.Close()
			return nil, err
		}
		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, req)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP upstream CONNECT failed: %s", resp.Status)
		}
		_ = conn.SetDeadline(time.Time{})
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func appendSOCKSAddress(dst []byte, host string, port int) ([]byte, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			dst = append(dst, 0x01)
			dst = append(dst, ipv4...)
		} else {
			dst = append(dst, 0x04)
			dst = append(dst, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, fmt.Errorf("invalid target host")
		}
		dst = append(dst, 0x03, byte(len(host)))
		dst = append(dst, host...)
	}
	dst = append(dst, byte(port>>8), byte(port))
	return dst, nil
}

func discardSOCKSAddress(reader io.Reader, addressType byte) error {
	length := 0
	switch addressType {
	case 0x01:
		length = 4
	case 0x04:
		length = 16
	case 0x03:
		one := []byte{0}
		if _, err := io.ReadFull(reader, one); err != nil {
			return err
		}
		length = int(one[0])
	default:
		return fmt.Errorf("invalid SOCKS address type")
	}
	_, err := io.CopyN(io.Discard, reader, int64(length+2))
	return err
}
