package backend

import (
	"app/generated"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

type gunshotRAWTestReporter struct {
	NopReporter
	keys     []string
	statuses []ThreadStatus
}

func (r *gunshotRAWTestReporter) GunshotRAWPairMediaKeys(cover, raw string) {
	r.keys = []string{cover, raw}
}
func (r *gunshotRAWTestReporter) ThreadStatus(s ThreadStatus) {
	r.statuses = append(r.statuses, s)
}

func TestGunshotRAWPairClassification(t *testing.T) {
	for _, tc := range []struct {
		files   []string
		cover   string
		raw     string
		matched bool
	}{
		{[]string{"a.DNG", "a.JPG"}, "a.JPG", "a.DNG", true},
		{[]string{"a.jpeg", "a.cr3"}, "a.jpeg", "a.cr3", true},
		{[]string{"a.HEIC", "a.NEF"}, "a.HEIC", "a.NEF", true},
		{[]string{"a.jpg"}, "", "", false},
		{[]string{"a.jpg", "a.mov"}, "", "", false},
		{[]string{"a.jpg", "b.jpg"}, "", "", false},
		{[]string{"a.dng", "b.dng"}, "", "", false},
		{[]string{"a.bin", "b.bin"}, "", "", false},
	} {
		cover, raw, ok := gunshotRAWPair(tc.files)
		if cover != tc.cover || raw != tc.raw || ok != tc.matched {
			t.Fatalf("classify %v: cover=%q raw=%q ok=%t", tc.files, cover, raw, ok)
		}
	}
}

func TestGunshotRAWPairTwoConfirmedCommits(t *testing.T) {
	GunshotSetNativeBearerProvider(func(string) (string, error) { return "fixture-token", nil })
	t.Cleanup(func() { GunshotSetNativeBearerProvider(nil) })
	for _, tc := range []struct {
		name     string
		rawFirst bool
		fail     string
		saver    bool
		quota    bool
	}{
		{"original", false, "", false, false},
		{"raw-first", true, "", false, false},
		{"saver", false, "", true, false},
		{"quota", false, "", false, true},
		{"raw commit fails", false, "raw", false, false},
		{"cover commit fails", false, "cover", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cover := filepath.Join(dir, "IMG_42.JPG")
			raw := filepath.Join(dir, "IMG_42.DNG")
			contents := map[string][]byte{
				cover: []byte("original jpeg payload"),
				raw:   []byte("original raw payload"),
			}
			hashes := make(map[string][]byte)
			for path, data := range contents {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				stamp := time.Unix(1700000000, 0)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				hash := sha1.Sum(data)
				hashes[path] = hash[:]
			}
			api, err := newAPIFromCredential("Email=test%40example.com&gunshot_native_id=fixture", "")
			if err != nil {
				t.Fatal(err)
			}
			api.saver, api.useQuota = tc.saver, tc.quota
			commits := []string{}
			uploaded := []string{}
			response := func(message proto.Message) (*http.Response, error) {
				data, err := proto.Marshal(message)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}, nil
			}
			api.client.Transport = gunshotRoundTripper(func(r *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/data/upload/uploadmedia/interactive":
					uploadID := strings.TrimPrefix(r.Header.Get("X-Goog-Hash"), "sha1=")
					resp, err := response(&generated.CommitToken{})
					resp.Header.Set("X-GUploader-UploadID", uploadID)
					return resp, err
				case r.Method == http.MethodPut:
					sha := sha1.Sum(body)
					uploadID := base64.StdEncoding.EncodeToString(sha[:])
					if r.URL.Query().Get("upload_id") != uploadID {
						t.Fatal("wrong raw/cover component sent")
					}
					match := false
					for _, bytes := range contents {
						if bytes != nil && string(bytes) == string(body) {
							match = true
							break
						}
					}
					if !match {
						t.Fatal("changed original payload bytes")
					}
					uploaded = append(uploaded, uploadID)
					return response(&generated.CommitToken{Field1: 2, Field2: sha[:]})
				case strings.HasSuffix(r.URL.Path, "/16538846908252377752"):
					var commit generated.CommitUpload
					if err := proto.Unmarshal(body, &commit); err != nil {
						t.Fatal(err)
					}
					name := commit.GetField1().GetFileName()
					commits = append(commits, name)
					var filename string
					if strings.HasSuffix(strings.ToLower(name), ".dng") {
						filename = raw
					} else {
						filename = cover
					}
					if !bytes.Equal(commit.GetField1().GetSha1Hash(), hashes[filename]) {
						t.Fatal("wrong source fingerprint in commit")
					}
					var policy int64 = 3
					if tc.saver {
						policy = 1
					}
					if commit.GetField1().GetQuality() != policy || commit.GetField1().GetField10() != 1 ||
						commit.GetField1().GetField4().GetFileLastModifiedTimestamp() != 1700000000 {
						t.Fatal("original bytes, time or quality policy lost")
					}
					if tc.fail == "raw" && filename == raw || tc.fail == "cover" && filename == cover {
						return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("not committed"))}, nil
					}
					key := "cover-key"
					if filename == raw {
						key = "raw-key"
					}
					return response(&generated.CreateMediaItemsResponse{Item: []*generated.CreateMediaItemResponseItem{{ResultItem: &generated.CreateMediaItemResult{MediaKey: key}}}})
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
					return nil, errors.New("unexpected request")
				}
			})
			paths := []string{cover, raw}
			if tc.rawFirst {
				paths = []string{raw, cover}
			}
			first, second, ok := gunshotRAWPair(paths)
			if !ok {
				t.Fatal("real RAW pair not classified")
			}
			reporter := &gunshotRAWTestReporter{}
			opts := UploadOptions{ForceUpload: true, DeleteFromHost: true}
			key, err := gunshotUploadRAWPair(context.Background(), api, first, second, opts, reporter)
			if tc.fail == "" {
				if err != nil || key != "cover-key" || len(commits) != 2 || len(uploaded) != 2 ||
					len(reporter.keys) != 2 || reporter.keys[0] != "cover-key" || reporter.keys[1] != "raw-key" {
					t.Fatalf("unconfirmed two-component pair: key=%q err=%v commits=%v keys=%v", key, err, commits, reporter.keys)
				}
				for _, path := range paths {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("both committed components must be released")
					}
				}
			} else {
				if err == nil || key != "" || len(reporter.keys) != 0 {
					t.Fatalf("partial pair reported complete: key=%q err=%v keys=%v", key, err, reporter.keys)
				}
				for _, path := range paths {
					if _, err := os.Stat(path); err != nil {
						t.Fatal("removed a source before both commits succeeded")
					}
				}
				if tc.fail == "raw" {
					if len(commits) != 2 || len(uploaded) != 2 {
						t.Fatal("RAW failure did not exercise committed cover")
					}
					var seenCommit bool
					for _, s := range reporter.statuses {
						if s.Status == "finalizing" {
							seenCommit = true
						} else if seenCommit && s.Status != "finalizing" {
							t.Fatal("partial commit lost durable committing phase")
						}
					}
				} else if len(commits) != 1 {
					t.Fatal("uploaded RAW after cover commit was rejected")
				}
			}
		})
	}
}
