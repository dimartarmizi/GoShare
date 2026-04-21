package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"goshare/app/connection"
	"goshare/app/models"
	"goshare/app/utils"
)

const (
	protocolVersion  = 1
	defaultChunkSize = 128 * 1024
)

type Config struct {
	Address        string
	DeviceID       string
	DeviceName     string
	ReceiveDir     string
	ChunkSize      int
	MaxConnections int
}

type Manager struct {
	cfg       Config
	connMgr   *connection.Manager
	onChanged func()

	mu               sync.RWMutex
	transfers        map[string]*models.Transfer
	controls         map[string]*transferControl
	pendingDecisions map[string]chan bool
}

type transferControl struct {
	mu     sync.Mutex
	paused bool
	cancel context.CancelFunc
}

type fileSource struct {
	path string
	meta models.FileMeta
}

func NewManager(cfg Config, onChanged func()) (*Manager, error) {
	if cfg.Address == "" {
		cfg.Address = ":9000"
	}
	if cfg.DeviceID == "" {
		return nil, errors.New("device id is required")
	}
	if cfg.DeviceName == "" {
		cfg.DeviceName = "GoShare Device"
	}
	if cfg.ReceiveDir == "" {
		cfg.ReceiveDir = defaultReceiveDir()
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 50
	}

	return &Manager{
		cfg:              cfg,
		onChanged:        onChanged,
		transfers:        make(map[string]*models.Transfer),
		controls:         make(map[string]*transferControl),
		pendingDecisions: make(map[string]chan bool),
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	m.connMgr = connection.NewManager(m.cfg.Address, m.cfg.MaxConnections)
	return m.connMgr.Start(ctx, m.handleConn)
}

func (m *Manager) Stop() error {
	if m.connMgr == nil {
		return nil
	}
	return m.connMgr.Stop()
}

func (m *Manager) ListTransfers() []models.Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]models.Transfer, 0, len(m.transfers))
	for _, transfer := range m.transfers {
		copyTransfer := *transfer
		copyTransfer.Files = append([]models.FileMeta(nil), transfer.Files...)
		copyTransfer.DirectionLabel = transferDirectionLabel(copyTransfer.Direction)
		copyTransfer.StatusLabel = transferStatusLabel(copyTransfer.Status)
		items = append(items, copyTransfer)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (m *Manager) SendFiles(ctx context.Context, target models.Device, filePaths []string) (string, error) {
	if len(filePaths) == 0 {
		return "", errors.New("no file selected")
	}
	if target.IP == "" || target.Port == 0 {
		return "", errors.New("target device is invalid")
	}

	sources, totalBytes, err := m.prepareSources(filePaths)
	if err != nil {
		return "", err
	}

	transferID := uuid.NewString()
	transfer := &models.Transfer{
		ID:               transferID,
		PeerID:           target.ID,
		PeerName:         target.Name,
		Direction:        models.TransferDirectionOutgoing,
		Status:           models.TransferStatusPending,
		Files:            make([]models.FileMeta, 0, len(sources)),
		TotalBytes:       totalBytes,
		TransferredBytes: 0,
		Progress:         0,
		StartedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	for _, source := range sources {
		transfer.Files = append(transfer.Files, source.meta)
	}

	childCtx, cancel := context.WithCancel(ctx)
	control := &transferControl{cancel: cancel}

	m.mu.Lock()
	m.transfers[transferID] = transfer
	m.controls[transferID] = control
	m.mu.Unlock()
	m.notifyChanged()

	go m.runOutgoingTransfer(childCtx, transferID, target, sources, control)
	return transferID, nil
}

func (m *Manager) PauseTransfer(transferID string) error {
	m.mu.RLock()
	control, ok := m.controls[transferID]
	m.mu.RUnlock()
	if !ok {
		return errors.New("transfer not found")
	}

	control.setPaused(true)
	m.setStatus(transferID, models.TransferStatusPaused, "")
	return nil
}

func (m *Manager) ResumeTransfer(transferID string) error {
	m.mu.RLock()
	control, ok := m.controls[transferID]
	m.mu.RUnlock()
	if !ok {
		return errors.New("transfer not found")
	}

	control.setPaused(false)
	m.setStatus(transferID, models.TransferStatusInProgress, "")
	return nil
}

func (m *Manager) CancelTransfer(transferID string) error {
	m.mu.RLock()
	control, ok := m.controls[transferID]
	m.mu.RUnlock()
	if !ok {
		return errors.New("transfer not found")
	}

	control.stop()
	m.setStatus(transferID, models.TransferStatusCanceled, "")
	return nil
}

func (m *Manager) AcceptTransfer(transferID string) error {
	return m.resolveIncomingDecision(transferID, true)
}

func (m *Manager) RejectTransfer(transferID string) error {
	return m.resolveIncomingDecision(transferID, false)
}

func (m *Manager) resolveIncomingDecision(transferID string, accept bool) error {
	m.mu.Lock()
	decisionChan, ok := m.pendingDecisions[transferID]
	if !ok {
		m.mu.Unlock()
		return errors.New("incoming transfer not found")
	}
	delete(m.pendingDecisions, transferID)
	m.mu.Unlock()

	select {
	case decisionChan <- accept:
		return nil
	default:
		return errors.New("incoming transfer decision already resolved")
	}
}

func (m *Manager) runOutgoingTransfer(ctx context.Context, transferID string, target models.Device, files []fileSource, control *transferControl) {
	defer m.cleanupControl(transferID)

	address := net.JoinHostPort(target.IP, strconv.Itoa(target.Port))
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		m.failTransfer(transferID, fmt.Sprintf("cannot connect: %v", err))
		return
	}
	defer conn.Close()

	m.setStatus(transferID, models.TransferStatusInProgress, "")

	if err := writeControl(conn, controlMessage{
		Type:            controlTypeHandshake,
		TransferID:      transferID,
		DeviceID:        m.cfg.DeviceID,
		DeviceName:      m.cfg.DeviceName,
		ProtocolVersion: protocolVersion,
	}); err != nil {
		m.failTransfer(transferID, fmt.Sprintf("send handshake failed: %v", err))
		return
	}

	handshakeAck, err := readControl(conn)
	if err != nil {
		m.failTransfer(transferID, fmt.Sprintf("read handshake ack failed: %v", err))
		return
	}
	if handshakeAck.Type != controlTypeHandshakeAck {
		m.failTransfer(transferID, "invalid handshake ack")
		return
	}

	payloadFiles := make([]fileMetaPayload, 0, len(files))
	for _, source := range files {
		payloadFiles = append(payloadFiles, fileMetaPayload{ID: source.meta.ID, Name: source.meta.Name, Size: source.meta.Size})
	}

	if err := writeControl(conn, controlMessage{Type: controlTypeFileMeta, TransferID: transferID, Files: payloadFiles}); err != nil {
		m.failTransfer(transferID, fmt.Sprintf("send metadata failed: %v", err))
		return
	}

	metaAck, err := readControl(conn)
	if err != nil {
		m.failTransfer(transferID, fmt.Sprintf("read metadata ack failed: %v", err))
		return
	}
	if metaAck.Type != controlTypeFileMetaAck {
		m.failTransfer(transferID, "invalid metadata ack")
		return
	}
	if !metaAck.Accept {
		m.setStatus(transferID, models.TransferStatusRejected, firstNonEmpty(metaAck.Reason, "receiver rejected transfer"))
		return
	}

	buf := make([]byte, m.cfg.ChunkSize)
	for _, source := range files {
		if err := control.waitIfPaused(ctx); err != nil {
			m.handleContextCancellation(conn, transferID, err)
			return
		}
		if err := m.sendFileChunks(ctx, conn, transferID, source, buf, control); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				m.handleContextCancellation(conn, transferID, err)
				return
			}
			m.failTransfer(transferID, err.Error())
			return
		}
	}

	if err := writeControl(conn, controlMessage{Type: controlTypeTransferDone, TransferID: transferID}); err != nil {
		m.failTransfer(transferID, fmt.Sprintf("send completion failed: %v", err))
		return
	}

	m.setStatus(transferID, models.TransferStatusCompleted, "")
}

func (m *Manager) sendFileChunks(ctx context.Context, conn net.Conn, transferID string, source fileSource, buf []byte, control *transferControl) error {
	f, err := os.Open(source.path)
	if err != nil {
		return fmt.Errorf("open %s failed: %w", source.meta.Name, err)
	}
	defer f.Close()

	index := 0
	for {
		if err := control.waitIfPaused(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			checksum := utils.SHA256Hex(chunk)

			header := controlMessage{
				Type:       controlTypeChunkHeader,
				TransferID: transferID,
				FileID:     source.meta.ID,
				Index:      index,
				Size:       n,
				Checksum:   checksum,
			}
			if err := writeControl(conn, header); err != nil {
				return fmt.Errorf("send chunk header failed: %w", err)
			}
			if err := writeChunk(conn, chunk); err != nil {
				return fmt.Errorf("send chunk failed: %w", err)
			}
			m.incrementTransferred(transferID, int64(n))
			index++
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read %s failed: %w", source.meta.Name, readErr)
		}
	}

	return nil
}

func (m *Manager) handleContextCancellation(conn net.Conn, transferID string, err error) {
	_ = writeControl(conn, controlMessage{Type: controlTypeTransferCancel, TransferID: transferID, Reason: err.Error()})
	m.setStatus(transferID, models.TransferStatusCanceled, "")
}

func (m *Manager) handleConn(conn net.Conn) {
	defer conn.Close()

	handshake, err := readControl(conn)
	if err != nil {
		return
	}
	if handshake.Type != controlTypeHandshake {
		return
	}

	if err := writeControl(conn, controlMessage{Type: controlTypeHandshakeAck, ProtocolVersion: protocolVersion}); err != nil {
		return
	}

	metaMsg, err := readControl(conn)
	if err != nil {
		return
	}
	if metaMsg.Type != controlTypeFileMeta {
		return
	}

	if len(metaMsg.Files) == 0 {
		_ = writeControl(conn, controlMessage{Type: controlTypeFileMetaAck, Accept: false, Reason: "no files supplied"})
		return
	}

	transferID := firstNonEmpty(metaMsg.TransferID, uuid.NewString())
	transfer := m.createIncomingTransfer(transferID, handshake, metaMsg)

	decision := make(chan bool, 1)
	m.mu.Lock()
	m.pendingDecisions[transferID] = decision
	m.mu.Unlock()

	var accept bool
	select {
	case accept = <-decision:
	case <-time.After(60 * time.Second):
		accept = false
	}

	m.mu.Lock()
	delete(m.pendingDecisions, transferID)
	m.mu.Unlock()

	ackReason := ""
	if !accept {
		ackReason = "transfer rejected by user"
	}
	if err := writeControl(conn, controlMessage{Type: controlTypeFileMetaAck, Accept: accept, Reason: ackReason}); err != nil {
		m.failTransfer(transfer.ID, fmt.Sprintf("send transfer decision failed: %v", err))
		return
	}
	if !accept {
		m.setStatus(transferID, models.TransferStatusRejected, ackReason)
		return
	}

	m.setStatus(transferID, models.TransferStatusInProgress, "")
	writers, closeFiles, err := m.prepareIncomingFiles(transfer.Files)
	if err != nil {
		m.failTransfer(transferID, err.Error())
		return
	}
	defer closeFiles()

	var pendingHeader *controlMessage
	for {
		frameType, payload, err := readFrame(conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				m.failTransfer(transferID, "sender disconnected")
				return
			}
			m.failTransfer(transferID, fmt.Sprintf("read frame failed: %v", err))
			return
		}

		switch frameType {
		case frameTypeControl:
			var ctrl controlMessage
			if err := json.Unmarshal(payload, &ctrl); err != nil {
				m.failTransfer(transferID, "invalid control payload")
				return
			}
			switch ctrl.Type {
			case controlTypeChunkHeader:
				pendingHeader = &ctrl
			case controlTypeTransferDone:
				m.setStatus(transferID, models.TransferStatusCompleted, "")
				return
			case controlTypeTransferCancel:
				m.setStatus(transferID, models.TransferStatusCanceled, firstNonEmpty(ctrl.Reason, "sender canceled transfer"))
				return
			}
		case frameTypeChunk:
			if pendingHeader == nil {
				m.failTransfer(transferID, "received chunk without header")
				return
			}
			if pendingHeader.Size != len(payload) {
				m.failTransfer(transferID, "chunk size mismatch")
				return
			}
			if expected := pendingHeader.Checksum; expected != "" && expected != utils.SHA256Hex(payload) {
				m.failTransfer(transferID, "chunk checksum mismatch")
				return
			}

			writer, ok := writers[pendingHeader.FileID]
			if !ok {
				m.failTransfer(transferID, "unknown file id")
				return
			}

			offset := int64(pendingHeader.Index) * int64(m.cfg.ChunkSize)
			if _, err := writer.WriteAt(payload, offset); err != nil {
				m.failTransfer(transferID, fmt.Sprintf("write chunk failed: %v", err))
				return
			}
			m.incrementTransferred(transferID, int64(len(payload)))
			pendingHeader = nil
		}
	}
}

func (m *Manager) prepareSources(filePaths []string) ([]fileSource, int64, error) {
	sources := make([]fileSource, 0, len(filePaths))
	var total int64

	for _, rawPath := range filePaths {
		filePath := strings.TrimSpace(rawPath)
		if filePath == "" {
			continue
		}

		info, err := os.Stat(filePath)
		if err != nil {
			return nil, 0, fmt.Errorf("cannot stat file %s: %w", filePath, err)
		}
		if info.IsDir() {
			return nil, 0, fmt.Errorf("folders are not supported in this MVP: %s", filePath)
		}

		meta := models.FileMeta{
			ID:   uuid.NewString(),
			Name: filepath.Base(filePath),
			Size: info.Size(),
		}
		sources = append(sources, fileSource{path: filePath, meta: meta})
		total += info.Size()
	}

	if len(sources) == 0 {
		return nil, 0, errors.New("no valid file selected")
	}
	return sources, total, nil
}

func (m *Manager) createIncomingTransfer(transferID string, handshake controlMessage, metadata controlMessage) *models.Transfer {
	files := make([]models.FileMeta, 0, len(metadata.Files))
	var total int64
	for _, f := range metadata.Files {
		files = append(files, models.FileMeta{ID: f.ID, Name: f.Name, Size: f.Size})
		total += f.Size
	}

	transfer := &models.Transfer{
		ID:               transferID,
		PeerID:           handshake.DeviceID,
		PeerName:         firstNonEmpty(handshake.DeviceName, handshake.DeviceID),
		Direction:        models.TransferDirectionIncoming,
		Status:           models.TransferStatusPending,
		Files:            files,
		TotalBytes:       total,
		TransferredBytes: 0,
		Progress:         0,
		StartedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	m.mu.Lock()
	m.transfers[transferID] = transfer
	m.mu.Unlock()
	m.notifyChanged()

	return transfer
}

func (m *Manager) prepareIncomingFiles(files []models.FileMeta) (map[string]*os.File, func(), error) {
	if err := os.MkdirAll(m.cfg.ReceiveDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("cannot create receive directory: %w", err)
	}

	writers := make(map[string]*os.File, len(files))
	closeFn := func() {
		for _, f := range writers {
			_ = f.Close()
		}
	}

	for _, meta := range files {
		name := safeFileName(meta.Name)
		path := uniqueFilePath(m.cfg.ReceiveDir, name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			closeFn()
			return nil, nil, fmt.Errorf("cannot create %s: %w", name, err)
		}
		writers[meta.ID] = f
	}
	return writers, closeFn, nil
}

func (m *Manager) incrementTransferred(transferID string, delta int64) {
	m.mu.Lock()
	transfer, ok := m.transfers[transferID]
	if ok {
		transfer.TransferredBytes += delta
		if transfer.TransferredBytes > transfer.TotalBytes {
			transfer.TransferredBytes = transfer.TotalBytes
		}
		if transfer.TotalBytes > 0 {
			transfer.Progress = float64(transfer.TransferredBytes) / float64(transfer.TotalBytes)
		}
		transfer.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	if ok {
		m.notifyChanged()
	}
}

func (m *Manager) setStatus(transferID string, status string, message string) {
	m.mu.Lock()
	transfer, ok := m.transfers[transferID]
	if ok {
		transfer.Status = status
		if message != "" {
			transfer.Error = message
		}
		transfer.UpdatedAt = time.Now()
		if transfer.TotalBytes > 0 {
			transfer.Progress = float64(transfer.TransferredBytes) / float64(transfer.TotalBytes)
		}
	}
	m.mu.Unlock()

	if ok {
		m.notifyChanged()
	}
}

func transferStatusLabel(status string) string {
	switch status {
	case models.TransferStatusPending:
		return "Pending"
	case models.TransferStatusInProgress:
		return "In Progress"
	case models.TransferStatusPaused:
		return "Paused"
	case models.TransferStatusCompleted:
		return "Completed"
	case models.TransferStatusFailed:
		return "Failed"
	case models.TransferStatusCanceled:
		return "Canceled"
	case models.TransferStatusRejected:
		return "Rejected"
	default:
		return "Unknown"
	}
}

func transferDirectionLabel(direction string) string {
	switch direction {
	case models.TransferDirectionIncoming:
		return "Incoming"
	case models.TransferDirectionOutgoing:
		return "Outgoing"
	default:
		return "Unknown"
	}
}

func (m *Manager) failTransfer(transferID string, reason string) {
	if strings.TrimSpace(reason) == "" {
		reason = "transfer failed"
	}
	m.setStatus(transferID, models.TransferStatusFailed, reason)
}

func (m *Manager) cleanupControl(transferID string) {
	m.mu.Lock()
	delete(m.controls, transferID)
	delete(m.pendingDecisions, transferID)
	m.mu.Unlock()
}

func (c *transferControl) setPaused(paused bool) {
	c.mu.Lock()
	c.paused = paused
	c.mu.Unlock()
}

func (c *transferControl) waitIfPaused(ctx context.Context) error {
	for {
		c.mu.Lock()
		paused := c.paused
		c.mu.Unlock()

		if !paused {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(120 * time.Millisecond):
		}
	}
}

func (c *transferControl) stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (m *Manager) notifyChanged() {
	if m.onChanged != nil {
		m.onChanged()
	}
}

func defaultReceiveDir() string {
	executablePath, err := os.Executable()
	if err != nil {
		return "./received"
	}
	return filepath.Join(filepath.Dir(executablePath), "received")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "received.bin"
	}
	return base
}

func uniqueFilePath(dir string, fileName string) string {
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}

	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, time.Now().UnixNano(), ext))
}
