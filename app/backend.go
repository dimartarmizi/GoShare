package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"goshare/app/discovery"
	"goshare/app/transfer"
)

type Backend struct {
	core   *App
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

func NewBackend(cfg Config) *Backend {
	return &Backend{core: New(cfg)}
}

func (b *Backend) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx != nil {
		return nil
	}
	b.ctx, b.cancel = context.WithCancel(context.Background())
	return b.core.Start(b.ctx)
}

func (b *Backend) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
	}
	b.ctx = nil
	b.cancel = nil
}

func (b *Backend) Discover(timeoutMS int) ([]discovery.DeviceInfo, error) {
	ctx := context.Background()
	timeout := 3 * time.Second
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	return b.core.DiscoverDevices(ctx, timeout)
}

func (b *Backend) SendFile(targetAddr, filePath string) (string, error) {
	if targetAddr == "" {
		return "", fmt.Errorf("target address is required")
	}
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	return b.core.SendFile(context.Background(), targetAddr, filePath)
}

func (b *Backend) ListTransfers() []transfer.Task {
	return b.core.Transfers.ListTasks()
}

func (b *Backend) GetTransfer(id string) (transfer.Task, bool) {
	return b.core.Transfers.GetTask(id)
}

func (b *Backend) CancelTransfer(id string) error {
	if id == "" {
		return fmt.Errorf("transfer id is required")
	}
	return b.core.Transfers.CancelTransfer(id)
}
