package progress

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
	"sync"
	"time"

	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/cleanup"
)

type runKey struct{}

// NewContext returns ctx with prog added as a value.
func NewContext(ctx context.Context, prog *Progress) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runKey{}, prog)
}

// FromContext extracts the Progress added to ctx via NewContext.
func FromContext(ctx context.Context) *Progress {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(runKey{})
	if val == nil {
		return nil
	}

	if p, ok := val.(*Progress); ok {
		return p
	}

	return nil

	// return ctx.Value(runKey{}).(*Progress)
}

func DefaultColors() *Colors {
	return &Colors{
		Message: color.New(color.Faint),
		Spinner: color.New(color.FgGreen, color.Bold),
		Size:    color.New(color.Faint),
	}
}

type Colors struct {
	Message *color.Color
	Spinner *color.Color
	Size    *color.Color
}

func (c *Colors) EnableColor(enable bool) {
	if c == nil {
		return
	}

	if enable {
		c.Message.EnableColor()
		c.Spinner.EnableColor()
		c.Size.EnableColor()
		return
	}

	c.Message.DisableColor()
	c.Spinner.DisableColor()
	c.Size.DisableColor()
}

type Progress struct {
	p       *mpb.Progress
	mu      *sync.Mutex
	colors  *Colors
	cleanup *cleanup.Cleanup
}

// Wait waits for all bars to complete and finally shutdowns container. After
// this method has been called, there is no way to reuse `*Progress` instance.
func (p *Progress) Wait() {
	p.mu.Lock()
	defer p.mu.Unlock()

	_ = p.cleanup.Run()
	p.p.Wait()
}

func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithOutput(out),
		mpb.WithWidth(64),
		mpb.WithRenderDelay(renderDelay(delay)),
	)

	if colors == nil {
		colors = DefaultColors()
	}

	return &Progress{p: p, colors: colors, mu: &sync.Mutex{}, cleanup: cleanup.New()}
}

// ShutdownOnWriteTo returns a writer that will stop the
// progress.Progress when w.Write is called. Typically p writes
// to stderr, and stdout is passed to this method. That is, when
// the program starts writing to stdout, we want to shut down
// and remove the progress bar.
func (p *Progress) ShutdownOnWriteTo(w io.Writer) io.Writer {
	// REVISIT: Should we check if w implements other io interfaces,
	// such as io.WriterAt etc? Or do we really only care about io.Writer?
	if p == nil {
		return w
	}
	return &writeNotifier{
		p: p,
		w: w,
	}
}

var _ io.Writer = (*writeNotifier)(nil)

type writeNotifier struct {
	p          *Progress
	w          io.Writer
	notifyOnce sync.Once
}

// Write implements [io.Writer].
func (w *writeNotifier) Write(p []byte) (n int, err error) {
	w.notifyOnce.Do(w.p.Wait)

	return w.w.Write(p)
}

func normalizeMsgLength(msg string, length int) string {
	if len(msg) > length {
		msg = stringz.TrimLenMiddle(msg, length)
	}

	return fmt.Sprintf("%-*s", length, msg)
}

// NewIOSpinner returns a new indeterminate spinner bar whose metric is
// the count of bytes processed. The caller is ultimately
// responsible for calling [IOSpinner.Stop] on the returned IOSpinner. However,
// the returned IOSpinner is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the spinner's work.
func (p *Progress) NewIOSpinner(msg string) *IOSpinner {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	const (
		msgLength = 18
		barWidth  = 28
	)

	msg = normalizeMsgLength(msg, msgLength)

	style := mpb.SpinnerStyle("∙∙∙", "●∙∙", "∙●∙", "∙∙●", "∙∙∙")
	style = style.Meta(func(s string) string {
		return p.colors.Spinner.Sprint(s)
	})

	bar := p.p.New(0,
		style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(
			ColorMeta(decor.Name(msg), p.colors.Message),
		),
		mpb.AppendDecorators(
			ColorMeta(decor.Current(decor.SizeB1024(0), "% .1f"), p.colors.Message),
		),
		mpb.BarRemoveOnComplete(),
	)

	spinner := &IOSpinner{bar: bar}
	p.cleanup.Add(spinner.Stop)
	return spinner
}

type IOSpinner struct {
	bar *mpb.Bar
}

func (sp *IOSpinner) IncrBy(n int) {
	if sp == nil {
		return
	}
	sp.bar.IncrBy(n)
}

func (sp *IOSpinner) Stop() {
	if sp == nil {
		return
	}

	sp.bar.SetTotal(-1, true)
	sp.bar.Wait()
}

func renderDelay(d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(d, func() {
		close(ch)
	})
	return ch
}

func ColorMeta(decorator decor.Decorator, c *color.Color) decor.Decorator {
	return decor.Meta(decorator, func(s string) string {
		return c.Sprint(s)
	})
}
