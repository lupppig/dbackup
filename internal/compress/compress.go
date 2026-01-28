package compress

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

type Algorithm string

const (
	Gzip Algorithm = "gzip"
	Lz4  Algorithm = "lz4"
	Zstd Algorithm = "zstd"
	None Algorithm = "none"
	Tar  Algorithm = "tar"
)

type Compressor struct {
	Writer     io.Writer
	Tar        *tar.Writer
	compWriter io.Writer
	algo       Algorithm
	closer     io.Closer
	location   string
	tmpFile    *os.File
	bufferName string
	mu         sync.Mutex
}

func New(w io.Writer, algo Algorithm) (*Compressor, error) {
	if algo == "" {
		algo = Lz4
	}

	c := &Compressor{
		algo:   algo,
		Writer: w,
	}

	if lw, ok := w.(interface{ Location() string }); ok {
		c.location = lw.Location()
	}

	if algo == None {
		return c, nil
	}

	c.Tar = tar.NewWriter(w)

	tmp, err := os.CreateTemp("", "dbackup-comp-*")
	if err != nil {
		return nil, err
	}
	c.tmpFile = tmp

	switch algo {
	case Gzip:
		gz := gzip.NewWriter(tmp)
		c.compWriter = gz
		c.closer = gz
	case Lz4:
		l := lz4.NewWriter(tmp)
		c.compWriter = l
		c.closer = l
	case Zstd:
		z, err := zstd.NewWriter(tmp)
		if err != nil {
			return nil, err
		}
		c.compWriter = z
		c.closer = z
	case Tar:
		c.compWriter = nil
	default:
		return nil, ErrUnsupportedAlgo(algo)
	}

	return c, nil
}

func (c *Compressor) SetTarBufferName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bufferName = name
}

func (c *Compressor) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.algo == None {
		return c.Writer.Write(p)
	}

	if c.compWriter != nil {
		return c.compWriter.Write(p)
	}
	return c.tmpFile.Write(p)
}

func (c *Compressor) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.algo == None {
		if cl, ok := c.Writer.(io.Closer); ok {
			return cl.Close()
		}
		return nil
	}

	if c.closer != nil {
		if err := c.closer.Close(); err != nil {
			return err
		}
	}

	if c.Tar != nil && c.tmpFile != nil {
		info, err := c.tmpFile.Stat()
		if err != nil {
			return err
		}
		size := info.Size()

		if _, err := c.tmpFile.Seek(0, 0); err != nil {
			return err
		}

		name := c.bufferName
		if name == "" {
			name = "backup.sql"
		}
		switch c.algo {
		case Gzip:
			name += ".gz"
		case Lz4:
			name += ".lz4"
		case Zstd:
			name += ".zst"
		}

		hdr := &tar.Header{
			Name: name,
			Size: size,
			Mode: 0600,
		}
		if err := c.Tar.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(c.Tar, c.tmpFile); err != nil {
			return err
		}

		c.tmpFile.Close()
		os.Remove(c.tmpFile.Name())
		c.tmpFile = nil

		if err := c.Tar.Close(); err != nil {
			return err
		}
	}

	if b, ok := c.Writer.(io.Closer); ok && b != c.Tar {
		return b.Close()
	}

	return nil
}

func (c *Compressor) Location() string {
	return c.location
}

type ErrUnsupportedAlgo Algorithm

func (e ErrUnsupportedAlgo) Error() string {
	return "unsupported compression algorithm: " + string(e)
}

var ErrTarNotEnabled = &customError{"tar mode not enabled"}

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }
