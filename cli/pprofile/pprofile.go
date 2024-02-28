// Package pprofile encapsulates pprof functionality.
package pprofile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pkg/profile"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Modes returns the allowed pprof modes, including ModeNode.
func Modes() []string {
	return slices.Clone(modes)
}

var modes = []string{
	ModeOff,
	ModeCPU,
	ModeMem,
	ModeBlock,
	ModeMutex,
	ModeTrace,
	ModeThread,
	ModeGoroutine,
}

// Available pprof modes.
const (
	ModeOff       = "off"
	ModeCPU       = "cpu"
	ModeMem       = "mem"
	ModeBlock     = "block"
	ModeMutex     = "mutex"
	ModeTrace     = "trace"
	ModeThread    = "thread"
	ModeGoroutine = "goroutine"
)

// validMode returns an error if mode is not a valid pprof mode.
func validMode(mode string) error {
	if slices.Contains(modes, mode) {
		return nil
	}
	return errz.Errorf("invalid pprof mode: %s", mode)
}

var OptMode = options.NewString(
	"debug.pprof",
	&options.Flag{},
	ModeOff,
	validMode,
	"pprof profiling mode",
	`Configure pprof profiling, writing output to SQ_CONFIG/pprof. Allowed modes are:

  `+strings.Join(modes, ", "),
)

// Start starts pprof profiling, writing output to pprofDir, as well as a copy
// to pprofDir/history. If err is nil, the caller must invoke the returned stop
// func to stop profiling, typically in a defer.
func Start(ctx context.Context, mode, pprofDir string) (stop func(), err error) {
	log := lg.FromContext(ctx)

	if err = validMode(mode); err != nil {
		return nil, err
	}
	if mode == ModeOff {
		return func() {}, nil
	}

	timestamp := time.Now().Format("20060102_150405")
	tmpDir := filepath.Join(pprofDir, fmt.Sprintf("tmp_%s_%d_%s", timestamp, os.Getpid(), stringz.Uniq8()))
	if err = ioz.RequireDir(tmpDir); err != nil {
		return nil, err
	}

	ppath := profile.ProfilePath(tmpDir)

	// The profile package returns an interface with a single method, Stop.
	type profiler interface{ Stop() }
	var p profiler

	// fname is the base name of the output file that profile.Start produces.
	// It varies by mode; we'll capture it in the switch below.
	var fname string

	log.Debug("Starting pprof", lga.Mode, mode, lga.Path, pprofDir)
	switch mode {
	case ModeCPU:
		fname = "cpu.pprof"
		// Delete the current pprof file, if any.
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.CPUProfile, ppath, profile.NoShutdownHook)
	case ModeMem:
		fname = "mem.pprof"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.MemProfile, ppath, profile.NoShutdownHook)
	case ModeBlock:
		fname = "block.pprof"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.BlockProfile, ppath, profile.NoShutdownHook)
	case ModeMutex:
		fname = "mutex.pprof"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.MutexProfile, ppath, profile.NoShutdownHook)
	case ModeTrace:
		fname = "trace.out"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.TraceProfile, ppath, profile.NoShutdownHook)
	case ModeThread:
		fname = "threadcreation.pprof"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.ThreadcreationProfile, ppath, profile.NoShutdownHook)
	case ModeGoroutine:
		fname = "goroutine.pprof"
		_ = os.Remove(filepath.Join(pprofDir, fname))
		p = profile.Start(profile.GoroutineProfile, ppath, profile.NoShutdownHook)
	default:
		// Shouldn't be possible.
		panic(errz.Errorf("unknown pprof mode {%s}", mode))
	}

	stop = func() {
		defer func() { lg.WarnIfError(log, "Remove pprof temp dir", os.RemoveAll(tmpDir)) }()
		p.Stop()

		var (
			tmpFile     = filepath.Join(tmpDir, fname)
			destFile    = filepath.Join(pprofDir, fname)
			historyFile = filepath.Join(pprofDir, "history", timestamp+"-"+fname)
		)

		if err = ioz.CopyFile(historyFile, tmpFile, true); err != nil {
			log.Error("Failed to copy pprof history file", lga.From, tmpFile, lga.To, historyFile, lga.Err, err)
		}
		if err = ioz.CopyFile(destFile, tmpFile, true); err != nil {
			log.Error("Failed to copy pprof file", lga.From, tmpFile, lga.To, destFile, lga.Err, err)
			return
		}

		log.Info("Wrote pprof", lga.Mode, mode, lga.Path, destFile)
	}
	return stop, nil
}
