package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadResult describes a staged release asset ready to be applied.
type DownloadResult struct {
	Path      string
	SHA256    string
	AssetName string
}

// Downloader streams release assets into a staging directory with SHA-256
// verification. A download either ends as a verified file in the staging
// directory or leaves nothing behind.
type Downloader struct {
	updateDir string
	hc        *http.Client

	// freeBytes reports free space on the volume holding path. It defaults to
	// the platform implementation and is a test seam.
	freeBytes func(path string) (uint64, error)
}

// NewDownloader returns a downloader staging into updateDir.
func NewDownloader(updateDir string) *Downloader {
	return &Downloader{
		updateDir: updateDir,
		hc:        &http.Client{Timeout: 5 * time.Minute},
		freeBytes: defaultFreeBytes,
	}
}

// Download fetches asset from its browser download URL into the staging
// directory. When the asset carries a digest it is verified before the file
// is staged; a mismatch is a hard failure and leaves nothing behind.
func (d *Downloader) Download(ctx context.Context, asset Asset, progress func(done, total int64)) (DownloadResult, error) {
	result := DownloadResult{AssetName: asset.Name}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "wassap-desktop")

	resp, err := d.hc.Do(req)
	if err != nil {
		return result, fmt.Errorf("updater: download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return result, fmt.Errorf("updater: download %s: unexpected status %d", asset.Name, resp.StatusCode)
	}

	expected := resp.ContentLength
	if expected <= 0 {
		expected = 100 << 20 // no length advertised: assume 100 MB for the disk check
	}
	if err := d.checkDiskSpace(d.updateDir, uint64(expected)); err != nil {
		return result, err
	}

	if err := os.MkdirAll(d.updateDir, 0o700); err != nil {
		return result, err
	}

	partPath := filepath.Join(d.updateDir, asset.Name+".part")
	finalPath := filepath.Join(d.updateDir, asset.Name)
	defer func() { _ = os.Remove(partPath) }() // no-op once renamed

	f, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return result, err
	}

	if err := d.stream(resp.Body, f, expected, progress); err != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return result, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return result, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(partPath)
		return result, err
	}

	sum, err := sha256File(partPath)
	if err != nil {
		_ = os.Remove(partPath)
		return result, err
	}
	result.SHA256 = sum

	if asset.Digest != "" {
		if err := VerifyFileSHA256(partPath, asset.Digest); err != nil {
			_ = os.Remove(partPath)
			return result, err
		}
	} else {
		log.Printf("updater: asset %s carries no digest; staging unverified (HTTPS transport only)", asset.Name)
	}

	if err := os.Rename(partPath, finalPath); err != nil {
		_ = os.Remove(partPath)
		return result, err
	}
	result.Path = finalPath
	return result, nil
}

// stream copies body to w at most ~32 KiB per read, invoking progress no
// more than every 250ms.
func (d *Downloader) stream(body io.Reader, w io.Writer, total int64, progress func(done, total int64)) error {
	buf := make([]byte, 32*1024)
	var done int64
	lastEmit := time.Time{}
	emit := func(force bool) {
		if progress == nil {
			return
		}
		if !force && time.Since(lastEmit) < 250*time.Millisecond {
			return
		}
		lastEmit = time.Now()
		progress(done, total)
	}

	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			wn, werr := w.Write(buf[:n])
			if werr != nil {
				return werr
			}
			if wn != n {
				return io.ErrShortWrite
			}
			done += int64(n)
			emit(false)
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	emit(true)
	return nil
}

func (d *Downloader) checkDiskSpace(path string, need uint64) error {
	if d.freeBytes == nil {
		return nil
	}
	free, err := d.freeBytes(path)
	if err != nil {
		log.Printf("updater: cannot read free disk space at %s: %v (proceeding)", path, err)
		return nil
	}
	if free < 2*need {
		return fmt.Errorf("updater: not enough free disk space: need at least %d bytes, have %d", 2*need, free)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFileSHA256 compares the file at path against want ("sha256:<hex>" or
// bare hex). A mismatch or malformed input is an error.
func VerifyFileSHA256(path, want string) error {
	want = strings.TrimPrefix(want, "sha256:")
	if _, err := hex.DecodeString(want); err != nil {
		return fmt.Errorf("updater: malformed sha256 digest %q: %w", want, err)
	}
	got, err := sha256File(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("updater: digest mismatch for %s: got sha256:%s, want sha256:%s", filepath.Base(path), got, want)
	}
	return nil
}
