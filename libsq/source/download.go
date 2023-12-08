package source

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/exp/maps"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/fetcher"
)

var OptHTTPPingTimeout = options.NewDuration(
	"http.ping.timeout",
	"",
	0,
	time.Second*10,
	"HTTP ping timeout duration",
	`How long to wait for initial response from HTTP endpoint before
timeout occurs. Long-running operations, such as HTTP file downloads, are
not affected by this option. Example: 500ms or 3s.`,
	options.TagSource,
)

var OptHTTPSkipVerify = options.NewBool(
	"http.skip-verify",
	"",
	false,
	0,
	false,
	"Skip HTTPS TLS verification",
	"Skip HTTPS TLS verification. Useful when downloading against self-signed certs.",
)

func newDownloader(c *http.Client, cacheDir, url string) *downloader {
	return &downloader{
		c:        c,
		cacheDir: cacheDir,
		url:      url,
	}
}

// download is a helper for getting file contents from a URL,
// and caching the file locally. The structure of cacheDir
// is as follows:
//
//	cacheDir/
//	  pid.lock
//	  checksum.txt
//	  header.txt
//	  dl/
//	    <filename>
//
// Let's take a closer look.
//
//   - pid.lock is a lock file used to ensure that only one
//     process is downloading the file at a time.
//
//   - header.txt is a dump of the HTTP response header, included for
//     debugging convenience.
//
//   - checksum.txt contains a checksum:key pair, where the checksum is
//     calculated using checksum.ForHTTPHeader, and the key is the path
//     to the downloaded file, e.g. "dl/data.csv".
//
//     67a47a0...a53e3e28154  dl/actor.csv
//
//   - The file is downloaded to dl/<filename> instead of into the root
//     of cache dir, just to avoid the (remote) possibility of a name
//     collision with the other files in cacheDir. The filename is based
//     on the HTTP response, incorporating the Content-Disposition header
//     if present, or the last path segment of the URL. The filename is
//     sanitized.
//
// When downloader.Download is invoked, it appropriately clears the existing
// stored files before proceeding. Likewise, if the download fails, the stored
// files are wiped, to prevent a partial download from being used.
type downloader struct {
	c        *http.Client
	mu       sync.Mutex
	cacheDir string
	url      string
}

func (d *downloader) log(log *slog.Logger) *slog.Logger {
	return log.With(lga.URL, d.url, lga.Dir, d.cacheDir)
}

func (d *downloader) dlDir() string {
	return filepath.Join(d.cacheDir, "dl")
}

func (d *downloader) checksumFile() string {
	return filepath.Join(d.cacheDir, "checksum.txt")
}

func (d *downloader) headerFile() string {
	return filepath.Join(d.cacheDir, "header.txt")
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

	dlDir := d.dlDir()
	// Clear the download dir.
	if err = os.RemoveAll(dlDir); err != nil {
		return written, "", errz.Wrapf(err, "could not clear download dir for: %s", d.url)
	}

	if err = ioz.RequireDir(dlDir); err != nil {
		return written, "", errz.Wrapf(err, "could not create download dir for: %s", d.url)
	}

	// Make sure the header file is cleared.
	if err = os.RemoveAll(d.headerFile()); err != nil {
		return written, "", errz.Wrapf(err, "could not clear header file for: %s", d.url)
	}

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return written, "", errz.Wrapf(err, "download new request failed for: %s", d.url)
	}

	resp, err := d.c.Do(req)
	if err != nil {
		return written, "", errz.Wrapf(err, "download failed for: %s", d.url)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
		}
	}()

	if err = d.writeHeaderFile(resp); err != nil {
		return written, "", err
	}

	if resp.StatusCode != http.StatusOK {
		return written, "", errz.Errorf("download failed with %s for %s", resp.Status, d.url)
	}

	filename := getDownloadFilename(resp)
	if filename == "" {
		filename = "download"
	}

	fp = filepath.Join(dlDir, filename)
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

	sum := checksum.ForHTTPResponse(resp)
	if err = checksum.WriteFile(d.checksumFile(), sum, filepath.Join("dl", filename)); err != nil {
		lg.WarnIfFuncError(log, lgm.RemoveFile, func() error { return errz.Err(os.Remove(fp)) })
	}

	log.Info("Wrote download file", lga.Written, written, lga.File, fp)
	return written, fp, nil
}

func (d *downloader) writeHeaderFile(resp *http.Response) error {
	b, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return errz.Wrapf(err, "failed to dump HTTP response for: %s", d.url)
	}

	if len(b) == 0 {
		return errz.Errorf("empty HTTP response for: %s", d.url)
	}

	// Add a custom field just for human consumption convenience.
	b = bytes.TrimSuffix(b, []byte("\r\n"))
	b = append(b, "X-Sq-Downloaded-From: "+d.url+"\r\n"...)

	if err = os.WriteFile(d.headerFile(), b, os.ModePerm); err != nil {
		return errz.Wrapf(err, "failed to store HTTP response header for: %s", d.url)
	}
	return nil
}

func (d *downloader) Cached(ctx context.Context) (ok bool, sum checksum.Checksum, fp string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log := d.log(lg.FromContext(ctx))
	dlDir := d.dlDir()
	fi, err := os.Stat(dlDir)
	if err != nil {
		log.Debug("not cached: can't stat download dir")
		return false, "", ""
	}
	if !fi.IsDir() {
		log.Error("not cached: download dir is not a dir")
		return false, "", ""
	}

	if _, err = os.Stat(d.checksumFile()); err != nil {
		log.Debug("not cached: can't stat download checksum file")
		return false, "", ""
	}

	checksums, err := checksum.ReadFile(d.checksumFile())
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

	downloadFile := filepath.Join(dlDir, key)

	if _, err = os.Stat(downloadFile); err != nil {
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
	default:
		return nil, errz.Errorf("unexpected HTTP status (%s) for HEAD: %s", resp.Status, u)
	case http.StatusOK:
		return resp.Header, nil
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
// not present, it uses the last path segment of the URL. The
// filename is sanitized.
// It's possible that the returned value will be empty string; the
// caller should handle that situation themselves.
func getDownloadFilename(resp *http.Response) string {
	var filename string
	if resp == nil || resp.Header == nil {
		return ""
	}
	dispHeader := resp.Header.Get("Content-Disposition")
	if dispHeader != "" {
		if _, params, err := mime.ParseMediaType(dispHeader); err == nil {
			filename = params["filename"]
		}
	}

	if filename == "" {
		filename = path.Base(resp.Request.URL.Path)
	} else {
		filename = filepath.Base(filename)
	}

	return stringz.SanitizeFilename(filename)
}
