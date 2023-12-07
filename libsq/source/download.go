package source

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"golang.org/x/exp/maps"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

func newDownloader(srcCacheDir, url string) *downloader {
	downloadDir := filepath.Join(srcCacheDir, "download")
	return &downloader{
		srcCacheDir:  srcCacheDir,
		downloadDir:  downloadDir,
		checksumFile: filepath.Join(srcCacheDir, "download.checksum.txt"),
		url:          url,
	}
}

type downloader struct {
	mu           sync.Mutex
	srcCacheDir  string
	downloadDir  string
	checksumFile string
	url          string
}

func (d *downloader) log(log *slog.Logger) *slog.Logger {
	return log.With(lga.URL, d.url, "download_dir", d.downloadDir)
}

// Download downloads the file at the URL to the download dir, and also writes
// the file to dest, and returns the file path of the downloaded file.
// It is the caller's responsibility to close dest. If an appropriate file name
// cannot be determined from the HTTP response, the file is named "download".
// If the download fails at any stage, the download file is removed, but written
// always returns the number of bytes written to dest.
// Note that the download process is context-aware.
func (d *downloader) Download(ctx context.Context, dest io.Writer) (written int64, fp string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log := d.log(lg.FromContext(ctx))

	if err = ioz.RequireDir(d.downloadDir); err != nil {
		return written, "", errz.Wrapf(err, "could not create download dir for: %s", d.url)
	}

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return written, "", errz.Wrapf(err, "download new request failed for: %s", d.url)
	}

	// FIXME: Use a client that doesn't require SSL (see fetcher)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return written, "", errz.Wrapf(err, "download failed for: %s", d.url)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return written, "", errz.Errorf("download failed with %s for %s", resp.Status, d.url)
	}

	filename := getDownloadFilename(resp)
	if filename == "" {
		filename = "download"
	}

	fp = filepath.Join(d.downloadDir, filename)
	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return written, "", errz.Wrapf(err, "could not create download file for: %s", d.url)
	}

	written, err = io.Copy(
		contextio.NewWriter(ctx, io.MultiWriter(f, dest)),
		contextio.NewReader(ctx, resp.Body),
	)
	if err != nil {
		log.Error("failed to write download file", lga.File, fp, lga.URL, d.url, lga.Err, err)
		lg.WarnIfCloseError(log, lgm.CloseFileWriter, f)
		lg.WarnIfFuncError(log, lgm.RemoveFile, func() error { return errz.Err(os.Remove(fp)) })
		return written, "", err
	}

	if err = f.Close(); err != nil {
		lg.WarnIfFuncError(log, lgm.RemoveFile, func() error { return errz.Err(os.Remove(fp)) })
		return written, "", errz.Wrapf(err, "failed to close download file: %s", fp)
	}

	log.Info("Wrote download file", lga.Written, written, lga.File, fp)
	return written, fp, nil
}
func (d *downloader) Cached(ctx context.Context) (ok bool, sum checksum.Checksum, fp string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log := d.log(lg.FromContext(ctx))

	fi, err := os.Stat(d.downloadDir)
	if err != nil {
		log.Debug("not cached: can't stat download dir")
		return false, "", ""
	}
	if !fi.IsDir() {
		log.Error("not cached: download dir is not a dir")
		return false, "", ""
	}

	fi, err = os.Stat(d.checksumFile)
	if err != nil {
		log.Debug("not cached: can't stat download checksum file")
		return false, "", ""
	}

	checksums, err := checksum.ReadFile(d.checksumFile)
	if err != nil {
		log.Debug("not cached: can't read download checksum file")
		return false, "", ""
	}

	if len(checksums) != 1 {
		log.Debug("not cached: download checksum file has unexpected number of entries")
		return false, "", ""
	}

	key := maps.Keys(checksums)[0]
	sum = checksums[key]
	if len(sum) == 0 {
		log.Debug("not cached: checksum file has empty checksum", lga.File, key)
		return false, "", ""
	}

	downloadFile := filepath.Join(d.downloadDir, key)

	fi, err = os.Stat(downloadFile)
	if err != nil {
		log.Debug("not cached: can't stat download file referenced in checksum file", lga.File, key)
		return false, "", ""
	}

	log.Info("found cached file", lga.File, key)
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
	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()
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

// getDownloadFilename returns the filename to use for a download.
// It first checks the Content-Disposition header, and if that's
// not present, it uses the last path segment of the URL.
// It's possible that the returned value will be empty string; the
// caller should handle that situation themselves.
func getDownloadFilename(resp *http.Response) string {
	var filename string
	dispHeader := resp.Header.Get("Content-Disposition")
	if dispHeader != "" {
		if _, params, err := mime.ParseMediaType(dispHeader); err == nil {
			filename = params["filename"]
		}
	}
	if filename == "" {
		filename = path.Base(resp.Request.URL.Path)
	}

	return filename
}
