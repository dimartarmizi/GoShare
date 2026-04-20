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

	targets := discoveryTargets(s.DiscoveryPort)
	sent := false
	for _, target := range targets {
		if _, err := udpConn.WriteToUDP([]byte(discoveryRequest), target); err == nil {
			sent = true
		}
	}
	if !sent {
		return nil, fmt.Errorf("failed to send discovery broadcast")
	}

	devices := make(map[string]DeviceInfo)
	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return mapToSlice(devices), ctx.Err()
		default:
		}

		n, remoteAddr, err := udpConn.ReadFromUDP(buf)
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
		if remoteAddr != nil && remoteAddr.IP != nil && !remoteAddr.IP.IsLoopback() {
			if ip4 := remoteAddr.IP.To4(); ip4 != nil {
				info.IP = ip4.String()
			}
		}
		if isLocalIPv4(info.IP) && info.Port == s.ListenPort {
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
	rIP := remote.To4()
	var fallback string

	for _, ipNet := range getLocalIPNets(true) {
		ip := ipNet.IP.To4()
		if fallback == "" {
			fallback = ip.String()
		}
		if rIP != nil && ipNet.Contains(rIP) {
			return ip.String()
		}
	}

	if fallback != "" {
		return fallback
	}
	return "0.0.0.0"
}

func discoveryTargets(port int) []*net.UDPAddr {
	seen := map[string]bool{}
	var targets []*net.UDPAddr

	add := func(ip net.IP) {
		if ip4 := ip.To4(); ip4 != nil && !seen[ip4.String()] {
			seen[ip4.String()] = true
			targets = append(targets, &net.UDPAddr{IP: ip4, Port: port})
		}
	}

	add(net.IPv4bcast)
	for _, ipNet := range getLocalIPNets(true) {
		ip, mask := ipNet.IP.To4(), ipNet.Mask
		if len(mask) == 4 {
			add(net.IPv4(ip[0]|^mask[0], ip[1]|^mask[1], ip[2]|^mask[2], ip[3]|^mask[3]))
		}
	}
	return targets
}

func isLocalIPv4(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr)).To4()
	if ip == nil {
		return false
	}
	for _, ipNet := range getLocalIPNets(false) {
		if ipNet.IP.To4().Equal(ip) {
			return true
		}
	}
	return false
}

func getLocalIPNets(excludeLoopback bool) []*net.IPNet {
	var nets []*net.IPNet
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || (excludeLoopback && iface.Flags&net.FlagLoopback != 0) {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				nets = append(nets, ipNet)
			}
		}
	}
	return nets
}
