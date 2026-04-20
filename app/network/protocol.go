package network

import "encoding/json"

type FileMetadata struct {
	FileName   string `json:"fileName"`
	Size       int64  `json:"size"`
	ChunkSize  int64  `json:"chunkSize,omitempty"`
	TotalChunk int64  `json:"totalChunks,omitempty"`
}

func (m FileMetadata) JSON() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ParseMetadata(raw string) (FileMetadata, error) {
	var meta FileMetadata
	err := json.Unmarshal([]byte(raw), &meta)
	return meta, err
}
