package network

import (
	"bufio"
	"fmt"
	"strings"
)

func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line + "\n"); err != nil {
		return err
	}
	return w.Flush()
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func expectPrefix(line, prefix string) (string, error) {
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("expected prefix %q, got %q", prefix, line)
	}
	return strings.TrimPrefix(line, prefix), nil
}
