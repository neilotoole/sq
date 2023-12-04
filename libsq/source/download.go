package source

import (
	"context"
	"net/url"
	"os"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

// fetch ensures that loc exists locally as a file. This may
// entail downloading the file via HTTPS etc.
func (fs *Files) fetch(ctx context.Context, loc string) (fpath string, err error) {
	// This impl is a vestigial abomination from an early
	// experiment.

	var ok bool
	if fpath, ok = isFpath(loc); ok {
		// loc is already a local file path
		return fpath, nil
	}

	var u *url.URL
	if u, ok = httpURL(loc); !ok {
		return "", errz.Errorf("not a valid file location: %s", loc)
	}

	var dlFile *os.File
	dlFile, err = os.CreateTemp("", "")
	if err != nil {
		return "", errz.Err(err)
	}

	fetchr := &fetcher.Fetcher{}
	// TOOD: ultimately should be passing a real context here
	err = fetchr.Fetch(ctx, u.String(), dlFile)
	if err != nil {
		return "", errz.Err(err)
	}

	// dlFile is kept open until fs is closed.
	fs.clnup.AddC(dlFile)

	return dlFile.Name(), nil
}
