# GoToHP for iOS — Gunshot

[English](README.md) · [日本語](README.ja.md)

[xob0t/gotohp](https://github.com/xob0t/gotohp) の Go コアを使う Google Photos アップローダーです。jailbreak・サイドロード・LiveContainer に対応し、jailbreak では独立 daemon、jailed では Google Photos 内で動作します。

**開発版です。** 各機能がクラス・selector・正確な型を確認して API を選び、版番号では制限しません。従来の静的解析基準は 7.20.2 / 7.92.0 で、提供された 7.96.0 との限定的な比較も[互換性資料](docs/analysis/compatibility.md)に記載しています。静的解析と実機での動作確認は別です。

## スクリーンショット

<p>
  <img src="docs/images/unlimited-storage.png" width="240" alt="Google Photos 純正の無制限ストレージ表示">
  <img src="docs/images/profile-menu.png" width="240" alt="プロフィールメニュー内の GoToHP の設定">
</p>
<p>
  <img src="docs/images/upload-settings.png" width="240" alt="ログイン中アカウントと Pixel 1 のオリジナル画質設定">
  <img src="docs/images/backup-routing.png" width="240" alt="手動・自動バックアップの転送とキュー管理の設定">
  <img src="docs/images/appearance-settings.png" width="240" alt="表示言語と無制限ストレージ表示の切り替え">
</p>

## 導入

| 環境 | 配布物 | 実行場所 |
| --- | --- | --- |
| jailbreak・rootless | `gotohp-tweak-rootless.deb` | アプリで取り込み、daemon で送信 |
| jailbreak・rootful | `gotohp-tweak-rootful.deb` | アプリで取り込み、daemon で送信 |
| サイドロード / LiveContainer | `gotohp-tweak-jailed.deb` / `GunshotJailed.dylib` | Google Photos 内。前面で開いておく |

[Releases](https://github.com/tqmane/gunshot/releases) または [Actions](https://github.com/tqmane/gunshot/actions) から取得します。Sideloadly・手動注入・LiveContainer は[導入ガイド](docs/jailed.md)を参照してください。jailbreak 用パッケージの IPA 注入や、jailed deb と dylib の二重注入は避けてください。

**Google ログイン前に Gunshot を導入・有効化してください。** jailed には対応 SSO の識別子・Keychain 補正が含まれ、その補正のために別途 Sideload Spoofer を入れる必要はありません。更新時は署名アカウント・Bundle ID・アプリデータ、LiveContainer では同じ guest / データコンテナを維持します。ログイン成功やセッション維持を保証するものではありません。

jailbreak では substrate 互換の注入環境と、[opa334 のリポジトリ](https://opa334.github.io/)の **libSandy 1.1.6 以降**が必要です。依存を解決できるパッケージマネージャーで導入してください。クラッシュする場合は Choicy で Google Photos に Gunshot だけを有効化します。rootless / rootful の同時導入は避けてください。iOS「設定」への項目追加や PreferenceLoader は不要です。

## 使い方

1. Google Photos を起動してログインします。表示中のアカウントへ自動接続し、トークン入力や設定を開く操作は不要です。
2. **プロフィールメニュー → GoToHP の設定**で画質、キュー、再接続を操作します。
3. **アップロード → 写真・動画を選択**は1回100件まで。大量の取込には **アルバムを選択**を使います。[準備・停止・トラブル対処](docs/bulk-import.md)。
4. 必要なら **手動・自動バックアップを GoToHP へ送る**を有効化し、送信先を確認します。既定は OFF。自動バックアップには Google Photos 本体のバックアップも ON にします。[対応経路・診断](docs/native-routing.md)。

jailbreak では Apple Photos のボタンと対応する **Upload with GoToHP** 共有操作も使えます。原本がキューに入るまでアプリを開いてください。その後は認証が有効な間 daemon が送信を続けます。bearer 保持は最長5分で、更新には Google Photos が必要です。jailed はホスト停止中に送信を継続せず、前景・認証・通信条件が整うと再開します。

| 画質 | 要求する端末プロファイル・動作 |
| --- | --- |
| オリジナル | Pixel XL（Pixel 1）、原本画質・ストレージ使用なし |
| 保存容量の節約 | Pixel 2、保存容量を節約 |
| アカウントの保存容量 | Pixel 8、通常の容量を使う原本画質 |

これらは要求値であり、Google 側の容量計上を保証しません。アカウントと画質はキュー追加時に固定されます。PhotoKit は再エンコードせず原本を取り出し、Live Photo は写真と動画の両方を扱います。再試行・取消・再起動復旧に対応しますが、再送は先頭からです。commit 結果不明の場合はクラウド側を確認してから再試行してください。取消はクラウドの写真を削除しません。

**表示 → 言語**で日本語・英語を選べます。純正の **無制限ストレージ**カード表示は外観設定であり、実際の容量計上とは別です。

## 開発・資料

ビルド・テスト・upstream 更新は[開発ガイド](docs/development.md)、コードの担当箇所は[アーキテクチャ](docs/architecture.md)を参照してください。[ドキュメント索引](docs/README.md)から現在の手順と過去の解析記録を探せます。

## 免責事項

Google / Apple 非公式のプロジェクトです。**無保証・現状のまま**提供します。私有 API やアプリ更新により、動作不良・アカウント制限・データ損失・容量課金が発生する可能性があります。原本は別途バックアップしてください。Google Photos のバイナリ・証明書・認証情報は配布しません。

## ライセンス

Gunshot: [GNU GPL v3.0 or later](LICENSE)、Copyright (C) 2026 tqmane。同梱の [gotohp upstream](GotohpCore/upstream/LICENSE) は MIT、Copyright (c) 2024 xob0t。その他の依存も各ライセンスに従い、配布物に `ThirdPartyNotices.txt` を同梱します。
