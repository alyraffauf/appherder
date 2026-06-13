package main

import (
	"fmt"
	"os"
)

func copyFile(src string, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create destination %s: %w", dest, err)
	}

	if err := copyTo(src, out); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close destination %s: %w", dest, err)
	}
	return nil
}
