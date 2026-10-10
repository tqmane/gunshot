# 非脱獄（Jailed）導入ガイド — Sideloadly / TrollStore / LiveContainer

脱獄していない iOS 端末で Gunshot (GoToHP for iOS) を利用するためのガイドです。  
Google フォトの IPA ファイルに Gunshot を組み込むことで、通常環境でも Pixel 偽装アップロードを利用できます。

---

## 📌 Jailed（非脱獄）版の特徴と動作仕様

- **アプリ内蔵で動作**:
  - 脱獄版のような独立デーモンではなく、Google フォトアプリのプロセス内で動作（Go ランタイム内蔵 dylib）します。
- **フォアグラウンド実行**:
  - iOS のサンドボックス制限により、**Google フォトが画面上で開かれている間（フォアグラウンド）のみアップロードが進行します。**
  - アプリを閉じたり画面をロックするとアップロードは一時停止し、次回アプリを開いたときに自動的に再開されます。
- **ログイン補正機能を内蔵**:
  - サイドロード時に発生しやすい Google ログイン失敗や Keychain エラー（`-34018`）を自動回避する補正機能を搭載しています。別途「Sideload Spoofer」等を導入する必要はありません。

---

## 📦 必要なファイル

[GitHub Releases](https://github.com/tqmane/gunshot/releases) または [GitHub Actions](https://github.com/tqmane/gunshot/actions) の成果物（Artifacts: `gotohp-tweak-jailed`）から以下のファイルを入手します。

| ファイル名 | 用途 |
| :--- | :--- |
| `gotohp-tweak-jailed.deb` | **Sideloadly / TrollStore / 手動注入用**。dylib や設定がまとめられたパッケージ。 |
| `GunshotJailed.dylib` | **LiveContainer 用**。単体ライブラリとして取り込む場合に使用。 |

また、インストールには**復号（Decrypted）済みの Google フォト IPA** が必要です。

---

## 🚀 インストール手順

### 方法 1: Sideloadly を使用する場合 (推奨)

Windows または Mac の Sideloadly を使って IPA に注入・インストールします。

1. **Sideloadly を起動**し、iPhone を PC に接続します。
2. 左上の **IPA アイコン** に復号済み Google フォト IPA をドラッグ＆ドロップします。
3. **Advanced Options** を展開します。
4. **Inject dylibs/frameworks** にチェックを入れ、**「+」または「Add」ボタン**をクリックして `gotohp-tweak-jailed.deb` を選択します。
   *(※ファイル選択ダイアログで `.deb` が見つからない場合は、ファイル形式を「All Files (*.*)」に切り替えてください)*
5. 接続中の端末、Apple ID、および Bundle ID を確認し、**「Start」** をクリックして署名・インストールを実行します。

> [!TIP]
> **アプリを更新する場合**: 前回のインストール時と**同じ Apple ID・同じ Bundle ID** を指定して上書きインストールしてください。既存のログイン情報やキューが引き継がれます。

---

### 方法 2: TrollStore / 各種インジェクターツール

TrollStore 環境や Azule などのツールを使用する場合の手順です。

1. 復号済み Google フォト IPA に、インジェクターツール（Azule や Sideloadly の Export 機能など）を使って `gotohp-tweak-jailed.deb` を注入します。
   - ※手動でバイナリを注入する場合は、`GunshotJailed.dylib` をアプリ内の `Frameworks/` に配置し、メイン実行ファイル（`GooglePhotos`）に対して `@rpath/GunshotJailed.dylib` の `LC_LOAD_DYLIB` を追加してください。
2. 注入後の IPA を TrollStore 等で端末にインストールします。

---

### 方法 3: LiveContainer を使用する場合

JIT 不要でマルチアプリを実行できる LiveContainer で利用する手順です。

1. LiveContainer に復号済みの Google フォト IPA をインストールします。
2. **初回起動する前に**、LiveContainer の **Tweaks** タブを開きます。
3. Google フォト用のフォルダを作成し、その中に `GunshotJailed.dylib` をインポート（**Import Tweak**）します。
4. LiveContainer のアプリ設定から、Google フォトの **Tweak Folder** に作成したフォルダを指定し、**TweakLoader** を有効化します。
5. 必要に応じて署名（Sign）を行い、Google フォトを起動します。

---

## 🔑 初回起動とログインの流れ

> [!IMPORTANT]
> **必ず Gunshot を注入・有効化した状態で、初めて Google アカウントにログインしてください。**

1. インストールした Google フォトを起動します。
2. Google アカウントでログインします。
   - Gunshot が自動的にアプリの認証情報を検知し、GoToHP と連携します。
   - 手動でアクセストークンを調べたり貼り付けたりする必要はありません。
3. 画面右上の **プロフィールアイコン** をタップし、メニュー内に **「GoToHP の設定」** が表示されていることを確認します。
4. 設定画面を開き、お好みの画質プロファイル（Pixel XL オリジナル画質無制限など）を選択してください。

---

## 💡 日常の使い方と注意点

- **アップロードの実行**:
  - 写真・動画のアップロードは、Google フォトが画面上でアクティブになっている間に行われます。
  - 大量にバックアップする際は、端末の自動ロック（スリープ）を一時的に解除するか、画面を開いたまま充電器に接続しておくことをおすすめします。
- **純正バックアップの自動転送**:
  - 「GoToHP の設定」で **「手動・自動バックアップを GoToHP へ送る」** を ON にしておけば、Google フォト本来のバックアップ機能を通じて自動的に Pixel 偽装アップロードされます（詳細は [バックアップ転送ガイド](native-routing.md) を参照）。
- **中断と再開**:
  - アップロード中にアプリを閉じても、未完了のデータはキューに安全に保存されます。次回 Google フォトを開いた際に自動で再開されます。

---

## ❓ トラブルシューティング

- **Google にログインできない / エラー画面が出る**:
  - Gunshot が正しく注入されているか確認してください。GunshotJailed が注入されていれば、サイドロード時の SSO 識別子エラーが自動補正されます。
  - すでに Gunshot なしでログインを試みて失敗していた場合は、一度アプリをアンインストールし、Gunshot 注入済み IPA を再インストールしてからログインをお試しください。
- **「GoToHP の設定」が表示されない**:
  - 注入がメイン実行ファイル（`GooglePhotos`）に行われているか確認してください。App Extension や別バイナリに注入しても動作しません。
- **アップロードが進まない**:
  - Google フォトがフォアグラウンド（画面上）にあるか確認してください。
  - 「GoToHP の設定」を開き、アカウントが正しく接続されているか（緑色のステータスになっているか）確認し、必要に応じて「再接続」をタップしてください。
