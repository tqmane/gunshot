# Architecture

[Documentation index](README.md) · [Build and test](development.md)

## Source map

| Location | Responsibility |
| --- | --- |
| `Tweak.xm` / `Jailed/Hooks.m` | Jailbreak / in-process entry points and host lifecycle setup |
| `UI/` | Settings, album picker, share activity, account-menu presentation and optional glass appearance |
| `Native/` | Google Photos SSO, backup routing, completion monitoring, display integration and passive diagnostics |
| `Media/` | Serial PhotoKit original export and bounded batch import |
| `Shared/` | IPC contract/transport, sandbox discovery, localization and ABI-checked runtime helpers |
| `Daemon/` | Jailbreak process authorization, Mach service and network/power conditions |
| `Jailed/EmbeddedService.m` | In-process bridge, private storage and foreground/network conditions |
| `internal/service/` | Go queue and request handling, shared by both runtime modes |
| `GotohpCore/overlay/` | iOS additions and regression tests applied to the pinned upstream |

`internal/service/types.go` defines wire/persisted models; `engine.go` owns lifecycle and restart recovery; `state.go` owns durable writes/validation; `import.go` owns staging; `queue.go` schedules uploads. `protocol.go` authorizes operations, `upstream.go` adapts accounts/upload execution, and `summary.go` exposes redacted progress. These files remain one package and share the same engine lock.

[sources.mk](../sources.mk) is the shared host source list. Jailbreak links the Mach client and daemon; jailed links the embedded adapter and omits the daemon bearer relay. Neither uses a separate Preferences bundle.

## Upload flow

PhotoKit exports original bytes, including both Live Photo components, into a temporary host directory. Import reserves a daemon/service-created random job ID and explicit filename/size pairs. Offset-checked 32 KiB chunks populate private staging files; sealing checks sizes and fsyncs before publishing pending work. Hashes accumulate during import, avoiding a second large-file read under the queue lock.

The queue snapshots account and quality. Each upload gets an independent upstream API client and cancellation context. Ordinary files use upstream classification, hash lookup, upload-token acquisition, byte transfer and commit. Live Photos use Apple content identifiers / QuickTime identifiers and still-image-time metadata; arbitrary same-stem files are not silently paired. The facade accepts exactly one work item; two input files must form a Live Photo.

A remote hash match for one component does not prove a complete Live Photo. The facade lets upstream update an existing still with its video; a video-only match first creates the still and then follows that reconciliation path. Completion requires a commit result. A persistent unlinked match fails explicitly. Regression coverage lives in `GotohpCore/overlay/backend/gunshot_live_photo_upload_test.go`.

Native backup routing keeps Google's scheduler/delegates, sends originals through the same queue, then resumes native server/fingerprint lookup. It blocks native payload fallback during reconciliation and does not fabricate successful callbacks or write native backup flags. Direct GoToHP uploads also trigger account-bound `fetchData` through the foreground completion monitor. See [supported routes](native-routing.md).

## Durability and recovery

State is fsynced and atomically replaced before scheduling. Write failure stops scheduling until restart; corrupt state fails initialization rather than resetting history. Phase changes are durable; per-byte progress stays in memory to reduce flash writes.

On restart, an incomplete import is cancelled, an active preparation/upload returns to pending, and an uncertain commit becomes failed for manual review. Transfer retries start at byte zero. Hash checks reduce duplicates but cannot provide an exactly-once transaction with Google. Failed jobs retain staging until cancellation; cancellation never deletes remote media. Existing job IDs, JSON keys, state values and quality policies are shared across both runtimes.

## Transport and storage boundary

Jailbreak clients try direct Mach lookup, restricted libSandy access and authenticated XPC discovery. Only Apple Photos and Google Photos with expected signing identifiers and executable locations are accepted. The daemon derives identity from a kernel audit token bound to SecTask; JSON cannot supply its own role. Missing Security SPI fails closed. `Shared/GSXPC.h` supplies SDK-missing C ABI declarations, not another XPC implementation.

Messages have a fixed maximum size and validated lengths; port/OOL descriptors and filesystem path operations are not accepted. Only Google Photos may mutate accounts/options or submit native authorization. Approved Photos clients can import media. The internal settings role belongs to the jailed adapter; the daemon never grants it. Conditions are daemon-only.

Jailbreak data belongs to the mobile daemon. Theos package prefixes affect executables, LaunchDaemons and the libSandy profile, not user-data paths. Rootless and rootful must not be installed together. Jailed storage stays in the host's `Application Support/GoToHP`; it opens no external IPC. See [jailed storage constraints](jailed.md#実行保存の制約).

## Authentication

Both entry points start account connection on launch and recheck on foreground activation; no settings page is required. The native SSO authorizer supplies `photos.native` authorization. Jailed uses the provider directly; jailbreak relays it through audit-authorized `account_native` / `native_bearer` requests. A new binding is validated at the Photos endpoint before persisting email/native ID. Bearers remain in memory and are absent from queue state, account summaries and diagnostics.

Host renewal is serialized with reconnect, runs every 60 seconds while scheduled and rechecks on foreground activation. The daemon retains each bearer for at most five minutes; that is a local cap, not Google's expiry guarantee. Missing authorization pauses new work without consuming retries. Closing/suspending Google Photos prevents indefinite renewal; reopening replenishes it. A different identity's bearer is never used for an existing queue binding.

Legacy credential import still validates required fields and Google authentication before persistence. Its URL query string can contain Android ID, email, token, signature, scope, language and binding material. Previously imported credentials remain until removed; summaries never expose those secrets.

## Upstream and compatibility

The pin is recorded in [UPSTREAM_REVISION](../GotohpCore/UPSTREAM_REVISION). [Projection preparation](development.md#upstream-projection) replaces desktop configuration with an iOS JSON store and excludes Wails, ADB/executable discovery and desktop config-migration tests. Upstream Preferences/setters remain for backend test compatibility; two YAML persistence assertions are translated to JSON.

Explicit adaptations initialize a nil TLS configuration, keep certificate validation enabled (including proxies), set a six-hour request ceiling, fail closed on ordinary duplicate-check errors, and propagate upload cancellation. The submodule is never rewritten by a build.

Private features select legacy/modern APIs independently using exact class/selector/ABI checks. Version metadata is informational. [Compatibility evidence](analysis/compatibility.md) and [historical analysis](analysis/index.md) are distinct from [device validation](device-validation.md); metadata and CI cannot prove real authentication, quota, Live Photo playback or host UI behavior.
