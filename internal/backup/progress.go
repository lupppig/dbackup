package backup

import (
	"io"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// ProgressWriter tracks bytes written and updates an mpb.Bar.
type ProgressWriter struct {
	w   io.Writer
	bar *mpb.Bar
}

func NewProgressWriter(w io.Writer, bar *mpb.Bar) *ProgressWriter {
	return &ProgressWriter{w: w, bar: bar}
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if n > 0 {
		pw.bar.IncrBy(n)
	}
	return n, err
}

// ProgressReader tracks bytes read and updates an mpb.Bar.
type ProgressReader struct {
	r   io.Reader
	bar *mpb.Bar
}

func NewProgressReader(r io.Reader, bar *mpb.Bar) *ProgressReader {
	return &ProgressReader{r: r, bar: bar}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.bar.IncrBy(n)
	}
	return n, err
}

// ByteCounter counts bytes without a UI bar, used for manifest size tracking.
type ByteCounter struct {
	Count int64
}

func (bc *ByteCounter) Write(p []byte) (int, error) {
	n := len(p)
	bc.Count += int64(n)
	return n, nil
}

func NewProgressContainer() *mpb.Progress {
	// In the future, we can add a check for os.Stdout TTY status
	return mpb.New(mpb.WithWidth(64))
}

func AddBackupBar(p *mpb.Progress, name string) *mpb.Bar {
	if p == nil {
		return nil
	}
	return p.AddBar(0, // Indeterminate
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1}),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.Name(" processed"),
			decor.OnComplete(decor.Name(" [DONE]"), " [FINISH]"),
		),
	)
}

func AddRestoreBar(p *mpb.Progress, name string, total int64) *mpb.Bar {
	if p == nil {
		return nil
	}
	return p.AddBar(total,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1}),
			decor.Percentage(),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.CountersKibiByte("% .2f / % .2f"),
				"DONE",
			),
		),
	)
}
