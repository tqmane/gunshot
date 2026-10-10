package backend

import (
	"app/generated"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

type gunshotLivePhotoReporter struct {
	NopReporter
	last string
}

func (r *gunshotLivePhotoReporter) ThreadStatus(s ThreadStatus) { r.last = s.Status }

// Exercise the real upstream uploader and serialized requests without a Google
// account. Hash hits alone must never complete a pair or delete its local files.
func TestGunshotLivePhotoExistingComponents(t *testing.T) {
	GunshotSetNativeBearerProvider(func(string) (string, error) { return "fixture-token", nil })
	t.Cleanup(func() { GunshotSetNativeBearerProvider(nil) })
	for _, quality := range []struct {
		name, model  string
		saver, quota bool
		policy       int64
	}{
		{"original", "Pixel XL", false, false, 3},
		{"saver", "Pixel 2", true, false, 1},
		{"quota", "Pixel 8", false, true, 3},
	} {
		for _, tc := range []struct {
			name, photoKey, videoKey, fail       string
			uploads, stillCommits, linkedCommits int
		}{
			{"new", "", "", "", 2, 0, 1},
			{"still exists", "still-key", "", "", 1, 0, 1},
			{"video exists", "", "video-key", "", 2, 1, 1},
			{"separate components", "still-key", "video-key", "", 1, 0, 1},
			{"same remote item", "linked-key", "linked-key", "", 1, 0, 1},
			{"still lookup fails", "", "video-key", "photo lookup", 0, 0, 0},
			{"video lookup fails", "still-key", "", "video lookup", 0, 0, 0},
			{"still commit unknown", "", "video-key", "still commit", 1, 1, 0},
			{"link commit unknown", "still-key", "", "linked commit", 1, 0, 1},
			{"link fails after still commit", "", "video-key", "linked commit", 2, 1, 1},
			{"still not yet searchable", "", "video-key", "visibility", 1, 1, 0},
		} {
			t.Run(quality.name+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				pair := LivePhotoPair{PhotoPath: filepath.Join(dir, "image.HEIC"), VideoPath: filepath.Join(dir, "image.MOV")}
				contents := [][]byte{[]byte("still fixture"), []byte("video fixture")}
				paths := []string{pair.PhotoPath, pair.VideoPath}
				hashes := make([][]byte, 2)
				for i, path := range paths {
					if err := os.WriteFile(path, contents[i], 0600); err != nil {
						t.Fatal(err)
					}
					stamp := time.Unix(1700000000, 0)
					if err := os.Chtimes(path, stamp, stamp); err != nil {
						t.Fatal(err)
					}
					hash := sha1.Sum(contents[i])
					hashes[i] = hash[:]
				}
				api, err := newAPIFromCredential("Email=test%40example.com&gunshot_native_id=fixture", "")
				if err != nil {
					t.Fatal(err)
				}
				api.saver, api.useQuota = quality.saver, quality.quota
				photoKey, videoKey := tc.photoKey, tc.videoKey
				uploads, stillCommits, linkedCommits := 0, 0, 0
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
					case strings.HasSuffix(r.URL.Path, "/5084965799730810217"):
						var check generated.HashCheck
						if err := proto.Unmarshal(body, &check); err != nil {
							t.Fatal(err)
						}
						hash := check.GetField1().GetField1().GetSha1Hash()
						key := videoKey
						isPhoto := bytes.Equal(hash, hashes[0])
						if isPhoto {
							key = photoKey
						} else if !bytes.Equal(hash, hashes[1]) {
							t.Fatal("unexpected hash")
						}
						if (isPhoto && tc.fail == "photo lookup") || (!isPhoto && tc.fail == "video lookup") {
							return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("lookup failed"))}, nil
						}
						return response(&generated.RemoteMatches{Field1: &generated.RemoteMatchesField1Type{
							Field2: &generated.RemoteMatchesField1TypeField2Type{Field2: &generated.RemoteMatchesField1TypeField2TypeField2Type{MediaKey: key}},
						}})
					case r.Method == http.MethodPost && r.URL.Path == "/data/upload/uploadmedia/interactive":
						resp, err := response(&generated.CommitToken{})
						resp.Header.Set("X-GUploader-UploadID", strings.TrimPrefix(r.Header.Get("X-Goog-Hash"), "sha1="))
						return resp, err
					case r.Method == http.MethodPut:
						uploads++
						hash := sha1.Sum(body)
						if base64.StdEncoding.EncodeToString(hash[:]) != strings.TrimPrefix(r.URL.RawQuery, "upload_id=") {
							t.Fatal("wrong component uploaded")
						}
						return response(&generated.CommitToken{Field1: 2, Field2: hash[:]})
					case strings.HasSuffix(r.URL.Path, "/16538846908252377752"):
						var request generated.CreateMediaItemsRequest
						if err := proto.Unmarshal(body, &request); err != nil {
							t.Fatal(err)
						}
						if len(request.BlueprintArray) != 1 {
							t.Fatal("pair must produce one blueprint")
						}
						blueprint := request.BlueprintArray[0]
						if blueprint.StoragePolicy != quality.policy || blueprint.UploadQuality != 1 || request.GetUploadDeviceInfo().GetModel() != quality.model {
							t.Fatal("storage policy changed during Live Photo recovery")
						}
						isLinked := blueprint.LivePhotoInfo != nil || blueprint.ReconcileInfo != nil
						if !isLinked {
							stillCommits++
							if !bytes.Equal(blueprint.SourceSha1, hashes[0]) || blueprint.FileName != "image.HEIC" {
								t.Fatal("wrong still staged")
							}
							if tc.fail != "visibility" {
								photoKey = "staged-still-key"
							}
						} else {
							linkedCommits++
							if blueprint.ReconcileInfo != nil {
								if photoKey == "" || blueprint.LivePhotoInfo != nil || blueprint.ReconcileInfo.ReconcileType != generated.ReconcileType_RECONCILE_TYPE_PHODEO || blueprint.ReconcileInfo.PhotoUploadBlueprint != nil || !bytes.Equal(blueprint.ReconcileInfo.SourceSha1, hashes[0]) || !bytes.Equal(blueprint.SourceSha1, hashes[1]) || blueprint.FileName != "image.MOV" {
									t.Fatal("invalid still-first reconciliation")
								}
							} else if !bytes.Equal(blueprint.SourceSha1, hashes[0]) || !bytes.Equal(blueprint.LivePhotoInfo.VideoSourceSha1, hashes[1]) || len(blueprint.LivePhotoInfo.VideoUploadToken) == 0 {
								t.Fatal("new pair lost the linked video")
							}
							if blueprint.GetFilesystemCreateTime().GetSeconds() != 1700000000 {
								t.Fatal("capture date lost")
							}
						}
						if (isLinked && tc.fail == "linked commit") || (!isLinked && tc.fail == "still commit") {
							return response(&generated.CreateMediaItemsResponse{}) // Accepted but unconfirmed: never claim completion.
						}
						key := "staged-still-key"
						if isLinked {
							key = "linked-key"
						}
						return response(&generated.CreateMediaItemsResponse{Item: []*generated.CreateMediaItemResponseItem{{ResultItem: &generated.CreateMediaItemResult{MediaKey: key}}}})
					default:
						t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
						return nil, nil
					}
				})
				reporter := &gunshotLivePhotoReporter{}
				key, err := gunshotUploadWorkItem(context.Background(), api, UploadWorkItem{Kind: UploadWorkLivePhoto, LivePhoto: &pair}, UploadOptions{ForceUpload: quality.name == "original", DeleteFromHost: true}, reporter)
				if tc.fail == "" {
					if err != nil || key != "linked-key" {
						t.Fatalf("upload: key=%q err=%v", key, err)
					}
				} else {
					if err == nil || key != "" {
						t.Fatalf("incomplete pair reported complete: key=%q err=%v", key, err)
					}
					if strings.Contains(tc.fail, "commit") && reporter.last != "finalizing" {
						t.Fatal("ambiguous commit lost its phase")
					}
				}
				if uploads != tc.uploads || stillCommits != tc.stillCommits || linkedCommits != tc.linkedCommits {
					t.Fatalf("uploads/still/link commits = %d/%d/%d, want %d/%d/%d", uploads, stillCommits, linkedCommits, tc.uploads, tc.stillCommits, tc.linkedCommits)
				}
				for _, path := range paths {
					_, err := os.Stat(path)
					if tc.fail == "" && !os.IsNotExist(err) {
						t.Fatal("confirmed pair was not cleaned up")
					}
					if tc.fail != "" && err != nil {
						t.Fatal("local component deleted before linked commit")
					}
				}
			})
		}
	}
}
