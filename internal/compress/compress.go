package compress

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"sync"

	"strings"

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

	// For very large datasets, we avoid TAR and temp files unless explicitly requested.
	// If the user wants a single compressed stream, they get it directly.
	if algo == Tar {
		c.Tar = tar.NewWriter(w)
		return c, nil
	}

	// Direct streaming compression
	switch algo {
	case Gzip:
		gz := gzip.NewWriter(w)
		c.compWriter = gz
		c.closer = gz
	case Lz4:
		l := lz4.NewWriter(w)
		c.compWriter = l
		c.closer = l
	case Zstd:
		z, err := zstd.NewWriter(w)
		if err != nil {
			return nil, err
		}
		c.compWriter = z
		c.closer = z
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

	if c.algo == Tar {
		return 0, fmt.Errorf("direct streaming to TAR is not supported without a temp file (to calculate size); use a specific compression algorithm like LZ4 or Gzip for streaming")
	}

	return 0, fmt.Errorf("compressor not initialized for algorithm: %s", c.algo)
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

	if c.algo == Tar && c.Tar != nil {
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

type Decompressor struct {
	io.Reader
	closer io.Closer
	tar    *tar.Reader
}

func NewReader(r io.Reader, algo Algorithm) (*Decompressor, error) {
	if algo == "" || algo == None {
		return &Decompressor{Reader: r}, nil
	}

	var decomp io.Reader
	var closer io.Closer

	// Important: Our New() now streams directly.
	// If the user wants to decompress, we should wrap the reader 'r' directly.
	switch algo {
	case Gzip:
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		decomp = gz
		closer = gz
	case Lz4:
		l := lz4.NewReader(r)
		decomp = l
	case Zstd:
		z, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		decomp = z
		closer = z.IOReadCloser()
	case Tar:
		tr := tar.NewReader(r)
		_, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		decomp = tr
	default:
		return nil, ErrUnsupportedAlgo(algo)
	}

	return &Decompressor{
		Reader: decomp,
		closer: closer,
	}, nil
}

func DetectAlgorithm(filename string) Algorithm {
	if strings.HasSuffix(filename, ".gz") {
		return Gzip
	}
	if strings.HasSuffix(filename, ".lz4") {
		return Lz4
	}
	if strings.HasSuffix(filename, ".zst") {
		return Zstd
	}
	if strings.HasSuffix(filename, ".tar") {
		return Tar
	}
	return None
}

func (d *Decompressor) Close() error {
	if d.closer != nil {
		return d.closer.Close()
	}
	return nil
}

type ErrUnsupportedAlgo Algorithm

func (e ErrUnsupportedAlgo) Error() string {
	return "unsupported compression algorithm: " + string(e)
}

var ErrTarNotEnabled = &customError{"tar mode not enabled"}

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }
