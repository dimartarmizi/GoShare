package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"goshare/app/models"
	"goshare/app/utils"
)

const (
	defaultBroadcastInterval = 1 * time.Second
	defaultOfflineAfter      = 3 * time.Second
)

type Config struct {
	UDPPort           int
	TCPPort           int
	DeviceID          string
	DeviceName        string
	BroadcastInterval time.Duration
	OfflineAfter      time.Duration
}

type Service struct {
	cfg Config

	mu      sync.RWMutex
	devices map[string]models.Device

	conn      *net.UDPConn
	bufPool   sync.Pool
	onChanged func()
	cancel    context.CancelFunc

	wg sync.WaitGroup
}

func NewService(cfg Config, onChanged func()) (*Service, error) {
	if cfg.UDPPort == 0 {
		cfg.UDPPort = 9999
	}
	if cfg.TCPPort == 0 {
		cfg.TCPPort = 9000
	}
	if cfg.DeviceID == "" {
		return nil, errors.New("device id is required")
	}
	if cfg.DeviceName == "" {
		cfg.DeviceName = "GoShare Device"
	}
	if cfg.BroadcastInterval <= 0 {
		cfg.BroadcastInterval = defaultBroadcastInterval
	}
	if cfg.OfflineAfter <= 0 {
		cfg.OfflineAfter = defaultOfflineAfter
	}

	svc := &Service{
		cfg:       cfg,
		devices:   make(map[string]models.Device),
		onChanged: onChanged,
		bufPool: sync.Pool{
			New: func() any {
				buf := make([]byte, 2048)
				return &buf
			},
		},
	}
	return svc, nil
}

func (s *Service) Start(parentCtx context.Context) error {
	listenAddr := &net.UDPAddr{IP: net.IPv4zero, Port: s.cfg.UDPPort}
	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	if err := conn.SetReadBuffer(1 << 20); err != nil {
		_ = conn.Close()
		return fmt.Errorf("set read buffer: %w", err)
	}
	s.conn = conn

	ctx, cancel := context.WithCancel(parentCtx)
	s.cancel = cancel

	s.wg.Add(3)
	go func() {
		defer s.wg.Done()
		s.listenLoop(ctx)
	}()
	go func() {
		defer s.wg.Done()
		s.broadcastLoop(ctx)
	}()
	go func() {
		defer s.wg.Done()
		s.pruneLoop(ctx)
	}()

	s.broadcast("announce")
	return nil
}

func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.wg.Wait()
}

func (s *Service) Devices() []models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]models.Device, 0, len(s.devices))
	for _, d := range s.devices {
		if d.IsOnline {
			items = append(items, d)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

func (s *Service) GetByID(id string) (models.Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[id]
	return d, ok
}

func (s *Service) broadcastLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.broadcast("heartbeat")
		}
	}
}

func (s *Service) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			changed := false

			s.mu.Lock()
			for id, d := range s.devices {
				if d.IsOnline && now.Sub(d.LastSeen) >= s.cfg.OfflineAfter {
					d.IsOnline = false
					d.LastOnline = d.LastSeen
					s.devices[id] = d
					changed = true
				}
			}
			s.mu.Unlock()

			if changed {
				s.notifyChanged()
			}
		}
	}
}

func (s *Service) listenLoop(ctx context.Context) {
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return
		}

		bufPtr := s.bufPool.Get().(*[]byte)
		buf := *bufPtr
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			s.bufPool.Put(bufPtr)
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		var msg models.DiscoveryMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			s.bufPool.Put(bufPtr)
			continue
		}
		s.bufPool.Put(bufPtr)

		if msg.ID == "" || msg.ID == s.cfg.DeviceID {
			continue
		}

		s.updateDevice(msg, addr.IP.String())

		if msg.Type == "announce" || msg.Type == "heartbeat" {
			s.respondUnicast(addr)
		}
	}
}

func (s *Service) respondUnicast(addr *net.UDPAddr) {
	msg := s.makeMessage("response")
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_, _ = s.conn.WriteToUDP(payload, addr)
}

func (s *Service) broadcast(messageType string) {
	msg := s.makeMessage(messageType)
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	broadcastTargets := []string{fmt.Sprintf("255.255.255.255:%d", s.cfg.UDPPort)}
	addrs, err := utils.LocalIPv4Addrs()
	if err == nil {
		for _, ip := range addrs {
			parsed := net.ParseIP(ip).To4()
			if parsed == nil {
				continue
			}
			broadcast := net.IPv4(parsed[0], parsed[1], parsed[2], 255)
			broadcastTargets = append(broadcastTargets, fmt.Sprintf("%s:%d", broadcast.String(), s.cfg.UDPPort))
		}
	}

	for _, target := range broadcastTargets {
		addr, err := net.ResolveUDPAddr("udp4", target)
		if err != nil {
			continue
		}
		_, _ = s.conn.WriteToUDP(payload, addr)
	}
}

func (s *Service) makeMessage(messageType string) models.DiscoveryMessage {
	return models.DiscoveryMessage{
		Type:      messageType,
		ID:        s.cfg.DeviceID,
		Name:      s.cfg.DeviceName,
		IP:        utils.PrimaryIPv4(),
		Port:      s.cfg.TCPPort,
		Timestamp: time.Now().UnixMilli(),
	}
}

func (s *Service) updateDevice(msg models.DiscoveryMessage, sourceIP string) {
	now := time.Now()
	latency := int(now.UnixMilli() - msg.Timestamp)
	if latency < 0 {
		latency = 0
	}

	changed := false
	s.mu.Lock()
	existing, exists := s.devices[msg.ID]
	device := models.Device{
		ID:         msg.ID,
		Name:       msg.Name,
		IP:         sourceIP,
		Port:       msg.Port,
		LastSeen:   now,
		LastOnline: now,
		IsOnline:   true,
		Latency:    latency,
	}
	if exists {
		if existing.IP != device.IP || existing.IsOnline != device.IsOnline || existing.Name != device.Name || existing.Latency != device.Latency || existing.Port != device.Port {
			changed = true
		}
	} else {
		changed = true
	}
	s.devices[msg.ID] = device
	s.mu.Unlock()

	if changed {
		s.notifyChanged()
	}
}

func (s *Service) notifyChanged() {
	if s.onChanged != nil {
		s.onChanged()
	}
}
