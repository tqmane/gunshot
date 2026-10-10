# Google フォト内部解析・技術資料インデックス

Google フォトアプリのリバースエンジニアリング結果、内部クラス構造、イベント処理フロー、および互換性調査のレポート一覧です。  
Gunshot のフック実装や過去の不具合修正の根拠資料として参照できます。

---

## 📑 解析レポート一覧

| ドキュメント | 対象・内容 |
| :--- | :--- |
| **[バージョン互換性比較 (7.20.2 / 7.92.0 / 7.96.0)](compatibility.md)** | 提供された各バージョンの IPA から抽出した主要クラス・セレクタ・型の比較 |
| **[7.20.2 互換性監査レポート](google-photos-7.20.2.md)** | レガシーバージョン（7.20.2）と近年のバージョン間における API / ABI の詳細差分 |
| **[Objective-C メタデータ索引](objc/README.md)** | メイン実行ファイルおよびフレームワークから抽出した全クラス・メソッドの検索方法 |
| **[バックアップ転送ルーティングの解析](backup-routing.md)** | `GMUAssetUploadRequest` 等のバックアップ要求のフック手法と同期処理の詳細 |
| **[アカウント連携と認証フロー](native-account.md)** | Google SSO（`PHSAccountManagerImpl`）からのトークン取得経路と Keychain 補正 |
| **[アカウントメニュー統合とタップ処理](account-menu-tap.md)** | プロフィールメニューへのカスタム項目追加とタップ時のモーダル遷移の解析・修正 |
| **[無制限ストレージ表示の解析](unlimited-storage.md)** | プロフィール画面における「無制限ストレージ」カード表示の内部フック手法 |
| **[オリジナル画質表示とステータス同期](original-quality-display.md)** | 写真詳細パネルにおけるバックアップステータス表示（`modelForBackedupStatus` 等）の解析 |
| **[アップロード完了処理の内部依存](completion-analysis.md)** | アップロード完了コールバックの引数型（errorCode / NSError）の世代差の調査 |
| **[ボトムバー外観の解析](bottom-bar-glass.md)** | Liquid Glass 等の外観カスタマイズに関する内部ビュー構造の調査 |

---

## 🔍 解析対象と調査の前提

- 本解析資料は、主に Google フォトのメインバイナリ（`GooglePhotos`）および主要フレームワーク（`ModuleFramework` / `GooglePhotos_GeneratedFramework`）の Objective-C メタデータをベースに作成されています。
- 各バージョンで利用している内部 API の具体的な実装状況については、[バージョン互換性比較](compatibility.md) をご参照ください。
