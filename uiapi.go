package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"goshare/app"
	"goshare/app/discovery"
	"goshare/app/transfer"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type UIAPI struct {
	ctx     context.Context
	backend *app.Backend
}

type TransferTaskDTO struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	Target      string `json:"target"`
	Status      string `json:"status"`
	Transferred int64  `json:"transferred"`
	Total       int64  `json:"total"`
	LastError   string `json:"lastError"`
	UpdatedAt   string `json:"updatedAt"`
}

func NewUIAPI(cfg app.Config) *UIAPI {
	return &UIAPI{backend: app.NewBackend(cfg)}
}

func (u *UIAPI) Startup(ctx context.Context) {
	u.ctx = ctx
	if err := u.backend.Start(); err != nil {
		runtime.LogErrorf(ctx, "backend start failed: %v", err)
	}
}

func (u *UIAPI) BeforeClose(ctx context.Context) bool {
	return false
}

func (u *UIAPI) Shutdown(ctx context.Context) {
	u.backend.Stop()
}

func (u *UIAPI) Ping() string {
	return "GoShare UI connected"
}

func (u *UIAPI) PickFile() (string, error) {
	if u.ctx == nil {
		return "", fmt.Errorf("ui context not ready")
	}
	path, err := runtime.OpenFileDialog(u.ctx, runtime.OpenDialogOptions{
		Title: "Pilih file yang akan dikirim",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func (u *UIAPI) DiscoverDevices(timeoutMS int) ([]discovery.DeviceInfo, error) {
	return u.backend.Discover(timeoutMS)
}

func (u *UIAPI) SendFile(targetAddr, filePath string) (string, error) {
	if targetAddr == "" {
		return "", fmt.Errorf("target address is required")
	}
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	return u.backend.SendFile(targetAddr, filePath)
}

func (u *UIAPI) ListTransfers() []TransferTaskDTO {
	tasks := u.backend.ListTransfers()
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})

	out := make([]TransferTaskDTO, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toDTO(t))
	}
	return out
}

func (u *UIAPI) CancelTransfer(id string) error {
	return u.backend.CancelTransfer(id)
}

func toDTO(t transfer.Task) TransferTaskDTO {
	return TransferTaskDTO{
		ID:          t.ID,
		FileName:    t.FileName,
		Target:      t.Target,
		Status:      string(t.Status),
		Transferred: t.Transferred,
		Total:       t.Total,
		LastError:   t.LastError,
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
	}
}
