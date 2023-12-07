package source

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"golang.org/x/exp/maps"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

func newDownloader(log *slog.Logger, srcCacheDir, url string) *downloader {
	downloadDir := filepath.Join(srcCacheDir, "download")
	return &downloader{
		log:          log.With(lga.URL, url, "download_dir", downloadDir),
		srcCacheDir:  srcCacheDir,
		downloadDir:  downloadDir,
		checksumFile: filepath.Join(srcCacheDir, "download.checksum.txt"),
		url:          url,
	}
}

type downloader struct {
	log          *slog.Logger
	srcCacheDir  string
	downloadDir  string
	checksumFile string
	url          string
}

func (d *downloader) Cached() (ok bool, sum checksum.Checksum, fp string) {
	fi, err := os.Stat(d.downloadDir)
	if err != nil {
		d.log.Debug("not cached: can't stat download dir")
		return false, "", ""
	}
	if !fi.IsDir() {
		d.log.Error("not cached: download dir is not a dir")
		return false, "", ""
	}

	fi, err = os.Stat(d.checksumFile)
	if err != nil {
		d.log.Debug("not cached: can't stat download checksum file")
		return false, "", ""
	}

	checksums, err := checksum.ReadFile(d.checksumFile)
	if err != nil {
		d.log.Debug("not cached: can't read download checksum file")
		return false, "", ""
	}

	if len(checksums) != 1 {
		d.log.Debug("not cached: download checksum file has unexpected number of entries")
		return false, "", ""
	}

	key := maps.Keys(checksums)[0]
	sum = checksums[key]
	if len(sum) == 0 {
		d.log.Debug("not cached: checksum file has empty checksum", lga.File, key)
		return false, "", ""
	}

	downloadFile := filepath.Join(d.downloadDir, key)

	fi, err = os.Stat(downloadFile)
	if err != nil {
		d.log.Debug("not cached: can't stat download file referenced in checksum file", lga.File, key)
		return false, "", ""
	}

	d.log.Info("found cached file", lga.File, key)
	return true, sum, downloadFile
}

// fetchHTTPHeader fetches the HTTP header for u. First HEAD is used, and
// if that's not allowed (http.StatusMethodNotAllowed), then GET is used.
func fetchHTTPHeader(ctx context.Context, u string) (header http.Header, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return nil, errz.Err(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errz.Err(err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Header, nil
	default:
		return nil, errz.Errorf("unexpected HTTP status (%s) for HEAD: %s", resp.Status, u)
	case http.StatusMethodNotAllowed:
	}

	// HEAD not allowed, try GET
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, errz.Err(err)
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, errz.Err(err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errz.Errorf("unexpected HTTP status (%s) for GET: %s", resp.Status, u)
	}

	return resp.Header, nil
}

func getRemoteChecksum(ctx context.Context, u string) (string, error) {
	return "", errz.New("not implemented")
}

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
