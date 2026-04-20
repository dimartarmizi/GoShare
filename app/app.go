package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"goshare/app/chunk"
	"goshare/app/discovery"
	"goshare/app/network"
	"goshare/app/transfer"
)

type Config struct {
	ListenAddr    string
	SaveDir       string
	DeviceName    string
	DeviceTCPPort int
	DiscoveryPort int
	ChunkSize     int64
}

type App struct {
	Config      Config
	Connections *network.ConnectionManager
	Transfers   *transfer.Manager
	Discovery   *discovery.Service
	Chunk       *chunk.Engine

	OnFileReceived func(network.FileMetadata, string, error)
}

func New(cfg Config) *App {
	conn := network.NewConnectionManager(cfg.ListenAddr, cfg.SaveDir)
	return &App{
		Config:      cfg,
		Connections: conn,
		Transfers:   transfer.NewManager(conn),
		Discovery:   discovery.NewService(cfg.DeviceName, cfg.DeviceTCPPort, cfg.DiscoveryPort),
		Chunk:       chunk.NewEngine(cfg.ChunkSize),
	}
}

func (a *App) Start(ctx context.Context) error {
	if err := a.Chunk.Validate(); err != nil {
		return err
	}

	if err := a.Discovery.StartResponder(ctx); err != nil {
		return fmt.Errorf("start discovery responder: %w", err)
	}

	if err := a.Connections.StartServer(ctx, a.OnFileReceived); err != nil {
		return fmt.Errorf("start TCP server: %w", err)
	}

	return nil
}

func (a *App) DiscoverDevices(ctx context.Context, timeout time.Duration) ([]discovery.DeviceInfo, error) {
	return a.Discovery.Discover(ctx, timeout)
}

func (a *App) SendFile(ctx context.Context, targetAddr, filePath string) (string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	return a.Transfers.StartTransfer(ctx, abs, targetAddr)
}
