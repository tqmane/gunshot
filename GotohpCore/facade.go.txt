package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
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

// GunshotUpload keeps upstream's ordinary and paired upload paths intact.
func GunshotUpload(ctx context.Context, paths []string, opts UploadOptions, reporter UploadReporter) (string, error) {
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
