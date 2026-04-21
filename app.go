package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"goshare/app/discovery"
	"goshare/app/models"
	"goshare/app/transfer"
)

const (
	udpPort = 9999
	tcpPort = 9000
)

type App struct {
	ctx context.Context

	deviceID   string
	deviceName string

	discoveryService *discovery.Service
	transferManager  *transfer.Manager

	mu sync.RWMutex
}

func NewApp() *App {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "GoShare Device"
	}

	deviceName := hostname
	if !strings.HasPrefix(strings.ToLower(deviceName), "goshare") {
		deviceName = fmt.Sprintf("%s", hostname)
	}

	return &App{
		deviceID:   uuid.NewString(),
		deviceName: deviceName,
	}
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	notify := func() {
		a.emit("state:updated")
	}

	discoverySvc, err := discovery.NewService(discovery.Config{
		UDPPort:    udpPort,
		TCPPort:    tcpPort,
		DeviceID:   a.deviceID,
		DeviceName: a.deviceName,
	}, notify)
	if err != nil {
		log.Printf("discovery setup failed: %v", err)
		return
	}
	if err := discoverySvc.Start(ctx); err != nil {
		log.Printf("discovery start failed: %v", err)
		return
	}
	a.discoveryService = discoverySvc

	receiveDir := defaultReceiveDir()
	transferMgr, err := transfer.NewManager(transfer.Config{
		Address:        fmt.Sprintf(":%d", tcpPort),
		DeviceID:       a.deviceID,
		DeviceName:     a.deviceName,
		ReceiveDir:     receiveDir,
		ChunkSize:      64 * 1024,
		MaxConnections: 50,
	}, notify)
	if err != nil {
		log.Printf("transfer setup failed: %v", err)
		return
	}
	if err := transferMgr.Start(ctx); err != nil {
		log.Printf("transfer manager start failed: %v", err)
		return
	}
	a.transferManager = transferMgr

	a.emit("state:updated")
}

func (a *App) shutdown(ctx context.Context) {
	if a.discoveryService != nil {
		a.discoveryService.Stop()
	}
	if a.transferManager != nil {
		if err := a.transferManager.Stop(); err != nil {
			log.Printf("transfer manager stop failed: %v", err)
		}
	}
}

func (a *App) AppInfo() map[string]any {
	return map[string]any{
		"name":          "GoShare",
		"tagline":       "Fast local file sharing without internet",
		"deviceId":      a.deviceID,
		"deviceName":    a.deviceName,
		"udpPort":       udpPort,
		"tcpPort":       tcpPort,
		"refreshMs":     1000,
		"startedAt":     time.Now().Format(time.RFC3339),
		"receiveFolder": defaultReceiveDir(),
	}
}

func (a *App) ListDevices() []models.Device {
	if a.discoveryService == nil {
		return []models.Device{}
	}
	return a.discoveryService.Devices()
}

func (a *App) ListTransfers() []models.Transfer {
	if a.transferManager == nil {
		return []models.Transfer{}
	}
	return a.transferManager.ListTransfers()
}

func (a *App) PickFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, errors.New("app is not ready")
	}
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select files to send",
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func (a *App) SendFiles(deviceID string, filePaths []string) (string, error) {
	if a.transferManager == nil {
		return "", errors.New("transfer manager is not ready")
	}
	if a.discoveryService == nil {
		return "", errors.New("discovery service is not ready")
	}

	device, ok := a.discoveryService.GetByID(deviceID)
	if !ok {
		return "", errors.New("device not found")
	}
	if !device.IsOnline {
		return "", errors.New("device is offline")
	}

	transferID, err := a.transferManager.SendFiles(a.ctx, device, filePaths)
	if err != nil {
		return "", err
	}
	return transferID, nil
}

func (a *App) PauseTransfer(transferID string) error {
	if a.transferManager == nil {
		return errors.New("transfer manager is not ready")
	}
	return a.transferManager.PauseTransfer(transferID)
}

func (a *App) ResumeTransfer(transferID string) error {
	if a.transferManager == nil {
		return errors.New("transfer manager is not ready")
	}
	return a.transferManager.ResumeTransfer(transferID)
}

func (a *App) CancelTransfer(transferID string) error {
	if a.transferManager == nil {
		return errors.New("transfer manager is not ready")
	}
	return a.transferManager.CancelTransfer(transferID)
}

func (a *App) AcceptTransfer(transferID string) error {
	if a.transferManager == nil {
		return errors.New("transfer manager is not ready")
	}
	return a.transferManager.AcceptTransfer(transferID)
}

func (a *App) RejectTransfer(transferID string) error {
	if a.transferManager == nil {
		return errors.New("transfer manager is not ready")
	}
	return a.transferManager.RejectTransfer(transferID)
}

func (a *App) emit(eventName string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventName)
	}
}

func defaultReceiveDir() string {
	executablePath, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "received")
	}
	return filepath.Join(filepath.Dir(executablePath), "received")
}
