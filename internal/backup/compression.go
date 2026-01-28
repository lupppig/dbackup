package backup

import "compress/gzip"

type GzipWriter struct {
	gzip *gzip.Writer
	base BackupWriter
}

func NewGzipWriter(base BackupWriter) *GzipWriter {
	return &GzipWriter{
		gzip: gzip.NewWriter(base),
		base: base,
	}
}

func (w *GzipWriter) Write(p []byte) (int, error) {
	return w.gzip.Write(p)
}

func (w *GzipWriter) Close() error {
	if err := w.gzip.Close(); err != nil {
		return err
	}
	return w.base.Close()
}

func (w *GzipWriter) Location() string {
	return w.base.Location() + ".gz"
}
