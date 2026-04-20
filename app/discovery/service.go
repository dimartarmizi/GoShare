package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"goshare/internal/utils"
)

const (
	discoveryRequest = "GOSHARE_DISCOVER"
	discoveryReply   = "GOSHARE_HERE "
)

type DeviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type Service struct {
	ID            string
	Name          string
	ListenPort    int
	DiscoveryPort int

	mu        sync.Mutex
	listening bool
}

func NewService(name string, listenPort, discoveryPort int) *Service {
	host, _ := os.Hostname()
	if strings.TrimSpace(name) == "" {
		name = host
	}
	if discoveryPort <= 0 {
		discoveryPort = 9999
	}
	return &Service{
		ID:            utils.NewID(),
		Name:          name,
		ListenPort:    listenPort,
		DiscoveryPort: discoveryPort,
	}
}

func (s *Service) StartResponder(ctx context.Context) error {
	s.mu.Lock()
	if s.listening {
		s.mu.Unlock()
		return nil
	}
	s.listening = true
	s.mu.Unlock()

	addr := &net.UDPAddr{IP: net.IPv4zero, Port: s.DiscoveryPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
		s.mu.Lock()
		s.listening = false
		s.mu.Unlock()
	}()

	go func() {
		buf := make([]byte, 2048)
		for {
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}

			message := strings.TrimSpace(string(buf[:n]))
			if message != discoveryRequest {
				continue
			}

			replyIP := pickLocalIPv4For(remote.IP)
			payload, _ := json.Marshal(DeviceInfo{
				ID:   s.ID,
				Name: s.Name,
				IP:   replyIP,
				Port: s.ListenPort,
			})
			_, _ = conn.WriteToUDP([]byte(discoveryReply+string(payload)), remote)
		}
	}()

	return nil
}

func (s *Service) Discover(ctx context.Context, timeout time.Duration) ([]DeviceInfo, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		return nil, fmt.Errorf("unexpected packet conn type")
	}

	if err := udpConn.SetWriteBuffer(64 * 1024); err != nil {
		return nil, err
	}
	if err := udpConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	target := &net.UDPAddr{IP: net.IPv4bcast, Port: s.DiscoveryPort}
	if _, err := udpConn.WriteToUDP([]byte(discoveryRequest), target); err != nil {
		return nil, err
	}

	devices := make(map[string]DeviceInfo)
	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return mapToSlice(devices), ctx.Err()
		default:
		}

		n, _, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return mapToSlice(devices), nil
			}
			return nil, err
		}

		line := strings.TrimSpace(string(buf[:n]))
		raw, err := expectPrefix(line, discoveryReply)
		if err != nil {
			continue
		}

		var info DeviceInfo
		if err := json.Unmarshal([]byte(raw), &info); err != nil {
			continue
		}
		if info.ID == "" {
			continue
		}
		if info.ID == s.ID {
			continue
		}
		if info.Port <= 0 {
			continue
		}
		devices[info.ID] = info
	}
}

func mapToSlice(m map[string]DeviceInfo) []DeviceInfo {
	out := make([]DeviceInfo, 0, len(m))
	for _, d := range m {
		out = append(out, d)
	}
	return out
}

func expectPrefix(line, prefix string) (string, error) {
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("prefix %s not found", prefix)
	}
	return strings.TrimPrefix(line, prefix), nil
}

func pickLocalIPv4For(remote net.IP) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "0.0.0.0"
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			if ipNet.Contains(remote) {
				return ip.String()
			}
		}
	}

	return "0.0.0.0"
}
