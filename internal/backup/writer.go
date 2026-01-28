package backup

import "io"

type BackupWriter interface {
	io.Writer
	Close() error
	Location() string
}




