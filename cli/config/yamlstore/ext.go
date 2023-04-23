package yamlstore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
)

// loadExt loads extension config files into cfg.
func (fs *YAMLFileStore) loadExt(cfg *config.Config) error {
	const extSuffix = ".sq.yml"
	var extCfgCandidates []string

	for _, extPath := range fs.ExtPaths {
		// TODO: This seems overly complicated: could just use glob
		//  for any files in the same or child dir?
		if fiExtPath, err := os.Stat(extPath); err == nil {
			// path exists

			if fiExtPath.IsDir() {
				files, err := os.ReadDir(extPath)
				if err != nil {
					// just continue; no means of logging this yet (logging may
					// not have bootstrapped), and we shouldn't stop bootstrap
					// because of bad sqext files.
					continue
				}

				for _, file := range files {
					if file.IsDir() {
						// We don't currently descend through sub dirs
						continue
					}

					if !strings.HasSuffix(file.Name(), extSuffix) {
						continue
					}

					extCfgCandidates = append(extCfgCandidates, filepath.Join(extPath, file.Name()))
				}

				continue
			}

			// it's a file
			if !strings.HasSuffix(fiExtPath.Name(), extSuffix) {
				continue
			}
			extCfgCandidates = append(extCfgCandidates, filepath.Join(extPath, fiExtPath.Name()))
		}
	}

	for _, fp := range extCfgCandidates {
		bytes, err := os.ReadFile(fp)
		if err != nil {
			return errz.Wrapf(err, "error reading config ext file: %s", fp)
		}
		ext := &config.Ext{}

		err = ioz.UnmarshallYAML(bytes, ext)
		if err != nil {
			return errz.Wrapf(err, "error parsing config ext file: %s", fp)
		}

		cfg.Ext.UserDrivers = append(cfg.Ext.UserDrivers, ext.UserDrivers...)
	}

	return nil
}
