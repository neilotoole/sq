package yamlstore

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/pflag"
)

// Load loads sq config from the default location (~/.config/sq/sq.yml) or
// the location specified in envars or flags.
func Load(ctx context.Context, osArgs []string, optsReg *options.Registry,
	upgrades UpgradeRegistry,
) (*config.Config, config.Store, error) {
	var (
		cfgDir string
		origin string
		ok     bool
		err    error
	)

	_ = options.Registry{}

	if cfgDir, ok, _ = getConfigDirFromFlag(osArgs); ok {
		origin = originFlag
	} else if cfgDir, ok = getConfigDirFromEnv(); ok {
		origin = originEnv
	} else {
		origin = originDefault
		if cfgDir, err = getDefaultConfigDir(); err != nil {
			return nil, nil, err
		}
	}

	cfgPath := filepath.Join(cfgDir, "sq.yml")
	extDir := filepath.Join(cfgDir, "ext")
	cfgStore := &Store{
		Path:            cfgPath,
		PathOrigin:      origin,
		ExtPaths:        []string{extDir},
		UpgradeRegistry: upgrades,
	}

	if !cfgStore.fileExists() {
		cfg := config.New()
		return cfg, cfgStore, nil
	}

	// file does exist, let's try to load it
	cfg, err := cfgStore.Load(ctx, optsReg)
	if err != nil {
		return nil, nil, err
	}

	if _, err = source.VerifyIntegrity(cfg.Collection); err != nil {
		return nil, nil, err
	}

	return cfg, cfgStore, nil
}

// getConfigDirFromFlag parses osArgs looking for flag.ConfigUsage.
// We need to do manual flag parsing because config is loaded before
// cobra is initialized.
func getConfigDirFromFlag(osArgs []string) (dir string, ok bool, err error) {
	fs := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.SetOutput(io.Discard)

	_ = fs.String(flag.Config, "", flag.ConfigUsage)
	if err = fs.Parse(osArgs); err != nil {
		return "", false, errz.Err(err)
	}

	if !fs.Changed(flag.Config) {
		return "", false, nil
	}

	if dir, err = fs.GetString(flag.Config); err != nil {
		return "", false, errz.Err(err)
	}

	if dir == "" {
		return "", false, nil
	}

	return dir, true, nil
}

// getDefaultConfigDir returns "~/.config/sq".
func getDefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		// TODO: we should be able to run without the homedir... revisit this
		return "", errz.Wrap(err, "unable to get user home dir")
	}

	cfgDir := filepath.Join(home, ".config", "sq")
	return cfgDir, nil
}

func getConfigDirFromEnv() (string, bool) {
	if cfgDir, ok := os.LookupEnv(config.EnvarConfig); ok && cfgDir != "" {
		return cfgDir, ok
	}

	// Legacy envar, will eventually remove.
	if cfgDir, ok := os.LookupEnv(config.EnvarConfigDir); ok && cfgDir != "" {
		return cfgDir, ok
	}

	return "", false
}
