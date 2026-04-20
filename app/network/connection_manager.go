package network

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goshare/internal/utils"
)

type ProgressFunc func(transferred, total int64)

const (
	defaultChunkSize = int64(1024 * 1024)
	maxChunkRetry    = 5
)

type ConnectionManager struct {
	ListenAddr string
	SaveDir    string
	Timeout    time.Duration

	listener net.Listener
	mu       sync.Mutex
}

func NewConnectionManager(listenAddr, saveDir string) *ConnectionManager {
	return &ConnectionManager{
		ListenAddr: listenAddr,
		SaveDir:    saveDir,
		Timeout:    30 * time.Second,
	}
}

func (cm *ConnectionManager) StartServer(ctx context.Context, onReceived func(FileMetadata, string, error)) error {
	if err := os.MkdirAll(cm.SaveDir, 0o755); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", cm.ListenAddr)
	if err != nil {
		return err
	}

	cm.mu.Lock()
	cm.listener = ln
	cm.mu.Unlock()

	go func() {
		<-ctx.Done()
		_ = cm.StopServer()
	}()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}

			go func(c net.Conn) {
				meta, path, handleErr := cm.handleInboundConnection(c)
				if onReceived != nil {
					onReceived(meta, path, handleErr)
				}
			}(conn)
		}
	}()

	return nil
}

func (cm *ConnectionManager) StopServer() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.listener == nil {
		return nil
	}
	err := cm.listener.Close()
	cm.listener = nil
	return err
}

func (cm *ConnectionManager) SendFile(ctx context.Context, targetAddr, filePath string, progress ProgressFunc) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	hash, err := utils.FileSHA256(filePath)
	if err != nil {
		return err
	}

	d := net.Dialer{Timeout: cm.Timeout}
	conn, err := d.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	_ = conn.SetDeadline(time.Now().Add(cm.Timeout))
	if err := writeLine(w, "HELLO"); err != nil {
		return err
	}

	line, err := readLine(r)
	if err != nil {
		return err
	}
	if line != "OK" {
		return fmt.Errorf("unexpected handshake response: %s", line)
	}

	meta := FileMetadata{
		FileName:   filepath.Base(filePath),
		Size:       fi.Size(),
		ChunkSize:  defaultChunkSize,
		TotalChunk: totalChunks(fi.Size(), defaultChunkSize),
	}
	metaRaw, err := meta.JSON()
	if err != nil {
		return err
	}
	if err := writeLine(w, "META "+metaRaw); err != nil {
		return err
	}

	line, err = readLine(r)
	if err != nil {
		return err
	}
	readyRaw, err := expectPrefix(line, "READY ")
	if err != nil {
		return fmt.Errorf("remote side not ready: %s", line)
	}

	var resumeOffset int64
	if _, err := fmt.Sscanf(readyRaw, "%d", &resumeOffset); err != nil {
		return fmt.Errorf("invalid resume offset: %w", err)
	}
	if resumeOffset < 0 || resumeOffset > fi.Size() {
		return fmt.Errorf("invalid resume offset %d for size %d", resumeOffset, fi.Size())
	}
	if resumeOffset%meta.ChunkSize != 0 {
		return fmt.Errorf("resume offset %d is not aligned to chunk size %d", resumeOffset, meta.ChunkSize)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(resumeOffset, io.SeekStart); err != nil {
		return err
	}

	buf := make([]byte, defaultChunkSize)
	sent := resumeOffset
	chunkIndex := resumeOffset / meta.ChunkSize
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			if err := cm.sendChunkWithRetry(conn, r, w, chunkIndex, buf[:n]); err != nil {
				return err
			}
			sent += int64(n)
			chunkIndex++
			if progress != nil {
				progress(sent, fi.Size())
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}
	}

	if err := writeLine(w, "DONE "+hash); err != nil {
		return err
	}

	line, err = readLine(r)
	if err != nil {
		return err
	}
	if resultErrRaw, err := expectPrefix(line, "RESULT ERROR "); err == nil {
		return fmt.Errorf(strings.TrimSpace(resultErrRaw))
	}
	if line != "RESULT OK" {
		return fmt.Errorf("transfer failed: %s", line)
	}

	return nil
}

func (cm *ConnectionManager) sendChunkWithRetry(conn net.Conn, r *bufio.Reader, w *bufio.Writer, chunkIndex int64, data []byte) error {
	for attempt := 1; attempt <= maxChunkRetry; attempt++ {
		_ = conn.SetDeadline(time.Now().Add(cm.Timeout))
		if err := writeLine(w, fmt.Sprintf("CHUNK %d %d", chunkIndex, len(data))); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}

		line, err := readLine(r)
		if err != nil {
			if attempt == maxChunkRetry {
				return err
			}
			continue
		}

		ackIndexRaw, ackErr := expectPrefix(line, "ACK ")
		if ackErr == nil {
			var ackIndex int64
			if _, err := fmt.Sscanf(ackIndexRaw, "%d", &ackIndex); err == nil && ackIndex == chunkIndex {
				return nil
			}
		}

		nackIndexRaw, nackErr := expectPrefix(line, "NACK ")
		if nackErr == nil {
			var nackIndex int64
			if _, err := fmt.Sscanf(nackIndexRaw, "%d", &nackIndex); err == nil && nackIndex == chunkIndex {
				if attempt == maxChunkRetry {
					return fmt.Errorf("chunk %d rejected after %d attempts", chunkIndex, maxChunkRetry)
				}
				continue
			}
		}

		if attempt == maxChunkRetry {
			return fmt.Errorf("unexpected chunk response: %s", line)
		}
	}

	return fmt.Errorf("chunk %d failed after retries", chunkIndex)
}

func (cm *ConnectionManager) handleInboundConnection(conn net.Conn) (FileMetadata, string, error) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(cm.Timeout))

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	line, err := readLine(r)
	if err != nil {
		return FileMetadata{}, "", err
	}
	if line != "HELLO" {
		return FileMetadata{}, "", fmt.Errorf("invalid handshake: %s", line)
	}
	if err := writeLine(w, "OK"); err != nil {
		return FileMetadata{}, "", err
	}

	line, err = readLine(r)
	if err != nil {
		return FileMetadata{}, "", err
	}
	metaRaw, err := expectPrefix(line, "META ")
	if err != nil {
		return FileMetadata{}, "", err
	}

	meta, err := ParseMetadata(metaRaw)
	if err != nil {
		return FileMetadata{}, "", err
	}
	if meta.Size < 0 {
		return meta, "", fmt.Errorf("invalid metadata size")
	}
	if meta.ChunkSize <= 0 {
		meta.ChunkSize = defaultChunkSize
	}

	sendResultError := func(message string) {
		clean := strings.ReplaceAll(strings.TrimSpace(message), "\n", " ")
		clean = strings.ReplaceAll(clean, "\r", " ")
		if clean == "" {
			clean = "unknown receiver error"
		}
		_ = writeLine(w, "RESULT ERROR "+clean)
	}

	safeName := filepath.Base(meta.FileName)
	tmpPath := filepath.Join(cm.SaveDir, safeName+".part")
	targetPath := filepath.Join(cm.SaveDir, safeName)

	offset, err := computeResumeOffset(tmpPath, meta.Size, meta.ChunkSize)
	if err != nil {
		return meta, "", err
	}
	if err := writeLine(w, fmt.Sprintf("READY %d", offset)); err != nil {
		return meta, "", err
	}

	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return meta, "", err
	}
	defer func() {
		_ = file.Close()
	}()

	if err := file.Truncate(offset); err != nil {
		_ = os.Remove(tmpPath)
		return meta, "", err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	written, err := cm.receiveChunks(conn, r, w, file, meta, offset)
	if err != nil {
		sendResultError("receive chunk failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}
	if written != meta.Size {
		sendResultError(fmt.Sprintf("incomplete payload: %d/%d", written, meta.Size))
		_ = os.Remove(tmpPath)
		return meta, "", fmt.Errorf("incomplete payload: %d/%d", written, meta.Size)
	}

	line, err = readLine(r)
	if err != nil {
		sendResultError("read DONE failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}
	remoteHash, err := expectPrefix(line, "DONE ")
	if err != nil {
		sendResultError("invalid DONE message")
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	if err := file.Sync(); err != nil {
		sendResultError("file sync failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	localHash, err := utils.FileSHA256(tmpPath)
	if err != nil {
		sendResultError("hash compute failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}
	if !strings.EqualFold(localHash, strings.TrimSpace(remoteHash)) {
		_ = os.Remove(tmpPath)
		_ = writeLine(w, "RESULT HASH_MISMATCH")
		return meta, "", fmt.Errorf("hash mismatch")
	}

	if err := os.MkdirAll(cm.SaveDir, 0o755); err != nil {
		sendResultError("save dir unavailable: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	if err := file.Close(); err != nil {
		sendResultError("close temp file failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	targetPath = avoidOverwrite(targetPath)
	if err := renameWithRetry(tmpPath, targetPath, 8, 120*time.Millisecond); err != nil {
		sendResultError("finalize file failed: " + err.Error())
		_ = os.Remove(tmpPath)
		return meta, "", err
	}

	if err := writeLine(w, "RESULT OK"); err != nil {
		return meta, targetPath, err
	}

	return meta, targetPath, nil
}

func (cm *ConnectionManager) receiveChunks(conn net.Conn, r *bufio.Reader, w *bufio.Writer, file *os.File, meta FileMetadata, offset int64) (int64, error) {
	remaining := meta.Size - offset
	written := offset
	expectedIndex := offset / meta.ChunkSize

	for remaining > 0 {
		_ = conn.SetDeadline(time.Now().Add(cm.Timeout))

		line, err := readLine(r)
		if err != nil {
			return written, err
		}

		raw, err := expectPrefix(line, "CHUNK ")
		if err != nil {
			return written, err
		}

		var idx int64
		var size int64
		if _, err := fmt.Sscanf(raw, "%d %d", &idx, &size); err != nil {
			return written, err
		}
		if idx != expectedIndex {
			_ = writeLine(w, fmt.Sprintf("NACK %d", idx))
			continue
		}
		if size <= 0 || size > remaining || size > meta.ChunkSize {
			_ = writeLine(w, fmt.Sprintf("NACK %d", idx))
			continue
		}

		buf := make([]byte, size)
		if _, err := io.ReadFull(r, buf); err != nil {
			_ = writeLine(w, fmt.Sprintf("NACK %d", idx))
			return written, err
		}

		if _, err := file.Write(buf); err != nil {
			_ = writeLine(w, fmt.Sprintf("NACK %d", idx))
			return written, err
		}

		written += size
		remaining -= size
		expectedIndex++

		if err := writeLine(w, fmt.Sprintf("ACK %d", idx)); err != nil {
			return written, err
		}
	}

	return written, nil
}

func computeResumeOffset(tmpPath string, totalSize, chunkSize int64) (int64, error) {
	fi, err := os.Stat(tmpPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	size := fi.Size()
	if size <= 0 {
		return 0, nil
	}
	if size > totalSize {
		return 0, nil
	}
	if chunkSize <= 0 {
		return 0, nil
	}

	aligned := (size / chunkSize) * chunkSize
	if aligned > totalSize {
		return 0, nil
	}
	return aligned, nil
}

func avoidOverwrite(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	stamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s%s", base, stamp, ext)
}

func totalChunks(size, chunkSize int64) int64 {
	if size <= 0 || chunkSize <= 0 {
		return 1
	}
	total := size / chunkSize
	if size%chunkSize != 0 {
		total++
	}
	if total == 0 {
		return 1
	}
	return total
}

func renameWithRetry(src, dst string, maxAttempts int, delay time.Duration) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := os.Rename(src, dst)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts {
			time.Sleep(delay)
		}
	}

	return lastErr
}
