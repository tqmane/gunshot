# Google フォト各バージョン間の互換性資料 (7.20.2 / 7.92.0 / 7.96.0)

[ドキュメントポータル](../README.md) · [内部解析インデックス](index.md)

Gunshot は、Google フォトのバージョン番号をハードコードして固定するのではなく、**実行時に必要なクラス・セレクタ・型シグネチャの存在を動的に検証して適切なフックを選択する設計**になっています。

本書では、検証用として確認された 3 つの代表的バージョン（7.20.2 / 7.92.0 / 7.96.0）におけるバイナリ解析結果および主要内部 API の互換性状況をまとめています。

---

## 📊 主要 API / メタデータ比較

| 項目 / クラス・セレクタ | 7.20.2 (旧世代) | 7.92.0 | 7.96.0 (最新) | 備考 |
| :--- | :--- | :--- | :--- | :--- |
| **主要フレームワーク** | `ModuleFramework` | `GooglePhotos_GeneratedFramework` | `GooglePhotos_GeneratedFramework` | 構成フレームワーク名の移行 |
| **Mach-O 最小ビルド OS** | iOS 16.1 | iOS 18.0 | iOS 18.0 | ビルド時のベース SDK |
| `PHSAccountManagerImpl`<br>`photosSSOService` | 未検出 | `@16@0:8` | `@16@0:8` | 7.92+ の SSO サービス取得 |
| `PHSAccountManagerImpl`<br>`ssoService` | `@16@0:8` | `@16@0:8` | `@16@0:8` | 全版共通のフォールバック取得 |
| `GMUAssetUploadRequest`<br>完了コールバック | `didCompleteWithSuccess:...errorCode:`<br>(`v36@0:8B16@20q28`) | `didCompleteWithSuccess:...error:`<br>(`v36@0:8B16@20@28`) | `didCompleteWithSuccess:...error:`<br>(`v36@0:8B16@20@28`) | 完了引数の型（整数コード → NSError オブジェクト） |
| `GMULivePhotoSingleUploadRequest`<br>`didCompleteWithError:resultantMediaItem:` | `v32@0:8@16@24` | `v32@0:8@16@24` | `v32@0:8@16@24` | Live Photo 完了通知（全版互換） |
| `PHSOneUpInfoPanelDetailsViewController`<br>`getBackupStatusModelData` | 未検出 | `@16@0:8` | `@16@0:8` | 詳細画面のバックアップ状態モデル取得 |
| `PHSOneUpInfoPanelDetailsViewController`<br>`modelForBackedupStatus` | `@16@0:8` | `@16@0:8` | `@16@0:8` | 全版共通のバックアップモデル取得 |
| `PHSUserItemsSynchronizer`<br>`fetchData` | `v16@0:8` | `v16@0:8` | `v16@0:8` | アップロード後のステータス再取得（全版互換） |

> **解析のポイント**:
> - **7.92.0 と 7.96.0**: Gunshot が利用する主要なフック対象（アカウント SSO、アップロード要求、Live Photo、ステータス同期）のセレクタおよび型シグネチャは**完全に一致**しており、高い互換性が維持されています。
> - **7.20.2**: 古いフレームワーク構造（`ModuleFramework`）や完了コールバックの引数型（`errorCode:`）の違いがありますが、Gunshot 内部で両世代のシグネチャを自動判定して切り替える実装になっています。

---

## 🔍 メタデータの再抽出手順

新しい IPA や手持ちの IPA から Objective-C メタデータを抽出し、シグネチャを検証する手順です。

1. IPA ファイルを展開（unzip）します。
2. 付属の抽出スクリプトを実行して、メインバイナリとフレームワークからメタデータを JSON として書き出します：

```sh
# メインバイナリの抽出
python3 scripts/extract-objc-metadata.py /path/to/GooglePhotos.app/GooglePhotos /tmp/main-methods.json

# フレームワークバイナリの抽出
python3 scripts/extract-objc-metadata.py /path/to/GooglePhotos.app/Frameworks/GooglePhotos_GeneratedFramework.framework/GooglePhotos_GeneratedFramework /tmp/framework-methods.json
```

3. 出力された JSON と `docs/analysis/objc/compatibility-contracts.json` を照合し、必要なセレクタや引数シグネチャが一致しているか確認します。
