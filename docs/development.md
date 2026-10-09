# Development

[Documentation index](README.md) · [Architecture and source map](architecture.md)

## Checkout and requirements

```sh
git clone --recurse-submodules https://github.com/tqmane/gunshot.git
cd gunshot
```

Core checks run on Linux or macOS with **Go 1.26.0**, Python 3 and a C compiler. Native fixtures require macOS/Xcode. Device packages additionally require Theos, `ldid` and `dpkg`. CI pins Theos in [setup-theos.sh](../scripts/setup-theos.sh).

## Checks

Run commands from the repository root. CI uses these same scripts.

| Command | Coverage |
| --- | --- |
| `bash scripts/test-core.sh` | Localization, upstream projection, Go race tests, projected backend tests, vet and the real C/Go bridge |
| `bash scripts/test-native.sh` | All macOS native fixtures; optional `jailbreak` / `jailed` argument selects a suite |
| `python3 scripts/format.py --check` | First-party native, Go and Python formatting |
| `python3 scripts/verify-package.py jailed` | Built package layout; also accepts `rootless` / `rootful` |

Native fixtures cover modern/legacy APIs, unrelated version strings, incompatible ABIs, account changes, backup handoff, reconciliation, diagnostics, PhotoKit import and IPC. They simulate private API contracts; they do not log into Google or verify cloud playback/quota. Use the [device checklist](device-validation.md) for that.

## Formatting and source ownership

Install the development formatters (`python3 -m pip install clang-format==18.1.8 black==25.1.0`), then run `python3 scripts/format.py` before committing. The script uses `.clang-format`, `gofmt` and Black; it excludes generated localization and the upstream submodule. `Tweak.xm` uses Logos syntax and is maintained manually. Python uses four-space indentation; shell scripts use Bash with `set -euo pipefail`.

Keep UI presentation in `UI/`, host integration in `Native/`, and original export/import in `Media/`. Add shared host sources to [sources.mk](../sources.mk), which both packages consume. Preserve private-method ABI checks, account checks, callback ordering and inherited-method scoping. Reuse `Shared/GSPhotosRuntime.h` for checked object getters; do not replace checked calls with unchecked `performSelector:` calls.

## Upstream projection

`GotohpCore/upstream` is an immutable submodule pinned by [UPSTREAM_REVISION](../GotohpCore/UPSTREAM_REVISION). Build preparation copies its backend/generated code into `.build/upstream`, removes desktop-only pieces and applies the small iOS adaptations in [prepare-core.py](../scripts/prepare-core.py).

The maintained adaptations and their tests live together in `GotohpCore/overlay/backend/` as ordinary `.go` files. `configmanager.go.tmpl` is the only template: preparation inserts upstream's Preferences definitions and setters. The overlay's `go.mod` defines the projected module and prevents the root `go test ./...` from compiling these incomplete inputs in isolation. **Test the assembled projection with `go test -tags cli app/backend`**, or simply use `test-core.sh`.

Do not edit `.build/upstream`; it is replaced on each preparation. `replace_once` guards patch anchors so upstream drift cannot silently drop TLS/auth adaptations. To update the pin:

```sh
bash scripts/sync-upstream.sh <commit>
```

Review the upstream diff, projection changes and device behavior before committing both the submodule pointer and revision file. The projection tests include Live Photo component reconciliation, quality serialization, context cancellation and native authentication. Keep upstream's license and distribution notices.

## Build and release

```sh
export THEOS="$HOME/theos"
bash scripts/package.sh jailed  # or rootless / rootful
```

Jailed output is in `packages/jailed/`; jailbreak output is in `packages/`. Build dependencies are not required to install prebuilt packages. [Jailed installation](jailed.md) explains injection and signing.

The [workflow](../.github/workflows/build.yml) builds all three package schemes. A successful `v*` tag build publishes assets. For an existing tag, use **Actions → Build and test → Run workflow**, choose the updated workflow branch, and enter `release_tag`. It builds that tag's source without moving the tag, replaces identically named assets, and preserves an existing release's title and notes. Empty `release_tag` builds only. Re-running an old job uses its old workflow.
