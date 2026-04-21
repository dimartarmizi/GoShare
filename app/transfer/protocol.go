package transfer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	frameTypeControl byte = 1
	frameTypeChunk   byte = 2
)

const (
	controlTypeHandshake      = "handshake"
	controlTypeHandshakeAck   = "handshake_ack"
	controlTypeFileMeta       = "file_meta"
	controlTypeFileMetaAck    = "file_meta_ack"
	controlTypeChunkHeader    = "chunk_header"
	controlTypeTransferDone   = "transfer_complete"
	controlTypeTransferCancel = "transfer_cancel"
)

type controlMessage struct {
	Type            string `json:"type"`
	TransferID      string `json:"transfer_id,omitempty"`
	DeviceID        string `json:"device_id,omitempty"`
	DeviceName      string `json:"device_name,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`

	Files    []fileMetaPayload `json:"files,omitempty"`
	Accept   bool              `json:"accept,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	FileID   string            `json:"file_id,omitempty"`
	Index    int               `json:"index,omitempty"`
	Size     int               `json:"size,omitempty"`
	Checksum string            `json:"checksum,omitempty"`
}

type fileMetaPayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func writeControl(w io.Writer, msg controlMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return writeFrame(w, frameTypeControl, payload)
}

func readControl(r io.Reader) (controlMessage, error) {
	frameType, payload, err := readFrame(r)
	if err != nil {
		return controlMessage{}, err
	}
	if frameType != frameTypeControl {
		return controlMessage{}, fmt.Errorf("expected control frame but got type %d", frameType)
	}

	var msg controlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return controlMessage{}, err
	}
	return msg, nil
}

func writeChunk(w io.Writer, data []byte) error {
	return writeFrame(w, frameTypeChunk, data)
}

func readFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	frameType := header[0]
	length := binary.BigEndian.Uint32(header[1:5])
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return frameType, payload, nil
}

func writeFrame(w io.Writer, frameType byte, payload []byte) error {
	header := make([]byte, 5)
	header[0] = frameType
	binary.BigEndian.PutUint32(header[1:5], uint32(len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}
