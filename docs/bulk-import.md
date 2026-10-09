# Bulk import and troubleshooting

[Documentation index](README.md) · [Native backup routing](native-routing.md)

## Choose an album

Open **GoToHP settings → Uploads → Choose album**. Browse smart/user albums or open a folder to choose a child album, then confirm the accessible item count. Folders are navigation containers, not recursive upload selections. Limited photo permission may hide albums or exclude items.

This path uses a PhotoKit fetch result instead of loading thousands of PHPicker selections. **Choose photos and videos** accepts up to 100 items per selection; that is GoToHP's limit, not an Apple-documented failure threshold. If the system picker shows **Unable to Load Items**, cancel it and use **Choose album**.

## Preparation and restart

Only one preparation batch runs in the host at a time. A shared serial exporter bounds PhotoKit/iCloud requests from both manual imports and native backup. Identifier lookup runs off the main thread in pages of 64 and matches by identifier, not result order. HEIC/HEIF stays unconverted; Live Photos retain the original still and paired video. Upload concurrency is configured separately.

Keep the host open while preparing. **Stop preparing** stops after the current item and keeps queued jobs. An expired background task also stops preparation. Items not yet in the durable queue do not survive process termination. Reselect after reopening; account/quality/content deduplication reuses existing queue entries.

Individual unreadable originals count as failed while the rest continue. Account changes, unavailable service, storage failure or rejected queue requests stop preparation. Check photo access and iCloud availability before retrying. A commit with an unknown outcome needs cloud-side inspection first; retries may otherwise duplicate media.

## Diagnostics

Export diagnostics before restarting the host when investigating preparation failures.

| Field | Meaning |
| --- | --- |
| `batchImport` | Latest in-memory preparation: source, stage, counts and fixed failure/stop codes; resets on process exit |
| `completionMonitor.uploadSummary.mediaTypes` | Persisted job counts by HEIC/HEIF, HEIC Live Photo, other Live Photo, JPEG, PNG, video or other |
| `completionMonitor.uploadSummary` | Queue outcome/failure codes; completed does not establish quota treatment |
| `photosIntegration` | Native synchronization requests/waits; cloud completion and grid refresh are separate |

Diagnostics exclude filenames, asset/account identifiers, tokens and raw upstream errors. Aggregate failure counts alone cannot identify a codec or server failure. The original large-selection report is [issue #21](https://github.com/tqmane/gunshot/issues/21); it motivated the album path.

Fixtures exercise 2,000 identifiers, reordered/missing results, export failure, cancellation and account changes. Export/import fixtures compare original bytes and timestamps across 60 HEIC/HEIF resources, including a Live Photo. These prove orchestration and byte preservation, not image decoding or Google's servers. Use the [device checklist](device-validation.md) for real albums and cloud-only media.
