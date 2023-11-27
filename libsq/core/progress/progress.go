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

	//return ctx.Value(runKey{}).(*Progress)
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
	p      *mpb.Progress
	colors *Colors
}

// Wait waits for all bars to complete and finally shutdowns container. After
// this method has been called, there is no way to reuse `*Progress` instance.
func (p *Progress) Wait() {
	p.p.Wait()
}

//// Shutdown cancels any running bar immediately and then shutdowns `*Progress`
//// instance. Normally this method shouldn't be called unless you know what you
//// are doing. Proper way to shutdown is to call `(*Progress).Wait()` instead.
//func (p *Progress) Shutdown() {
//	p.p.Shutdown()
//}

func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithOutput(out),
		mpb.WithWidth(64),
		mpb.WithRenderDelay(renderDelay(delay)),
	)

	if colors == nil {
		colors = DefaultColors()
	}

	return &Progress{p: p, colors: colors}
}

func (p *Progress) NewIOSpinner(msg string) *IOSpinner {
	if p == nil {
		return nil
	}
	style := mpb.SpinnerStyle("∙∙∙", "●∙∙", "∙●∙", "∙∙●", "∙∙∙")
	style = style.Meta(func(s string) string {
		return p.colors.Spinner.Sprint(s)
	})

	bar := p.p.New(0,
		style,
		mpb.BarWidth(36),
		mpb.PrependDecorators(
			ColorMeta(decor.Name(msg), p.colors.Message),
		),
		mpb.AppendDecorators(
			ColorMeta(decor.Current(decor.SizeB1024(0), "% .1f"), p.colors.Message),
		),
		mpb.BarRemoveOnComplete(),
	)

	return &IOSpinner{bar: bar}
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

func (sp *IOSpinner) Finish() {
	if sp == nil {
		return
	}
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
