# Collision - Matchmaking Server

Collisionは、リアルタイムマッチメイキング機能を提供するgRPCサーバーです。

## 機能

- **マッチメイキング**: プレイヤーのチケットを作成し、自動的に対戦相手とマッチング
- **リアルタイム通知**: WatchAssignments APIでマッチング結果をリアルタイムに取得
- **1vs1マッチング**: 2人のプレイヤーが揃った時点でマッチを作成

## 必要な環境

- Go 1.21+
- Redis (マッチメイキングデータの保存用)

## セットアップ

### 1. Redisの起動

```bash
# Dockerを使用する場合
docker run -d -p 6379:6379 redis:alpine

# または、ローカルにインストールされたRedisを使用
redis-server
```

### 2. プロジェクトのビルド

```bash
# Protocol Bufferファイルの生成
task proto

# サーバーとクライアントのビルド
go build -o bin/collision ./cmd/collision
go build -o bin/simpleticket ./cmd/simpleticket
```

## 使用方法

### サーバーの起動

ターミナル1でマッチメイキングサーバーを起動:

```bash
./bin/collision
```

出力例:

```
Listening on 127.0.0.1:31080
```

### クライアントの実行

ターミナル2でクライアントアプリケーションを実行:

```bash
./bin/simpleticket
```

クライアントは以下の処理を行います:

1. **チケット作成**: 4人のプレイヤー (Player1, Player2, Player3, Player4) のチケットを作成
2. **マッチング監視**: 各チケットに対してWatchAssignments APIを使用してマッチング結果を監視
3. **結果表示**: マッチが見つかったときに接続先サーバー情報を表示
4. **クリーンアップ**: 30秒後またはすべてのマッチが完了したら、残りのチケットを削除

### 期待される出力

```bash
Connecting to 127.0.0.1:31080
Creating tickets for players...
Created ticket abc123 for player Player1
Created ticket def456 for player Player2
Created ticket ghi789 for player Player3
Created ticket jkl012 for player Player4

✅ Created 4 tickets successfully
Starting to watch for assignments...

Watching assignments for ticket abc123...
Watching assignments for ticket def456...
Watching assignments for ticket ghi789...
Watching assignments for ticket jkl012...

🎉 MATCH FOUND! Ticket abc123 assigned to server: brave-monkey-42
🎉 MATCH FOUND! Ticket def456 assigned to server: brave-monkey-42
🎉 MATCH FOUND! Ticket ghi789 assigned to server: wise-elephant-73
🎉 MATCH FOUND! Ticket jkl012 assigned to server: wise-elephant-73

🏁 All assignment watching completed

Cleaning up tickets...
Deleted ticket abc123
Deleted ticket def456
Deleted ticket ghi789
Deleted ticket jkl012
```

## アーキテクチャ

### マッチング処理フロー

1. **チケット作成**: クライアントが `CreateTicket` APIでマッチング要求を送信
2. **チケット保存**: サーバーがRedisにチケット情報を保存
3. **マッチング実行**: 1秒ごとにマッチング処理が実行され、2つ以上のチケットがある場合にマッチを作成
4. **Assignment作成**: マッチが作成されるとランダムなサーバー名でAssignmentが生成
5. **通知**: `WatchAssignments` APIを通じてクライアントに結果が通知

### プロジェクト構造

```
.
├── api/                    # Protocol Buffer定義
├── cmd/
│   ├── collision/         # マッチメイキングサーバー
│   └── simpleticket/      # クライアント
├── gen/pb/                # 生成されたgRPC/Protocol Bufferコード
├── domain/                # ドメインロジック
├── handler/               # gRPCハンドラー
├── infrastructure/        # Redis接続など
└── usecase/              # ビジネスロジック
```

## API仕様

### FrontendService

- `CreateTicket(CreateTicketRequest) → CreateTicketResponse`
  - マッチングチケットを作成
- `DeleteTicket(DeleteTicketRequest) → Empty`
  - チケットを削除
- `GetTicket(GetTicketRequest) → Ticket`
  - チケット情報を取得
- `WatchAssignments(WatchAssignmentsRequest) → stream WatchAssignmentsResponse`
  - マッチング結果をストリームで監視

## カスタマイズ

マッチング条件やロジックは `cmd/collision/main.go` の `MatchFunctionSimple1vs1` 関数で定義されています。
より複雑なマッチング条件を実装する場合は、この関数を変更してください。

## License

This project is licensed under the MIT License.
