// Package hostinfo provides high-level details about the runtime OS.
package hostinfo

import (
	"runtime"
	"strings"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/osquery"
)

// Info encapsulates OS info.
type Info struct {
	// The platform is a high-level description of the OS. This maps to GOOS.
	Platform string `json:"platform" yaml:"platform"`

	// Arch maps to runtime.
	Arch string `json:"arch" yaml:"arch"`

	// The name of the kernel used by the operating system.
	Kernel string `json:"kernel,omitempty" yaml:"kernel,omitempty"`

	// The specific version of the kernel.
	KernelVersion string `json:"kernel_version,omitempty" yaml:"kernel_version,omitempty"`

	// The variant or distribution of the kernel.
	Variant string `json:"variant,omitempty" yaml:"variant,omitempty"`

	// The version of the variant.
	VariantVersion string `json:"variant_version,omitempty" yaml:"variant_version,omitempty"`
}

// LogValue implements slog.LogValuer.
func (si Info) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("platform", si.Platform),
		slog.String("arch", si.Arch),
		slog.String("kernel", si.Kernel),
		slog.String("kernel_version", si.KernelVersion),
		slog.String("variant", si.Variant),
		slog.String("variant_version", si.VariantVersion),
	)
}

// String returns a log-debug friendly representation.
func (si Info) String() string {
	sb := strings.Builder{}
	sb.WriteString(si.Platform)
	sb.WriteRune('|')
	sb.WriteString(si.Arch)
	if si.Kernel != "" {
		sb.WriteRune('|')
		sb.WriteString(si.Kernel)
		if si.KernelVersion != "" {
			sb.WriteRune(' ')
			sb.WriteString(si.KernelVersion)
		}
	}

	if si.Variant != "" {
		sb.WriteRune('|')
		sb.WriteString(si.Variant)
		if si.VariantVersion != "" {
			sb.WriteRune(' ')
			sb.WriteString(si.VariantVersion)
		}
	}

	return sb.String()
}

// Get returns system information. At a minimum, Info.Platform and
// Info.Arch are guaranteed to be populated.
func Get() Info {
	info := Info{
		Platform: runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	osq, err := osquery.Get()
	if err != nil || osq == nil {
		return info
	}

	const unk = "unknown"
	// osquery sets unknown fields to "unknown".
	// We'd rather they be empty.
	if osq.Kernel != unk {
		info.Kernel = osq.Kernel
	}
	if osq.KernelVersion != unk {
		info.KernelVersion = osq.KernelVersion
	}
	if osq.Variant != unk {
		info.Variant = osq.Variant
	}
	if osq.VariantVersion != unk {
		info.VariantVersion = osq.VariantVersion
	}

	return info
}
