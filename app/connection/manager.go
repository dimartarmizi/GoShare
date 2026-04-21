package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

type Handler func(conn net.Conn)

type Manager struct {
	address        string
	maxConnections int

	listener net.Listener
	sem      chan struct{}
	wg       sync.WaitGroup
}

func NewManager(address string, maxConnections int) *Manager {
	if maxConnections <= 0 {
		maxConnections = 50
	}
	return &Manager{
		address:        address,
		maxConnections: maxConnections,
		sem:            make(chan struct{}, maxConnections),
	}
}

func (m *Manager) Start(ctx context.Context, handler Handler) error {
	if handler == nil {
		return errors.New("connection handler cannot be nil")
	}

	listener, err := net.Listen("tcp", m.address)
	if err != nil {
		return err
	}
	m.listener = listener

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.acceptLoop(ctx, handler)
	}()
	return nil
}

func (m *Manager) Stop() error {
	if m.listener == nil {
		return nil
	}
	_ = m.listener.Close()
	m.wg.Wait()
	return nil
}

func (m *Manager) acceptLoop(ctx context.Context, handler Handler) {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}

		select {
		case m.sem <- struct{}{}:
			m.wg.Add(1)
			go func(c net.Conn) {
				defer m.wg.Done()
				defer func() { <-m.sem }()
				handler(c)
			}(conn)
		case <-ctx.Done():
			_ = conn.Close()
			return
		}
	}
}
