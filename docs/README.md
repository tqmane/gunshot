# ドキュメント

利用方法は [日本語 README](../README.ja.md) / [English README](../README.md) から始めてください。以下は目的別の参照先です。

## 現在の手順・仕様

| 目的 | 参照先 | 内容 |
| --- | --- | --- |
| インストール | [jailed / Sideloadly / LiveContainer](jailed.md) | 注入・署名・更新時の注意。jailbreak の導入は README |
| 通常のバックアップ操作を転送 | [バックアップ連携](native-routing.md) | 有効化、対応経路、診断、実行制約 |
| 大量の写真を取り込む | [一括取込](bulk-import.md) | アルバム、PhotoKit 原本、停止・再試行 |
| 開発・テスト・リリース | [開発ガイド](development.md) | 環境、共通コマンド、書式、upstream 更新 |
| コードの役割と制約 | [アーキテクチャ](architecture.md) | ソース配置、認証、IPC、永続キュー |
| 翻訳を変更する | [ローカライズ](localization.md) | カタログと生成ヘッダーの更新 |
| 端末で確認する | [実機検証](device-validation.md) | CI だけでは確認できない項目 |

## 解析資料

[解析索引](analysis/index.md) は、IPA の静的解析・過去の診断・修正の根拠を探すための資料です。日付付きの観測結果や古い実装の説明を、現在の導入手順として使わないでください。

[3版の互換性比較](analysis/compatibility.md) は今回提供された 7.20.2 / 7.92.0 / 7.96.0 の限定的な metadata 確認です。API の存在確認、fixture の成功、実機・Google サーバーでの成功は区別します。

文書を更新するときは、手順・仕様を上の担当ページに反映し、README には入口を置きます。新しい不具合の経緯を README に追記したり、同じ設定の説明を複数ページで管理したりしないでください。
