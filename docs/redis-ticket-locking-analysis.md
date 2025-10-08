# Redisからのチケット取得時のロック戦略解析

## 質問

> Redisからticketの情報を取得する際はlockを利用していないようですがこれは問題ありませんか

## 結論

**問題ありません。** minimatchは、必要な箇所でのみロックを使用する戦略的な設計になっています。

## ロック戦略の詳細

### ロックが使用される場所

minimatchでは、**チケットインデックスの変更を伴う操作**でのみロックを取得します：

#### 1. `GetActiveTicketIDs` (`redis.go:198`)

```go
func (s *RedisStore) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error) {
    // Acquire a lock to prevent multiple backends from fetching the same Ticket.
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    if err != nil {
        return nil, fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
    }
    defer unlock()
    
    // ... チケットIDを取得してpending状態にする ...
}
```

**ロックの目的:**

- 複数のバックエンドが同じチケットを同時に取得するのを防ぐ
- チケットインデックスの読み取りとpending状態への変更を原子的に実行

#### 2. `DeleteTicket` (`redis.go:125`)

```go
func (s *RedisStore) DeleteTicket(ctx context.Context, ticketID string) error {
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    // ...
}
```

#### 3. `ReleaseTickets` (`redis.go:273`)

```go
func (s *RedisStore) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    // ...
}
```

#### 4. `deIndexTickets` (`redis.go:436`)

```go
func (s *RedisStore) deIndexTickets(ctx context.Context, ticketIDs []string) error {
    // Acquire locks to avoid race condition with GetActiveTicketIDs.
    //
    // Without locks, when the following order,
    // The assigned ticket is fetched again by the other backend, resulting in overlapping matches.
    //
    // 1. (GetActiveTicketIDs) getAllTicketIDs
    // 2. (deIndexTickets) ZREM and SREM from ticket index
    // 3. (GetActiveTicketIDs) getPendingTicketIDs
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    // ...
}
```

### ロックが**使用されない**場所とその理由

#### `GetTicket` / `GetTickets` (`redis.go:145`, `redis.go:163`)

```go
func (s *RedisStore) GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error) {
    // ロックなし！
    tickets := make([]*pb.Ticket, 0, len(ticketIDs))
    
    // Read Replicaから読み取り（オプション）
    if s.opts.readReplicaClient != nil {
        ticketsInReplica, ticketIDsNotFound, err := s.getTickets(ctx, s.opts.readReplicaClient, ticketIDs)
        // ...
    }
    
    // プライマリから読み取り
    ticketsInPrimary, ticketIDsDeleted, err := s.getTickets(ctx, s.client, ticketIDs)
    // ...
}
```

**ロックが不要な理由:**

1. **読み取り専用操作**
   - チケットデータの取得のみを行う
   - インデックスの変更を伴わない

2. **Redisの個別キーアクセスは原子的**
   - 各チケットは独立したキー(`ticket:ID`)に保存されている
   - `MGET`によるバッチ読み取りは、各キーの読み取りが原子的

3. **整合性の取れた不整合処理**
   - TTLで削除されたチケットは後で`deIndexTickets`で処理される
   - Read Replicaとの遅延は許容される設計

## 一貫性モデル

### Eventual Consistency（結果整合性）

minimatchは結果整合性を採用しています：

#### ケース1: TTLによるチケット削除

```
1. チケットインデックスにticket:123が存在
2. Redisがticket:123のデータをTTLで削除
3. GetActiveTicketIDsがticket:123を返す（インデックスはまだ存在）
4. GetTicketsがticket:123を取得しようとするが見つからない
5. GetTicketsがdeIndexTicketsを呼び出してインデックスから削除
```

**コメントより (`redis.go:195`):**

```go
// GetActiveTicketIDs may also retrieve tickets deleted by TTL.
// This is because the ticket index and Ticket data are stored in separate keys.
// The next `GetTickets` call will resolve this inconsistency.
```

#### ケース2: Read Replicaの遅延

```go
// Missing tickets in read replica are due to either TTL or replication delay.
// (redis.go:171)
ticketIDs = ticketIDsNotFound
```

- Read Replicaで見つからないチケットはプライマリから取得
- レプリケーション遅延は許容される

### Strong Consistency（強整合性）が必要な箇所

**チケットの取得とPending化は原子的に実行される必要がある:**

```go
// In order to avoid race conditions with other Ticket Index changes, 
// get tickets and set them to pending state should be done atomically.
// (redis.go:200)
```

これが`GetActiveTicketIDs`でロックを使用する理由です。

## パフォーマンス最適化

### 1. ロックの最小化

ロックは必要最小限に抑えられています：

- **ロック有り**: インデックス変更操作（GetActiveTicketIDs、deIndexTickets）
- **ロック無し**: データ読み取り操作（GetTicket、GetTickets）

### 2. Read Replicaサポート

```go
if s.opts.readReplicaClient != nil {
    // fast return if it is in read replica
    ticket, err := s.getTicket(ctx, s.opts.readReplicaClient, ticketID)
    if err == nil {
        return ticket, nil
    }
}
```

(`redis.go:146`)

- 読み取り負荷をレプリカに分散
- ロックなしで並列読み取り可能

### 3. Ticket Existence Validation（オプション）

```go
if b.options.validateTicketsBeforeAssign {
    filteredAsgs, notAssigned, err := b.validateTicketsBeforeAssign(ctx, asgs)
    // ...
}
```

(`backend.go:237`)

**デフォルトで有効、パフォーマンス重視の場合は無効化可能:**

```go
backend, err := minimatch.NewBackend(
    store, 
    assigner, 
    minimatch.WithTicketValidationBeforeAssign(false)
)
```

## 実装における重要な設計判断

### 1. 分散ロック vs パフォーマンス

- **最小限のロック使用**: インデックス操作のみ
- **ロックなし読み取り**: 高スループット実現

### 2. 整合性 vs 可用性（CAP定理）

minimatchは**AP（Availability + Partition tolerance）**を選択：

- 一時的な不整合を許容（Eventual Consistency）
- 高可用性とパフォーマンスを優先

### 3. トレードオフの明示

`docs/consistency.md`で整合性の限界を明示：

> even with this validation enabled, invalid Assignment cannot be completely prevented.
> For example, if a ticket is deleted during step 5, an invalid Assignment will still occur.

## まとめ

### ロック戦略の正当性

1. **インデックス操作はロック保護**: 二重マッチングを防止
2. **データ読み取りはロックなし**: パフォーマンス最適化
3. **結果整合性による不整合解決**: deIndexTicketsによる自動修復
4. **Read Replicaサポート**: スケーラビリティ向上

### minimatchの設計哲学

> In general, there is a trade-off between consistency and performance. As a distributed system over the Internet, we must accept a certain amount of inconsistency.

（`docs/consistency.md:5`）

**結論**: Redisからのチケット取得時にロックを使用しないのは、**意図的な設計判断**であり、パフォーマンスとスケーラビリティのための最適化です。必要な箇所（インデックス変更）では適切にロックが使用されており、問題ありません。
