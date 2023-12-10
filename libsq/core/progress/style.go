package progress

import (
	"time"

	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	msgLength   = 28
	barWidth    = 28
	boxWidth    = 64
	refreshRate = 150 * time.Millisecond
)

// DefaultColors returns the default colors used for the progress bars.
func DefaultColors() *Colors {
	return &Colors{
		Error:   color.New(color.FgRed, color.Bold),
		Filler:  color.New(color.FgGreen, color.Bold, color.Faint),
		Message: color.New(color.Faint),
		Percent: color.New(color.FgCyan, color.Faint),
		Size:    color.New(color.Faint),
		Waiting: color.New(color.FgYellow, color.Faint),
		Warning: color.New(color.FgYellow),
	}
}

// Colors is the set of colors used for the progress bars.
type Colors struct {
	Error   *color.Color
	Filler  *color.Color
	Message *color.Color
	Percent *color.Color
	Size    *color.Color
	Waiting *color.Color
	Warning *color.Color
}

// EnableColor enables or disables color for the progress bars.
func (c *Colors) EnableColor(enable bool) {
	if c == nil {
		return
	}

	if enable {
		c.Error.EnableColor()
		c.Filler.EnableColor()
		c.Message.EnableColor()
		c.Percent.EnableColor()
		c.Size.EnableColor()
		c.Waiting.EnableColor()
		c.Warning.EnableColor()

		return
	}

	c.Error.DisableColor()
	c.Filler.DisableColor()
	c.Message.DisableColor()
	c.Percent.DisableColor()
	c.Size.DisableColor()
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
