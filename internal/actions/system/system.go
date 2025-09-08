package system

import (
	"fmt"
	"os"
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

	if _, err := file.WriteString(content + "\n"); err != nil {
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
