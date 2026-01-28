package backup

import (
	"os"
	"path/filepath"
)

type FileWriter struct {
	file *os.File
	path string
}

func NewFileWriter(dir, name string) (*FileWriter, error) {
	if dir == "" {
		dir = "./"
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	return &FileWriter{file: f, path: path}, nil
}

func (w *FileWriter) Write(p []byte) (int, error) {
	return w.file.Write(p)
}

func (w *FileWriter) Close() error {
	return w.file.Close()
}

func (w *FileWriter) Location() string {
	return w.path
}
