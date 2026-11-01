package notifications

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ensureFile создаёт каталог и файл с начальным содержимым, если файла ещё нет.
// Существующий файл не перезаписывается.
func ensureFile(path string, initial []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	if _, err := file.Write(initial); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// loadJSONFile создаёт файл, если его нет, и читает его. Повреждённый файл не
// затирается: он переименовывается в *.corrupt-<время>, а вместо него создаётся новый.
func loadJSONFile[T any](path string, initial []byte) (T, error) {
	var value T
	if err := ensureFile(path, initial); err != nil {
		return value, fmt.Errorf("create %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return value, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return value, nil
	}
	if err := json.Unmarshal(data, &value); err != nil {
		var zero T
		backup := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		if renameErr := os.Rename(path, backup); renameErr != nil {
			return zero, fmt.Errorf("%s is corrupted (%v) and could not be moved aside: %w", path, err, renameErr)
		}
		log.Printf("notifications: %s is corrupted, saved as %s and recreated: %v", path, backup, err)
		return zero, ensureFile(path, initial)
	}
	return value, nil
}

// writeJSONFile записывает значение через временный файл, чтобы при сбое не остался
// наполовину записанный JSON.
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
