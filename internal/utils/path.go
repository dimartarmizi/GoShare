package utils

import "path/filepath"

func SafeFileName(name string) string {
	return filepath.Base(name)
}
