# GoToHP for iOS — Gunshot

[English](README.md) · [日本語](README.ja.md)

A Google Photos uploader for jailbreak, sideloading and LiveContainer, using the Go core from [xob0t/gotohp](https://github.com/xob0t/gotohp). Jailbreak packages use a separate daemon; jailed packages run inside Google Photos.

**Development build.** Private APIs are selected per feature by class, selector and exact signature, without a version allowlist. The established static reference versions are 7.20.2 and 7.92.0; a scoped comparison with the supplied 7.96.0 IPA is recorded in [compatibility evidence](docs/analysis/compatibility.md). Static inspection is not device verification.

## Screenshots

<p>
  <img src="docs/images/unlimited-storage.png" width="240" alt="Google Photos profile menu showing the native Unlimited storage card">
  <img src="docs/images/profile-menu.png" width="240" alt="GoToHP settings entry in the Google Photos profile menu">
</p>
<p>
  <img src="docs/images/upload-settings.png" width="240" alt="Signed-in account and original-quality Pixel 1 upload settings">
  <img src="docs/images/backup-routing.png" width="240" alt="Manual and automatic backup routing and queue management settings">
  <img src="docs/images/appearance-settings.png" width="240" alt="Language and Show unlimited storage settings">
</p>

## Install

| Environment | Package | Execution |
| --- | --- | --- |
| Jailbreak, rootless | `gotohp-tweak-rootless.deb` | Imports in the host, uploads through the daemon |
| Jailbreak, rootful | `gotohp-tweak-rootful.deb` | Imports in the host, uploads through the daemon |
| Sideload / LiveContainer | `gotohp-tweak-jailed.deb` / `GunshotJailed.dylib` | Runs inside Google Photos; keep it in the foreground |

Get packages from [Releases](https://github.com/tqmane/gunshot/releases) or [Actions](https://github.com/tqmane/gunshot/actions). The [jailed installation guide](docs/jailed.md) covers Sideloadly, manual injection and LiveContainer. Do not inject jailbreak packages or inject both the jailed deb and dylib.

**Install and enable Gunshot before Google sign-in.** Jailed includes compatible SSO identifier/Keychain adaptations; a separate Sideload Spoofer is not needed for those corrections. Preserve the signing account, bundle ID and app data when updating, or the same LiveContainer guest/data container. Sign-in/session retention is not guaranteed.

Jailbreak requires a substrate-compatible injection system and **libSandy 1.1.6+** from [opa334's repository](https://opa334.github.io/). Install through a package manager to resolve dependencies. If Google Photos crashes, use Choicy to enable only Gunshot for it. Rootless and rootful must not be installed together. There is no iOS Settings entry or PreferenceLoader dependency.

## Use

1. Launch Google Photos and sign in. GoToHP connects the viewed account automatically; no token paste or settings visit is needed.
2. Open **Profile menu → GoToHP settings** to choose quality, check the queue or reconnect.
3. Use **Uploads → Choose photos and videos** (up to 100 per selection), or **Choose album** for bulk imports. See [preparation, stopping and troubleshooting](docs/bulk-import.md).
4. Optionally enable **Route manual and automatic backups through GoToHP**, confirm the destination, and use supported native backup actions. It defaults off; automatic backup also requires Google Photos backup to be on. See [routing coverage and diagnostics](docs/native-routing.md).

Jailbreak also provides an Apple Photos button and a supported **Upload with GoToHP** share action. Keep the host open until originals enter the queue. The daemon can then continue while authorization is valid; its bearer retention is capped at five minutes, and renewed authorization requires Google Photos. Jailed uploads pause when the host stops running and resume when foreground/auth/network conditions permit.

| Quality | Requested device profile / behavior |
| --- | --- |
| Original | Pixel XL (Pixel 1), original quality without storage usage |
| Storage saver | Pixel 2, storage saver |
| Account storage | Pixel 8, original quality using normal quota |

These are requests, not guarantees of quota treatment. Account and quality are fixed per queued job. PhotoKit exports original resources, including both Live Photo components, without re-encoding. Queue retry/cancel/recovery is supported; transfers restart from byte zero. An uncertain commit needs manual cloud-side review before retrying. Cancellation never deletes remote photos.

English and Japanese are available under **Appearance → Language**. The optional native **Unlimited storage** card is a display setting; it does not prove account quota behavior.

## Development

Start with the [development guide](docs/development.md) for build/test commands and upstream updates, and the [source map](docs/architecture.md) to locate a feature. The [documentation index](docs/README.md) separates current guides from historical analysis. Installation/routing guides and the index are in Japanese; this README provides the English quick start.

## Disclaimer

Unofficial and unaffiliated with Google or Apple, provided **as is, without warranty**. Private APIs and app updates may break functionality or lead to account restrictions, data loss or storage charges. Keep a separate backup of your originals. Google Photos binaries, certificates and credentials are not distributed here.

## License

Gunshot: [GNU GPL v3.0 or later](LICENSE), Copyright (C) 2026 tqmane. The bundled [gotohp upstream](GotohpCore/upstream/LICENSE) remains MIT-licensed, Copyright (c) 2024 xob0t. Other components retain their licenses; distributions include `ThirdPartyNotices.txt`.
