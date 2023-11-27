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
	P *mpb.Progress
}

func New(ctx context.Context, out io.Writer, delay time.Duration) *Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithOutput(out),
		mpb.WithWidth(64),
		mpb.WithRenderDelay(renderDelay(delay)),
	)

	return &Progress{P: p}
}

func renderDelay(d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(d, func() {
		close(ch)
	})
	return ch
}

func NewProxyBar(p *mpb.Progress) {
	bar := p.New(0,
		mpb.BarStyle().Rbound("|"),
		mpb.PrependDecorators(
			decor.Counters(decor.SizeB1024(0), "% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 30),
			decor.Name(" ] "),
			decor.EwmaSpeed(decor.SizeB1024(0), "% .2f", 30),
		),
	)

	_ = bar
}

func ColorMeta(decorator decor.Decorator, c *color.Color) decor.Decorator {
	return decor.Meta(decorator, func(s string) string {
		return c.Sprint(s)
	})
}
