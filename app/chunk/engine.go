package chunk

import (
	"fmt"
	"os"
)

type ChunkInfo struct {
	Index  int64
	Offset int64
	Size   int64
}

type Engine struct {
	ChunkSize int64
}

func NewEngine(chunkSize int64) *Engine {
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024
	}
	return &Engine{ChunkSize: chunkSize}
}

func (e *Engine) Plan(filePath string) ([]ChunkInfo, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	total := fi.Size()
	if total == 0 {
		return []ChunkInfo{{Index: 0, Offset: 0, Size: 0}}, nil
	}

	var chunks []ChunkInfo
	var offset int64
	var idx int64
	for offset < total {
		remaining := total - offset
		size := e.ChunkSize
		if remaining < size {
			size = remaining
		}
		chunks = append(chunks, ChunkInfo{Index: idx, Offset: offset, Size: size})
		offset += size
		idx++
	}
	return chunks, nil
}

func (e *Engine) Validate() error {
	if e.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be > 0")
	}
	return nil
}
