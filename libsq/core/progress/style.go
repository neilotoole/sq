package progress

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/runtimez"
)

const (
	msgLength = 36
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

// var syncWidth = decor.WCSyncSpaceR
//var syncWidth = decor.WCSyncWidth

func nopWidget(c *color.Color, val string, wc decor.WC) decor.Decorator {
	return decor.Any(func(statistics decor.Statistics) string {
		return c.Sprint(val)
	}, wc)
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
	c.Waiting.DisableColor()
	c.Warning.DisableColor()
}

//func colorize(decorator decor.Decorator, c *color.Color) decor.Decorator {
//	return decor.Meta(decorator, func(s string) string {
//		return c.Sprint(s)
//	})
//}

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

type optTimer struct {
}

func (optTimer) apply(p *Progress, cfg *barConfig) {
	startTime := time.Now()
	//cfg.timerWidget = newElapsedSeconds(p.colors.Size, time.Now(), syncWidth)
	fn := func(s decor.Statistics) string {
		var msg string
		if !s.Completed {
			//msg = producer(time.Since(startTime))
			msg = time.Since(startTime).Round(time.Second).String()
			msg = p.colors.Size.Sprint(msg)
		}
		return msg
	}
	cfg.timerWidget = decor.Any(fn, p.align.timer)
}

// OptMemUsage is an BarOpt that causes the bar to display program
// memory usage.
var OptMemUsage = optMemUsage{}

var _ BarOpt = optMemUsage{}

type optMemUsage struct{}

func (optMemUsage) apply(p *Progress, cfg *barConfig) {
	fn := func(s decor.Statistics) string {
		stats := runtimez.MemStats()
		msg := fmt.Sprintf("xx (% .1f)", decor.SizeB1024(stats.Sys))
		return p.colors.MemUsage.Sprint(msg)
	}
	cfg.memoryWidget = decor.Any(fn, p.align.mem)

}
