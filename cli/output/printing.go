package output

import (
	"log/slog"
	"time"

	"github.com/fatih/color"
	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/timez"
)

// Printing describes color and pretty-printing options.
type Printing struct {
	// FormatDatetime formats a timestamp e.g. 2020-11-12T13:14:15Z.
	// Defaults to timez.DefaultDatetime.
	FormatDatetime func(time time.Time) string

	// FormatTime formats a time of day, e.g. 13:14:15.
	// Defaults to timez.DefaultTime.
	FormatTime func(time time.Time) string

	// FormatDate formats a date, e.g. 2020-11-12.
	// Defaults to timez.DefaultDate.
	FormatDate func(time time.Time) string

	// Active is the color for an active handle (or group, etc).
	Active *color.Color

	// Bold is the color for bold elements. Frequently Punc will just be color.Bold.
	Bold *color.Color

	// Bool is the color for boolean values.
	Bool *color.Color

	// Bytes is the color for byte / binary values.
	Bytes *color.Color

	// Datetime is the color for time-related values.
	Datetime *color.Color

	// Diff contains colors for diff output.
	Diff *diffdoc.Colors

	// Disabled is the color for disabled elements.
	Disabled *color.Color

	// Duration is the color for time duration values.
	Duration *color.Color

	// Enabled is the color for enabled elements.
	Enabled *color.Color

	// Error is the color for error elements such as an error message.
	Error *color.Color

	// Faint is the color for faint elements - the opposite of Hilite.
	Faint *color.Color

	// Handle is the color for source handles such as "@sakila"
	Handle *color.Color

	// Header is the color for header elements in a table.
	Header *color.Color

	// Hilite is the color for highlighted elements.
	Hilite *color.Color

	// Key is the color for keys such as a JSON field name.
	Key *color.Color

	// Location is the color for Source.Location values.
	Location *color.Color

	// Normal is the default color.
	Normal *color.Color

	// Null is the color for null.
	Null *color.Color

	// Number is the color for number values, including int, float, decimal etc.
	Number *color.Color

	// Punc is the color for punctuation such as colons, braces, etc.
	Punc *color.Color

	// Stack is the color for stack traces.
	Stack *color.Color

	// StackError is the color for errors attached to a stack trace.
	StackError *color.Color

	// StackErrorType is the color for the error types attached to a stack trace.
	StackErrorType *color.Color

	// String is the color for string values.
	String *color.Color

	// Success is the color for success elements.
	Success *color.Color

	// Warning is the color for warning elements.
	Warning *color.Color

	// NewBufferFn returns a new [ioz.Buffer] instance; it should be preferred
	// over [bytes.Buffer] when dealing large/unbounded data.
	NewBufferFn func() ioz.Buffer

	// Indent is the indent string to use when pretty-printing,
	// typically two spaces.
	Indent string

	// ExcelDatetimeFormat is the format string for datetime values.
	// See excelw.OptDatetimeFormat.
	ExcelDatetimeFormat string

	// ExcelDateFormat is the format string for date values.
	// See excelw.OptDateFormat.
	ExcelDateFormat string

	// ExcelTimeFormat is the format string for time values.
	// See excelw.OptTimeFormat.
	ExcelTimeFormat string

	// FlushThreshold is the size in bytes after which an output writer
	// should flush any internal buffer.
	FlushThreshold int

	// monochrome is controlled by EnableColor.
	monochrome bool

	// ShowHeader indicates that a header (e.g. a header row) should
	// be printed where applicable.
	ShowHeader bool

	// Verbose indicates that verbose output should be printed where
	// applicable.
	Verbose bool

	// Compact indicates that output should not be pretty-printed.
	// Typically this means indentation, new lines, etc., but
	// varies by output format.
	Compact bool

	// Redact indicates that sensitive fields (such as passwords)
	// should be redacted (hidden/masked).
	Redact bool

	// FormatDatetimeAsNumber indicates that datetime values should be
	// rendered as naked numbers (instead of as a string) if possible.
	// See cli.OptDatetimeFormatAsNumber.
	FormatDatetimeAsNumber bool

	// FormatTimeAsNumber indicates that time values should be
	// rendered as naked numbers (instead of as a string) if possible.
	// See cli.OptTimeFormatAsNumber.
	FormatTimeAsNumber bool

	// FormatDateAsNumber indicates that date values should be
	// rendered as naked numbers (instead of as a string) if possible.
	// See cli.OptDateFormatAsNumber.
	FormatDateAsNumber bool
}

// NewPrinting returns a Printing instance. Color and pretty-print
// are enabled. The default indent is two spaces.
func NewPrinting() *Printing {
	pr := &Printing{
		NewBufferFn:            ioz.NewDefaultBuffer,
		ShowHeader:             true,
		Verbose:                false,
		Compact:                false,
		Redact:                 true,
		FlushThreshold:         1000,
		FormatDatetime:         timez.FormatFunc(timez.DefaultDatetime),
		FormatDatetimeAsNumber: false,
		FormatTime:             timez.FormatFunc(timez.DefaultTime),
		FormatTimeAsNumber:     false,
		FormatDate:             timez.FormatFunc(timez.DefaultDate),
		FormatDateAsNumber:     false,
		monochrome:             false,
		Indent:                 "  ",
		Active:                 color.New(color.FgGreen, color.Bold),
		Bold:                   color.New(color.Bold),
		Bool:                   color.New(color.FgYellow),
		Bytes:                  color.New(color.Faint),
		Datetime:               color.New(color.FgGreen, color.Faint),
		Diff:                   diffdoc.NewColors(),
		Disabled:               color.New(color.FgYellow, color.Faint),
		Duration:               color.New(color.FgGreen, color.Faint),
		Enabled:                color.New(color.FgGreen, color.Faint),
		Error:                  color.New(color.FgRed, color.Bold),
		Faint:                  color.New(color.Faint),
		Handle:                 color.New(color.FgBlue),
		Header:                 color.New(color.FgBlue),
		Hilite:                 color.New(color.FgHiBlue),
		Key:                    color.New(color.FgBlue, color.Bold),
		Location:               color.New(color.FgGreen),
		Normal:                 color.New(),
		Null:                   color.New(color.Faint),
		Number:                 color.New(color.FgCyan),
		Punc:                   color.New(color.Bold),
		String:                 color.New(color.FgGreen),
		Stack:                  color.New(color.Faint),
		StackError:             color.New(color.FgYellow, color.Faint),
		StackErrorType:         color.New(color.FgGreen, color.Faint),
		Success:                color.New(color.FgGreen, color.Bold),
		Warning:                color.New(color.FgYellow),
	}

	pr.EnableColor(true)
	return pr
}

// Clone returns a clone of pr.
func (pr *Printing) Clone() *Printing {
	pr2 := &Printing{
		NewBufferFn:            pr.NewBufferFn,
		monochrome:             pr.monochrome,
		FlushThreshold:         pr.FlushThreshold,
		ShowHeader:             pr.ShowHeader,
		Verbose:                pr.Verbose,
		Compact:                pr.Compact,
		Indent:                 pr.Indent,
		Redact:                 pr.Redact,
		FormatDatetime:         pr.FormatDatetime,
		FormatDatetimeAsNumber: pr.FormatDatetimeAsNumber,
		FormatTime:             pr.FormatTime,
		FormatTimeAsNumber:     pr.FormatTimeAsNumber,
		FormatDate:             pr.FormatDate,
		FormatDateAsNumber:     pr.FormatDateAsNumber,
		Diff:                   pr.Diff.Clone(),
	}

	pr2.Active = lo.ToPtr(*pr.Active)
	pr2.Bold = lo.ToPtr(*pr.Bold)
	pr2.Bool = lo.ToPtr(*pr.Bool)
	pr2.Bytes = lo.ToPtr(*pr.Bytes)
	pr2.Datetime = lo.ToPtr(*pr.Datetime)
	pr2.Disabled = lo.ToPtr(*pr.Disabled)
	pr2.Duration = lo.ToPtr(*pr.Duration)
	pr2.Enabled = lo.ToPtr(*pr.Enabled)
	pr2.Error = lo.ToPtr(*pr.Error)
	pr2.Faint = lo.ToPtr(*pr.Faint)
	pr2.Handle = lo.ToPtr(*pr.Handle)
	pr2.Header = lo.ToPtr(*pr.Header)
	pr2.Hilite = lo.ToPtr(*pr.Hilite)
	pr2.Key = lo.ToPtr(*pr.Key)
	pr2.Location = lo.ToPtr(*pr.Location)
	pr2.Null = lo.ToPtr(*pr.Null)
	pr2.Normal = lo.ToPtr(*pr.Normal)
	pr2.Number = lo.ToPtr(*pr.Number)
	pr2.Punc = lo.ToPtr(*pr.Punc)
	pr2.String = lo.ToPtr(*pr.String)
	pr2.Success = lo.ToPtr(*pr.Success)
	pr2.Stack = lo.ToPtr(*pr.Stack)
	pr2.StackError = lo.ToPtr(*pr.StackError)
	pr2.StackErrorType = lo.ToPtr(*pr.StackErrorType)
	pr2.Warning = lo.ToPtr(*pr.Warning)

	return pr2
}

// LogValue implements slog.LogValuer.
func (pr *Printing) LogValue() slog.Value {
	if pr == nil {
		return slog.Value{}
	}

	// REVISIT: Should we output all Printing values here?
	return slog.GroupValue(
		slog.Bool("verbose", pr.Verbose),
		slog.Bool("header", pr.ShowHeader),
		slog.Bool("monochrome", pr.monochrome),
		slog.Bool("compact", pr.Compact),
		slog.Bool("redact", pr.Redact),
		slog.Int("flush-threshold", pr.FlushThreshold),
		slog.String("indent", pr.Indent),
		slog.Bool("format.datetime.number", pr.FormatDatetimeAsNumber),
		slog.Bool("format.date.number", pr.FormatDateAsNumber),
		slog.Bool("format.time.number", pr.FormatTimeAsNumber),
	)
}

func (pr *Printing) colors() []*color.Color {
	return []*color.Color{
		pr.Active,
		pr.Bold,
		pr.Bool,
		pr.Bytes,
		pr.Datetime,
		pr.Disabled,
		pr.Duration,
		pr.Enabled,
		pr.Error,
		pr.Faint,
		pr.Handle,
		pr.Header,
		pr.Hilite,
		pr.Key,
		pr.Location,
		pr.Normal,
		pr.Null,
		pr.Number,
		pr.Punc,
		pr.Stack,
		pr.StackError,
		pr.StackErrorType,
		pr.String,
		pr.Success,
		pr.Warning,
	}
}

// IsMonochrome returns true if in monochrome (no color) mode.
// Default is false (color enabled) for a new instance.
func (pr *Printing) IsMonochrome() bool {
	return pr.monochrome
}

// EnableColor enables or disables all colors.
func (pr *Printing) EnableColor(enable bool) {
	pr.Diff.EnableColor(enable)

	if enable {
		pr.monochrome = false
		for _, clr := range pr.colors() {
			clr.EnableColor()
		}
		return
	}

	pr.monochrome = true
	for _, clr := range pr.colors() {
		clr.DisableColor()
	}
}
