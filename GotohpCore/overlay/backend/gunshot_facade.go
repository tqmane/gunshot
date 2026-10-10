package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var ErrGunshotRemoteComponentExists = errors.New("remote Live Photo component already exists")

type contextTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.ctx.Err(); err != nil {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, err
	}
	// Keep the request/client deadline as well as the upload's cancellation.
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	done := func() { stop(); cancel() }
	response, err := t.base.RoundTrip(r.Clone(ctx))
	if err != nil || response == nil || response.Body == nil {
		done()
	} else {
		// RoundTrip ends at headers, not when the response body has been consumed.
		response.Body = &gunshotContextBody{ReadCloser: response.Body, done: done}
	}
	return response, err
}

type gunshotContextBody struct {
	io.ReadCloser
	done func()
}

func (b *gunshotContextBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.done()
	}
	return n, err
}
func (b *gunshotContextBody) Close() error { b.done(); return b.ReadCloser.Close() }

// Raw + rendered images are independent server media items. We can preserve
// both bytes in one durable job; unlike a verified Live Photo commit, ordinary
// single-media commits do not prove that Google has created a visual stack.
func gunshotRAWPair(paths []string) (cover, raw string, ok bool) {
	if len(paths) != 2 {
		return "", "", false
	}
	for _, path := range paths {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".jpe", ".heic", ".heif":
			if cover != "" {
				return "", "", false
			}
			cover = path
		case ".dng", ".arw", ".cr2", ".cr3", ".nef", ".nrw",
			".orf", ".pef", ".raf", ".raw", ".rw2", ".srw":
			if raw != "" {
				return "", "", false
			}
			raw = path
		default:
			return "", "", false
		}
	}
	return cover, raw, cover != "" && raw != ""
}

type gunshotRAWPairReporter struct {
	UploadReporter
	prefix     int64
	total      int64
	committing bool
}

func (r *gunshotRAWPairReporter) ThreadStatus(status ThreadStatus) {
	if status.BytesTotal > 0 {
		status.BytesUploaded += r.prefix
		status.BytesTotal = r.total
	}
	// After the cover is committed, a failure must not automatically retry
	// the transaction, which would duplicate an already committed image.
	if r.committing && status.Status != "finalizing" {
		status.Status = "finalizing"
	}
	r.UploadReporter.ThreadStatus(status)
}

func gunshotUploadRAWPair(ctx context.Context, api *Api, cover, raw string, opts UploadOptions, reporter UploadReporter) (string, error) {
	coverInfo, err := os.Stat(cover)
	if err != nil {
		return "", err
	}
	rawInfo, err := os.Stat(raw)
	if err != nil {
		return "", err
	}
	if !coverInfo.Mode().IsRegular() || !rawInfo.Mode().IsRegular() {
		return "", errors.New("RAW+rendered pair requires regular source files")
	}
	progress := &gunshotRAWPairReporter{UploadReporter: reporter, total: coverInfo.Size() + rawInfo.Size()}
	// Keep both originals until both independent server commits succeed.
	singleOpts := opts
	singleOpts.DeleteFromHost = false
	coverKey, err := uploadSingleFile(ctx, api, cover, singleOpts, 0, progress)
	if err != nil {
		return "", fmt.Errorf("upload RAW pair cover: %w", err)
	}
	if coverKey == "" {
		return "", errors.New("RAW pair cover media key missing")
	}
	progress.prefix = coverInfo.Size()
	progress.committing = true
	rawKey, err := uploadSingleFile(ctx, api, raw, singleOpts, 0, progress)
	if err != nil {
		return "", fmt.Errorf("upload RAW pair original: %w", err)
	}
	if rawKey == "" {
		return "", errors.New("RAW pair original media key missing")
	}
	// Emit only after both media keys are confirmed. This records both keys
	// without claiming that Google Photos has grouped them into one stack.
	if keys, ok := reporter.(interface{ GunshotRAWPairMediaKeys(string, string) }); ok {
		keys.GunshotRAWPairMediaKeys(coverKey, rawKey)
	}
	if opts.DeleteFromHost {
		if err := os.Remove(raw); err != nil {
			return coverKey, fmt.Errorf("delete committed RAW: %w", err)
		}
		if err := os.Remove(cover); err != nil {
			return coverKey, fmt.Errorf("delete committed cover: %w", err)
		}
	}
	return coverKey, nil
}

// GunshotUpload keeps upstream's ordinary and paired upload paths intact.
func GunshotUpload(ctx context.Context, paths []string, opts UploadOptions, reporter UploadReporter) (string, error) {
	if cover, raw, ok := gunshotRAWPair(paths); ok {
		api, err := NewApi(opts.Api)
		if err != nil {
			return "", err
		}
		api.client.Transport = contextTransport{ctx, api.client.Transport}
		return gunshotUploadRAWPair(ctx, api, cover, raw, opts, reporter)
	}
	items, warnings := ClassifyUploadWork(paths, LivePhotoClassificationOptions{Enabled: len(paths) == 2, SkipIncomplete: true, Cancelled: func() bool { return ctx.Err() != nil }}, nil)
	if len(warnings) > 0 || len(items) != 1 {
		return "", errors.New("media pairing failed")
	}
	if len(paths) == 2 && items[0].Kind != UploadWorkLivePhoto {
		return "", errors.New("not a Live Photo pair")
	}
	api, err := NewApi(opts.Api)
	if err != nil {
		return "", err
	}
	api.client.Transport = contextTransport{ctx, api.client.Transport}
	return gunshotUploadWorkItem(ctx, api, items[0], opts, reporter)
}

func gunshotUploadWorkItem(ctx context.Context, api *Api, item UploadWorkItem, opts UploadOptions, reporter UploadReporter) (string, error) {
	// A hash match proves that a component exists, not that the Live Photo is
	// complete. Let upstream attach the MOV and require its commit result.
	opts.UpdateExistingPhotosToLive = true
	key, skipped, err := uploadWorkItem(ctx, api, item, opts, 0, reporter)
	if skipped && err == nil && item.Kind == UploadWorkLivePhoto {
		// With updates enabled, pinned upstream skips only a remote MOV without
		// its still. Create the still first, then use the verified still-first
		// reconciliation path; do not guess the private VideoOriginal request.
		stillOpts := opts
		stillOpts.ForceUpload = false
		stillOpts.DeleteFromHost = false // Both files are needed until linked.
		if _, err := uploadSingleFile(ctx, api, item.LivePhoto.PhotoPath, stillOpts, 0, reporter); err != nil {
			return "", err
		}
		key, skipped, err = uploadWorkItem(ctx, api, item, opts, 0, reporter)
	}
	if skipped && err == nil {
		return "", ErrGunshotRemoteComponentExists
	}
	return key, err
}
