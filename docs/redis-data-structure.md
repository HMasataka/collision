# minimatchのRedisデータ構造詳細

## 概要

minimatchは状態管理にRedisを使用し、複数のBackendインスタンスでスケール可能な設計になっています。
このドキュメントでは、Redisに保存される具体的なキー、データ型、値の形式を詳しく解説します。

## Redisキー一覧

### 1. Ticket Index (Set型)

**キー:** `allTickets` (redis.go:512)

**データ型:** Redis Set

**用途:** すべてのアクティブなチケットIDを管理

**ライフサイクル:**

- 作成: `CreateTicket` 時に追加 (redis.go:112)
- 削除: `AssignTickets` または `DeleteTicket` 時に削除 (redis.go:134, 453)

**具体例:**

```redis
Key: "allTickets"
Type: Set
Value: {"cko1234abc", "cko5678def", "cko9012ghi"}

# コマンド例
SADD allTickets "cko1234abc"              # チケット追加
SRANDMEMBER allTickets 10000              # ランダムに最大10000件取得
SCARD allTickets                          # チケット数カウント
SREM allTickets "cko1234abc"              # チケット削除
```

### 2. Pending Ticket Index (Sorted Set型)

**キー:** `proposed_ticket_ids` (redis.go:516)

**データ型:** Redis Sorted Set

**用途:** Backend処理中（Pending状態）のチケットIDを管理

**スコア:** 処理開始時のUnixタイムスタンプ（秒）

**タイムアウト:** 1分（`pendingReleaseTimeout` - redis.go:18）

**ライフサイクル:**

- 作成: `GetActiveTicketIDs` 時に追加 (redis.go:262)
- 削除: `ReleaseTickets` または `AssignTickets` 時に削除 (redis.go:280, 452)

**具体例:**

```redis
Key: "proposed_ticket_ids"
Type: Sorted Set
Members:
  1704067200 -> "cko1234abc"  # 2024-01-01 00:00:00に処理開始
  1704067201 -> "cko5678def"  # 2024-01-01 00:00:01に処理開始
  1704067205 -> "cko9012ghi"  # 2024-01-01 00:00:05に処理開始

# コマンド例
ZADD proposed_ticket_ids 1704067200 "cko1234abc"                    # Pending状態に設定
ZRANGEBYSCORE proposed_ticket_ids 1704067140 1704071200            # 過去1分〜未来1時間
ZREM proposed_ticket_ids "cko1234abc"                               # Pending解除
ZREMRANGEBYSCORE proposed_ticket_ids 0 1704067140                  # タイムアウトチケット削除
```

**タイムアウト処理:**

現在時刻 - 1分より古いスコアのチケットは自動的にリリース対象となります。

```go
// redis.go:244
rangeMin := strconv.FormatInt(time.Now().Add(-s.opts.pendingReleaseTimeout).Unix(), 10)
rangeMax := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)
```

### 3. Ticket Data (String型、Protocol Buffer)

**キー:** `{ticketID}` (redis.go:524)

**データ型:** Redis String (Binary)

**用途:** チケットの詳細データを保存

**データ形式:** Protocol Buffer (`pb.Ticket`) のシリアライズバイナリ

**TTL:** 10分（デフォルト、frontend.go:24）

**ライフサイクル:**

- 作成: `CreateTicket` 時に保存 (redis.go:107)
- 取得: `GetTicket` または `GetTickets` で取得 (redis.go:145, 163)
- 更新: `AssignTickets` 時にTTLを1分に短縮 (redis.go:426)
- 削除: TTL期限切れまたは `DeleteTicket` (redis.go:133)

**具体例:**

```redis
Key: "cko1234abc"
Type: String
Value: <Binary Protocol Buffer data>
TTL: 600 seconds

# コマンド例
SET cko1234abc "\x08\x01\x12\x0acko1234abc..." EX 600   # チケット作成
GET cko1234abc                                           # チケット取得
EXPIRE cko1234abc 60                                     # TTL短縮（アサインメント後）
DEL cko1234abc                                           # チケット削除
```

**Protocol Buffer構造（デシリアライズ後）:**

```json
{
  "id": "cko1234abc",
  "searchFields": {
    "tags": ["mode:casual", "region:asia", "platform:pc"],
    "doubleArgs": {
      "skill": 1500.0,
      "latency": 50.0
    },
    "stringArgs": {
      "language": "ja"
    }
  },
  "createTime": "2024-01-01T00:00:00Z",
  "persistentField": {
    "ttl": {
      "@type": "type.googleapis.com/google.protobuf.Int64Value",
      "value": 600000000000
    }
  }
}
```

### 4. Assignment Data (String型、Protocol Buffer)

**キー:** `assign:{ticketID}` (redis.go:528)

**データ型:** Redis String (Binary)

**用途:** マッチング完了後のゲームサーバー接続情報を保存

**データ形式:** Protocol Buffer (`pb.Assignment`) のシリアライズバイナリ

**TTL:** 1分（デフォルト、redis.go:19）

**ライフサイクル:**

- 作成: `AssignTickets` 時に保存 (redis.go:377)
- 取得: `GetAssignment` で取得（`WatchAssignments`が使用）(redis.go:187)
- 削除: TTL期限切れ（1分後に自動削除）

**具体例:**

```redis
Key: "assign:cko1234abc"
Type: String
Value: <Binary Protocol Buffer data>
TTL: 60 seconds

# コマンド例
SET assign:cko1234abc "\x0a\x15gameserver-123:7777..." EX 60   # アサインメント保存
GET assign:cko1234abc                                           # アサインメント取得
```

**Protocol Buffer構造（デシリアライズ後）:**

```json
{
  "connection": "gameserver-123.example.com:7777",
  "extensions": {
    "sessionId": {
      "@type": "type.googleapis.com/google.protobuf.StringValue",
      "value": "session-xyz789"
    },
    "region": {
      "@type": "type.googleapis.com/google.protobuf.StringValue",
      "value": "us-west-1"
    },
    "matchId": {
      "@type": "type.googleapis.com/google.protobuf.StringValue",
      "value": "match-abc123"
    }
  }
}
```

### 5. Fetch Tickets Lock (分散ロック)

**キー:** `fetchTicketsLock` (redis.go:520)

**データ型:** String (rueidislockライブラリ管理)

**用途:** 複数Backendインスタンス間でチケット取得を排他制御

**TTL:** 短時間（数秒、ロック保持期間）

**使用箇所:**

- `GetActiveTicketIDs` (redis.go:201)
- `DeleteTicket` (redis.go:126)
- `deIndexTickets` (redis.go:445)

**具体例:**

```redis
Key: "fetchTicketsLock"
Type: String
Value: <unique_lock_token_generated_by_rueidislock>
TTL: 5 seconds

# 内部的な動作（rueidislockが管理）
SET fetchTicketsLock "node1-token-123" NX PX 5000   # ロック取得試行
DEL fetchTicketsLock                                 # ロック解放
```

**動作:**

```go
// redis.go:201
lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
if err != nil {
    return nil, fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
}
defer unlock()
```

## データフロー例

### シナリオ: 2人のプレイヤーがマッチングされるまで

#### ステップ1: プレイヤーAがチケット作成

```redis
# Frontend: CreateTicket
SADD allTickets "cko1234abc"
SET cko1234abc "\x08\x01\x12\x0acko1234abc..." EX 600
```

**Redis状態:**

```text
allTickets: {"cko1234abc"}
cko1234abc: <Ticket Data> (TTL: 600s)
```

#### ステップ2: プレイヤーBがチケット作成

```redis
# Frontend: CreateTicket
SADD allTickets "cko5678def"
SET cko5678def "\x08\x02\x12\x0acko5678def..." EX 600
```

**Redis状態:**

```text
allTickets: {"cko1234abc", "cko5678def"}
cko1234abc: <Ticket Data> (TTL: 600s)
cko5678def: <Ticket Data> (TTL: 600s)
```

#### ステップ3: Backend Tick処理開始

```redis
# Backend: GetActiveTicketIDs
SET fetchTicketsLock "backend1-token" NX EX 5
SRANDMEMBER allTickets 10000
# → ["cko1234abc", "cko5678def"]
ZRANGEBYSCORE proposed_ticket_ids 1704067140 1704071200
# → [] (まだ処理中なし)
ZADD proposed_ticket_ids 1704067200 "cko1234abc"
ZADD proposed_ticket_ids 1704067200 "cko5678def"
DEL fetchTicketsLock
```

**Redis状態:**

```text
allTickets: {"cko1234abc", "cko5678def"}
proposed_ticket_ids: {1704067200: "cko1234abc", 1704067200: "cko5678def"}
cko1234abc: <Ticket Data> (TTL: 600s)
cko5678def: <Ticket Data> (TTL: 600s)
```

#### ステップ4: チケットデータ取得

```redis
# Backend: GetTickets
GET cko1234abc
GET cko5678def
```

#### ステップ5: マッチング成功、アサインメント

```redis
# Backend: AssignTickets
SET assign:cko1234abc "\x0a\x15gameserver-123:7777..." EX 60
SET assign:cko5678def "\x0a\x15gameserver-123:7777..." EX 60

SET fetchTicketsLock "backend1-token" NX EX 5
ZREM proposed_ticket_ids "cko1234abc" "cko5678def"
SREM allTickets "cko1234abc" "cko5678def"
DEL fetchTicketsLock

EXPIRE cko1234abc 60
EXPIRE cko5678def 60
```

**Redis状態:**

```text
allTickets: {}
proposed_ticket_ids: {}
cko1234abc: <Ticket Data> (TTL: 60s)
cko5678def: <Ticket Data> (TTL: 60s)
assign:cko1234abc: <Assignment Data> (TTL: 60s)
assign:cko5678def: <Assignment Data> (TTL: 60s)
```

#### ステップ6: プレイヤーがアサインメント受信

```redis
# Frontend: WatchAssignments (100msごとにポーリング)
GET assign:cko1234abc
GET assign:cko5678def
```

#### ステップ7: TTL期限切れで自動削除（60秒後）

```text
allTickets: {}
proposed_ticket_ids: {}
(すべてのキーが削除される)
```

## オプション機能

### 1. Key Prefix（キープレフィックス）

複数のminimatchインスタンスで同じRedisを共有する場合に使用 (redis.go:77):

```go
WithRedisKeyPrefix("mm1:")
```

**適用結果:**

```redis
mm1:allTickets
mm1:proposed_ticket_ids
mm1:fetchTicketsLock
mm1:cko1234abc
mm1:assign:cko1234abc
```

### 2. Separated Assignment Redis（アサインメント専用Redis）

アサインメント関連のキーを別のRedisインスタンスに保存 (redis.go:71):

```go
WithSeparatedAssignmentRedis(assignmentRedisClient)
```

**分離結果:**

```text
メインRedis:
  - allTickets
  - proposed_ticket_ids
  - fetchTicketsLock
  - {ticketID}

アサインメント専用Redis:
  - assign:{ticketID}
```

**メリット:**

- `WatchAssignments` の読み取り負荷を分散
- チケット管理とアサインメント配信のスケーリングを独立化

### 3. Read Replica（読み取りレプリカ）

読み取り専用のRedisレプリカを設定可能 (redis.go:83):

```go
WithRedisReadReplicaClient(replicaRedisClient)
```

**動作 (redis.go:146):**

```go
if s.opts.readReplicaClient != nil {
    ticket, err := s.getTicket(ctx, s.opts.readReplicaClient, ticketID)
    if err == nil {
        return ticket, nil
    }
}
// レプリカで見つからない場合のみプライマリを確認
ticket, err := s.getTicket(ctx, s.client, ticketID)
```

**メリット:**

- `GetTicket` / `GetTickets` の読み取り負荷をレプリカに分散
- プライマリRedisへの負荷軽減

## データ整合性の保証

### 1. 分散ロックによる排他制御

`fetchTicketsLock` により以下の競合を防止:

- 複数Backendが同じチケットを同時に取得
- `GetActiveTicketIDs` と `deIndexTickets` の競合

### 2. TTLによる自動クリーンアップ

- **Ticket Data:** 10分後に自動削除（プレイヤーが切断した場合の対策）
- **Assignment Data:** 1分後に自動削除（メモリ節約）
- **Pending Tickets:** タイムアウト処理で自動リリース（Backend障害時の対策）

### 3. Ticket Validation（チケット検証）

アサインメント前にチケットの存在を確認 (backend.go:257):

```go
tickets, err := b.store.GetTickets(ctx, allTicketIDs)
// 存在しないチケットは除外してアサインメント
```

これにより、以下のケースを防止:

- TTLで削除されたチケットへのアサインメント
- 他のBackendが既にアサインしたチケットの重複アサインメント

## パフォーマンス特性

### メモリ使用量の見積もり

1チケットあたりの概算:

- Ticket Index: 約20バイト（チケットID）
- Pending Ticket Index: 約30バイト（チケットID + スコア）
- Ticket Data: 約200-500バイト（Protocol Buffer）
- Assignment Data: 約100-200バイト（Protocol Buffer）

**例:** 10,000チケット同時接続

```text
Ticket Index: 10,000 × 20B = 200KB
Pending Ticket Index: 10,000 × 30B = 300KB
Ticket Data: 10,000 × 350B = 3.5MB
Assignment Data: 10,000 × 150B = 1.5MB
---
合計: 約5.5MB
```

### Redis負荷の分散戦略

1. **Read Replicaの活用**: `GetTicket` / `GetTickets` の読み取り負荷を分散
2. **Separated Assignment Redis**: `WatchAssignments` のポーリング負荷を別インスタンスに分離
3. **FetchTicketsLimitの設定**: 一度に取得するチケット数を制限（OOM防止）

```go
WithFetchTicketsLimit(10000)  // デフォルト: backend.go:19
```

## まとめ

minimatchのRedisデータ構造は以下の特徴があります：

1. **シンプル:** 5種類のキーのみでマッチメイキングを実現
2. **スケーラブル:** 分散ロックにより複数Backendの並列実行が可能
3. **信頼性:** TTLとタイムアウト処理により自動的にリソースをクリーンアップ
4. **柔軟性:** Key Prefix、Separated Redis、Read Replicaで負荷分散が可能

この設計により、OpenMatch互換でありながらシンプルで運用しやすいマッチメイキングシステムを実現しています。
