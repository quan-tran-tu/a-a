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
	fmt.Printf("File created: %s\n", path)
	return nil
}

func DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("could not delete file: %w", err)
	}
	fmt.Printf("File deleted: %s\n", path)
	return nil
}

func CreateFolder(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create folder: %w", err)
	}
	fmt.Printf("Folder created: %s\n", path)
	return nil
}

func DeleteFolder(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("could not delete folder: %w", err)
	}
	fmt.Printf("Folder and its contents deleted: %s\n", path)
	return nil
}
