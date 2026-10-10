# アーキテクチャ・設計解説 (Architecture)

Gunshot (GoToHP for iOS) のシステム構成、各コンポーネントの責務、データフロー、および設計思想の解説です。

---

## 🏛️ システム概要

Gunshot は、iOS の Google フォトアプリ（Objective-C / Swift）と、Google Pixel 偽装アップロードを行う Go コアエンジンを橋渡しするアーキテクチャを採用しています。

動作モードによって 2 種類の実行形態を持ちます：

```
【脱獄環境 (Jailbreak)】
[Google フォト (Tweak.xm / Native / UI)]
       │ (Mach Message / XPC)
       ▼
[バックグラウンドデーモン gotohpd (Daemon/)]
       │ (C/Go ブリッジ)
       ▼
[Go コアエンジン (internal/service/)] ──(HTTPS)──▶ Google フォト サーバー

【非脱獄環境 (Jailed / Sideload)】
[Google フォト (Jailed/Hooks.m / Native / UI)]
       │ (Direct In-Process Call)
       ▼
[組み込みサービス (Jailed/EmbeddedService.m)]
       │ (C/Go ブリッジ)
       ▼
[Go コアエンジン (internal/service/)] ──(HTTPS)──▶ Google フォト サーバー
```

---

## 📁 ディレクトリ構造とコンポーネントの責務

| ディレクトリ | 主な役割 |
| :--- | :--- |
| `Tweak.xm` | 脱獄版のエントリーポイント。Logos によるフック定義とライフサイクル管理。 |
| `Jailed/` | 非脱獄（サイドロード）版のエントリーポイント（`Hooks.m`）およびアプリ内組み込みサービスブリッジ（`EmbeddedService.m`）。 |
| `UI/` | 設定パネル（`GSPanel`）、アルバムピッカー（`GSAlbumPicker`）、アカウントメニュー統合（`GSAccountMenu`）、外観オプション。 |
| `Native/` | Google フォト内部機能との連携。アカウント検出・SSOトークン取得（`GSNativeAccount`）、標準バックアップ転送（`GSNativeRouting`）、完了監視・UI同期（`GSPhotosIntegration`）。 |
| `Media/` | 写真・動画の取り出し。PhotoKit からのオリジナルバイナリ無劣化エクスポート（`GSExporter`）、アルバム一括処理（`GSBatchImport`）。 |
| `Shared/` | 共通基盤。プロセス間通信プロトコル（`IPCProtocol`）、Mach/XPC通信（`GSMachTransport`）、多言語ローカライズ（`GSLocalization`）、ランタイムチェック。 |
| `Daemon/` | 脱獄用デーモン（`gotohpd`）。Mach サービスの提供、クライアント認証（audit token 検証）、電源・通信状況の監視。 |
| `internal/service/` | Go 言語によるアップロードキューエンジン。ジョブ永続化（`state.go`）、キュー管理（`queue.go`）、ライフサイクル復旧（`engine.go`）。 |
| `GotohpCore/` | [xob0t/gotohp](https://github.com/xob0t/gotohp) のコアコード（upstream）と、iOS 向けに適用するオーバーレイ差分。 |

---

## 🔄 データフローとアップロード処理

### 1. メディアの抽出 (Media Export)
- ユーザーが写真を選択、または Google フォトのバックアップが開始されると、`Media/GSExporter` が PhotoKit (`PHAssetResourceManager`) を通じてオリジナルファイルを一時ディレクトリに書き出します。
- **Live Photo の場合**: 静止画（HEIC/JPEG）とペア動画（MOV）の両方のリソースを同時に取り出します。再エンコードやメタデータの欠落は発生しません。

### 2. ステージングとキュー登録 (Staging & Queueing)
- 一時ファイルは 32 KiB 単位で Go コアの専用ステージング領域にコピーされ、サイズ整合性とハッシュ値（SHA-1 等）の検証が行われます。
- ステージングが完了すると、キューの永続状態（`state.json`）がアトミックにディスクへ書き込まれ、アップロード待ちジョブとして確定します。

### 3. アップロード実行 (Upload Execution)
- キューワーカーがジョブを取り出し、現在の Google アカウントの認証情報と Pixel 端末プロファイル（Pixel XL 等）を用いて Google フォトサーバーと通信します。
- サーバー上のハッシュ照合（重複チェック）、アップロードトークンの取得、バイナリ転送、およびコミット処理を行います。
- **Live Photo の場合**: 静止画と動画の 2 つのリソースを適切なメタデータ（StillImageTime 等）とともに関連付け、Google フォト上で 1 つの再生可能な Live Photo としてコミットします。

### 4. 完了後のステータス同期 (Reconciliation)
- アップロードが完了すると、`Native/GSPhotosIntegration` を通じて Google フォト内部の同期処理（`fetchData` 等）が呼び出され、UI 上で対象の写真が「バックアップ完了」として反映されます。

---

## 🔑 認証とトークンの管理

- **シームレスなトークン取得**:
  - Google フォトの内部 SSO サービス（`PHSAccountManagerImpl`）を利用し、ログイン中のアカウントから OAuth Bearer トークンを直接取得します。
  - ユーザーが手動でトークンをコピーしたり、外部ツールからインポートする必要はありません。
- **メモリ内保持と自動更新**:
  - トークンはメモリ内でのみ保持され、ディスク上のログや設定ファイルには一切平文保存されません。
  - 脱獄版ではデーモン内でのトークン保持期限を最大 5 分とし、期限切れの際は Google フォトの復帰時に自動的に再取得が行われます。
- **サイドロード時のログイン補正**:
  - Jailed 版では、Bundle ID の変更等によって Google SSO の認証がブロックされる現象を防ぐため、リクエスト時の識別子や Keychain アクセスグループを自動的に正規の識別子へ補正します。

---

## 🛡️ 耐障害性と永続化 (Durability)

- **アトミックな状態保存**:
  - ジョブの状態変更（Pending / Uploading / Completed / Failed）は、常に `fsync` とファイルのアトミック置換によってディスクに安全に保存されます。
- **クラッシュ・強制終了からの復旧**:
  - アプリの強制終了や端末の再起動が発生した場合、次回起動時に前回の状態が検査され、中断されたジョブは安全に `Pending`（再試行待ち）へ戻されて自動再開されます。
- **重複アップロードの抑止**:
  - アップロード前にサーバー側でのハッシュ照合を行い、すでにライブラリに存在するファイルは即座にコミット処理へ移行します。
