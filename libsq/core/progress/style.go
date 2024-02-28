package progress

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/runtimez"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	msgMaxLen = 36
	barWidth  = 28
	boxWidth  = 64
)

type alignment struct {
	msg     decor.WC
	counter decor.WC
	percent decor.WC
	timer   decor.WC
	mem     decor.WC
}

func newAlignment() *alignment {
	return &alignment{
		msg:     decor.WC{C: decor.DSyncWidth},
		counter: decor.WC{C: decor.DSyncWidth},
		percent: decor.WC{C: decor.DSyncWidth},
		timer:   decor.WC{C: decor.DSyncWidth},
		mem:     decor.WC{C: decor.DSyncWidth},
	}
}

// DefaultColors returns the default colors used for the progress bars.
func DefaultColors() *Colors {
	return &Colors{
		Error:    color.New(color.FgRed, color.Bold),
		Filler:   color.New(color.FgGreen, color.Bold, color.Faint),
		MemUsage: color.New(color.FgGreen, color.Faint),
		Message:  color.New(color.Faint),
		Percent:  color.New(color.FgCyan, color.Faint),
		Size:     color.New(color.Faint),
		Waiting:  color.New(color.FgYellow, color.Faint),
		Timer:    color.New(color.FgYellow, color.Faint),
		Warning:  color.New(color.FgYellow),
	}
}

// Colors is the set of colors used for the progress bars.
type Colors struct {
	Error    *color.Color
	Filler   *color.Color
	MemUsage *color.Color
	Message  *color.Color
	Percent  *color.Color
	Size     *color.Color
	Timer    *color.Color
	Waiting  *color.Color
	Warning  *color.Color
}

// EnableColor enables or disables color for the progress bars.
func (c *Colors) EnableColor(enable bool) {
	if c == nil {
		return
	}

	if enable {
		c.Error.EnableColor()
		c.Filler.EnableColor()
		c.MemUsage.EnableColor()
		c.Message.EnableColor()
		c.Percent.EnableColor()
		c.Size.EnableColor()
		c.Timer.EnableColor()
		c.Waiting.EnableColor()
		c.Warning.EnableColor()
		return
	}

	c.Error.DisableColor()
	c.Filler.DisableColor()
	c.MemUsage.DisableColor()
	c.Message.DisableColor()
	c.Percent.DisableColor()
	c.Size.DisableColor()
	c.Timer.DisableColor()
	c.Waiting.DisableColor()
	c.Warning.DisableColor()
}

func colorize(decorator decor.Decorator, c *color.Color) decor.Decorator {
	return decor.Meta(decorator, func(s string) string {
		return c.Sprint(s)
	})
}

func spinnerStyle(c *color.Color) mpb.SpinnerStyleComposer {
	// REVISIT: maybe use ascii chars only, in case it's a weird terminal?
	frames := []string{"∙∙∙", "●∙∙", "●∙∙", "∙●∙", "∙●∙", "∙∙●", "∙∙●", "∙∙∙"}
	style := mpb.SpinnerStyle(frames...)
	if c != nil {
		style = style.Meta(func(s string) string {
			return c.Sprint(s)
		})
	}
	return style
}

func barStyle(c *color.Color) mpb.BarStyleComposer {
	clr := func(s string) string {
		return c.Sprint(s)
	}

	frames := []string{"∙", "●", "●", "●", "∙"}
	return mpb.BarStyle().
		Lbound("  ").Rbound("  ").
		Filler("∙").FillerMeta(clr).
		Padding(" ").
		Tip(frames...).TipMeta(clr)
}

// OptTimer is an BarOpt that causes the bar to display elapsed seconds.
var OptTimer = optTimer{}

var _ BarOpt = optTimer{}

// wsprefix is the spacing prefix for widgets.

type optTimer struct{}

func (optTimer) apply(p *Progress, cfg *barConfig) {
	startTime := time.Now()
	fn := func(s decor.Statistics) string {
		var msg string
		if !s.Completed {
			msg = "   " + time.Since(startTime).Round(time.Second).String()
		}
		return msg
	}
	cfg.timerWidget = colorize(decor.Any(fn, p.align.timer), p.colors.Timer)
}

// OptMemUsage is an BarOpt that causes the bar to display program
// memory usage.
var OptMemUsage = optMemUsage{}

var _ BarOpt = optMemUsage{}

type optMemUsage struct{}

func (optMemUsage) apply(p *Progress, cfg *barConfig) {
	fn := func(s decor.Statistics) string {
		stats := runtimez.MemStats()
		return "  " + fmt.Sprintf("(% .1f)", decor.SizeB1000(stats.Sys))
	}
	cfg.memoryWidget = colorize(decor.Any(fn, p.align.mem), p.colors.MemUsage)
}

func nopWidget(p *Progress, wc decor.WC) decor.Decorator {
	return colorize(decor.Name("", wc), p.colors.Size) // Shouldn't matter which color we use
}

func msgWidth(msg string) string {
	switch {
	case len(msg) < msgMaxLen:
		msg += strings.Repeat(" ", msgMaxLen-len(msg))
	case len(msg) > msgMaxLen:
		msg = stringz.Ellipsify(msg, msgMaxLen)
	}
	return msg
}

func staticMsgWidget(p *Progress, msg string) decor.Decorator {
	return colorize(decor.Name(msgWidth(msg), p.align.msg), p.colors.Message)
}
