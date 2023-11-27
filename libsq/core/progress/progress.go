package progress

import (
	"context"
	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"io"
	"time"
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
	return ctx.Value(runKey{}).(*Progress)
}

type Progress struct {
	P      *mpb.Progress
	Colors *Colors
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

func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithOutput(out),
		mpb.WithWidth(64),
		mpb.WithRenderDelay(renderDelay(delay)),
	)

	if colors == nil {
		colors = DefaultColors()
	}

	return &Progress{P: p, Colors: colors}
}

func (p *Progress) NewIOSpinner(msg string) *IOSpinner {
	s := mpb.SpinnerStyle("∙∙∙", "●∙∙", "∙●∙", "∙∙●", "∙∙∙")
	s = s.Meta(func(s string) string {
		return p.Colors.Spinner.Sprint(s)
	})
	bar := p.P.New(0,
		s,
		mpb.BarWidth(36),
		mpb.PrependDecorators(
			ColorMeta(decor.Name(msg), p.Colors.Message),
		),
		mpb.AppendDecorators(
			ColorMeta(decor.Current(decor.SizeB1024(0), "% .1f"), p.Colors.Message),
		),
		mpb.BarRemoveOnComplete(),
	)

	return &IOSpinner{bar: bar}
}

type IOSpinner struct {
	bar *mpb.Bar
}

func (sp *IOSpinner) IncrBy(n int) {
	sp.bar.IncrBy(n)
}

func (sp *IOSpinner) Finish() {
	sp.bar.SetTotal(-1, true)
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
