package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"a-a/internal/utils"
)

func CreateFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer file.Close()

	return nil
}

func DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("could not delete file: %w", err)
	}

	return nil
}

func CreateFolder(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create folder: %w", err)
	}

	return nil
}

func DeleteFolder(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("could not delete folder: %w", err)
	}

	return nil
}

func WriteFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open or create file for writing: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("could not write to file: %w", err)
	}

	return nil
}

func ReadFile(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	return map[string]any{"content": string(content)}, nil
}

func ListDirectory(path string) (map[string]any, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("could not list directory: %w", err)
	}
	var entryNames []string
	for _, e := range entries {
		entryNames = append(entryNames, e.Name())
	}
	return map[string]any{"entries": entryNames}, nil
}

func WriteFileAtomic(path string, content string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("atomic: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("atomic: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("atomic: close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic: rename: %w", err)
	}
	return nil
}

func HandleSystemAction(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	path, err := utils.GetStringPayload(payload, "path")
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch operation {
	case "create_file":
		return nil, CreateFile(path)
	case "delete_file":
		return nil, DeleteFile(path)
	case "create_folder":
		return nil, CreateFolder(path)
	case "delete_folder":
		return nil, DeleteFolder(path)
	case "write_file":
		content, err := utils.GetStringPayload(payload, "content")
		if err != nil {
			return nil, err
		}
		return nil, WriteFile(path, content)
	case "write_file_atomic":
		content, err := utils.GetStringPayload(payload, "content")
		if err != nil {
			return nil, err
		}
		return nil, WriteFileAtomic(path, content)
	case "read_file":
		return ReadFile(path)
	case "list_directory":
		return ListDirectory(path)
	default:
		return nil, fmt.Errorf("unknown system operation: %s", operation)
	}
}
