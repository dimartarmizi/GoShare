package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
)

type Checker struct{}

func NewChecker() *Checker {
	return &Checker{}
}

func (c *Checker) FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Checker) MatchFileSHA256(path, expected string) (bool, error) {
	actual, err := c.FileSHA256(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)), nil
}
