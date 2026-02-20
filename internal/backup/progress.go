package backup

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"

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
	if n > 0 && pw.bar != nil {
		pw.bar.IncrBy(n)
	}
	return n, err
}

type ProgressReader struct {
	r   io.Reader
	bar *mpb.Bar
}

func NewProgressReader(r io.Reader, bar *mpb.Bar) *ProgressReader {
	return &ProgressReader{r: r, bar: bar}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 && pr.bar != nil {
		pr.bar.IncrBy(n)
	}
	return n, err
}

type ByteCounter struct {
	Count int64
}

func (bc *ByteCounter) Write(p []byte) (int, error) {
	n := len(p)
	bc.Count += int64(n)
	return n, nil
}

func NewProgressContainer() *mpb.Progress {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return nil
	}
	return mpb.New(mpb.WithWidth(64))
}

func AddBackupBar(p *mpb.Progress, name string) *mpb.Bar {
	if p == nil {
		return nil
	}
	return p.AddBar(0, // Indeterminate
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 4}),
			decor.CountersKibiByte("% .2f / % .2f", decor.WC{W: 18}),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.Spinner(nil, decor.WC{W: 5}), " [DONE]"),
			decor.OnComplete(decor.Name(" processed"), " [FINISHED]"),
		),
	)
}

func AddRestoreBar(p *mpb.Progress, name string, total int64) *mpb.Bar {
	if p == nil {
		return nil
	}
	return p.AddBar(total,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 4}),
			decor.Percentage(decor.WC{W: 8}),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.CountersKibiByte("% .2f / % .2f"),
				"DONE",
			),
		),
	)
}
