# 結果整合性とdeIndexTicketsによる自動修復メカニズム

## 概要

minimatchでは「TTLで削除されたチケットは後でdeIndexTicketsで修復される」という結果整合性（Eventual Consistency）のアプローチを採用しています。この仕組みを詳しく解説します。

## Redisのデータ構造

minimatchはRedisに以下の2種類のデータを**別々のキー**で保存しています：

### 1. チケットデータ（個別キー）

```
キー: ticket:123
値: {id: "123", search_fields: {...}, ...} (Protocol Buffersでエンコード)
有効期限: TTLあり（例: 10分）
```

### 2. チケットインデックス（共有キー）

```
キー: allTickets
型: Set
値: ["123", "456", "789", ...]
有効期限: なし（永続）
```

**重要**: この2つは**独立したキー**なので、**同時に更新されない**ことがあります。

## 問題：TTLによる不整合

### チケット作成時

```go
// redis.go:101-122
func (s *RedisStore) CreateTicket(ctx context.Context, ticket *pb.Ticket, ttl time.Duration) error {
    queries := []rueidis.Completed{
        // 1. チケットデータを保存（TTLあり）
        s.client.B().Set().
            Key(redisKeyTicketData(s.opts.keyPrefix, ticket.Id)).
            Value(rueidis.BinaryString(data)).
            Ex(ttl).  // ← TTL設定！
            Build(),
        // 2. チケットインデックスに追加（TTLなし）
        s.client.B().Sadd().
            Key(redisKeyTicketIndex(s.opts.keyPrefix)).
            Member(ticket.Id).
            Build(),
    }
    // 同時実行
    for _, resp := range s.client.DoMulti(ctx, queries...) { ... }
}
```

### TTLによる削除の問題

時間が経つと：

```
t=0    : ticket:123作成、allTicketsに追加
t=10分 : Redisがticket:123をTTLで自動削除
        しかし！allTicketsには"123"が残り続ける
```

これにより**不整合**が発生します：

- `allTickets`には`"123"`が存在 ✅
- `ticket:123`のデータは存在しない ❌

## 解決策：結果整合性とdeIndexTickets

minimatchは、この不整合を**後で検出して修復**します。

### フロー図

```
[Backend Tick開始]
     |
     v
1. GetActiveTicketIDs()
   - allTicketsから取得: ["123", "456", "789"]
   - 戻り値: ["123", "456", "789"]

     |
     v
2. GetTickets(["123", "456", "789"])
   - ticket:123を取得 → 存在しない！（TTLで削除済み）
   - ticket:456を取得 → 成功
   - ticket:789を取得 → 成功

     |
     v
3. 存在しないチケットを検出
   - ticketIDsDeleted = ["123"]

     |
     v
4. deIndexTickets(["123"]) ← ここで修復！
   - allTicketsから"123"を削除
   - 不整合が解消される
```

### 実装コード

#### GetTickets (`redis.go:163`)

```go
func (s *RedisStore) GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error) {
    tickets := make([]*pb.Ticket, 0, len(ticketIDs))

    // プライマリから取得を試みる
    ticketsInPrimary, ticketIDsDeleted, err := s.getTickets(ctx, s.client, ticketIDs)
    if err != nil {
        return nil, err
    }
    tickets = append(tickets, ticketsInPrimary...)

    // ★重要：見つからないチケットIDがあればインデックスから削除
    if len(ticketIDsDeleted) > 0 {
        // Tickets not in the primary node are deleted by TTL.
        // It is deleted from the ticket index as well.
        _ = s.deIndexTickets(ctx, ticketIDsDeleted)
    }

    return tickets, nil
}
```

#### getTickets (`redis.go:390`)

```go
func (s *RedisStore) getTickets(ctx context.Context, client rueidis.Client, ticketIDs []string) (
    []*pb.Ticket,    // 取得できたチケット
    []string,        // 見つからなかったチケットID
    error,
) {
    // MGETで一括取得
    keys := make([]string, len(ticketIDs))
    for i, tid := range ticketIDs {
        keys[i] = redisKeyTicketData(s.opts.keyPrefix, tid)
    }
    mgetMap, err := rueidis.MGet(client, ctx, keys)

    tickets := make([]*pb.Ticket, 0, len(keys))
    var ticketIDsNotFound []string

    for key, resp := range mgetMap {
        if err := resp.Error(); err != nil {
            if rueidis.IsRedisNil(err) {
                // ★存在しないチケットIDを記録
                ticketIDsNotFound = append(ticketIDsNotFound,
                    ticketIDFromRedisKey(s.opts.keyPrefix, key))
                continue
            }
            return nil, nil, err
        }
        // デコード成功したチケットを追加
        ticket, _ := decodeTicket(data)
        tickets = append(tickets, ticket)
    }

    return tickets, ticketIDsNotFound, nil
}
```

#### deIndexTickets (`redis.go:436`)

```go
func (s *RedisStore) deIndexTickets(ctx context.Context, ticketIDs []string) error {
    // ロック取得（詳細は後述）
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    if err != nil {
        return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
    }
    defer unlock()

    cmds := []rueidis.Completed{
        // Pendingインデックスから削除
        s.client.B().Zrem().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Member(ticketIDs...).Build(),
        // ★チケットインデックスから削除（これで修復！）
        s.client.B().Srem().Key(redisKeyTicketIndex(s.opts.keyPrefix)).Member(ticketIDs...).Build(),
    }

    for _, resp := range s.client.DoMulti(lockedCtx, cmds...) {
        if err := resp.Error(); err != nil {
            return fmt.Errorf("failed to deindex tickets: %w", err)
        }
    }
    return nil
}
```

## 結果整合性（Eventual Consistency）の意味

### 定義

**強整合性（Strong Consistency）とは異なり、一時的な不整合を許容するが、最終的には整合性が保たれる**

### minimatchでの適用

```
t=0    : チケット作成
         - ticket:123 ✅
         - allTickets: ["123"] ✅
         状態: 整合性OK

t=10分 : TTLでticket:123削除
         - ticket:123 ❌（Redisが自動削除）
         - allTickets: ["123"] ✅（残ったまま）
         状態: 不整合！

t=10分+α : GetTicketsが呼ばれる
         - ticket:123を取得できない
         - deIndexTicketsを呼び出し
         - allTicketsから"123"を削除
         状態: 整合性回復！✅
```

### 重要な設計思想

コメントより (`redis.go:195-197`):

```go
// GetActiveTicketIDs may also retrieve tickets deleted by TTL.
// This is because the ticket index and Ticket data are stored in separate keys.
// The next `GetTickets` call will resolve this inconsistency.
```

つまり：

1. `GetActiveTicketIDs`は削除済みチケットIDを返すことがある（不整合）
2. **次の`GetTickets`呼び出しで自動的に修復される**（結果整合性）

## GetTicket（単数）の場合

Frontend API（ユーザーがチケット取得）の場合も考慮されています (`redis.go:155-158`):

```go
func (s *RedisStore) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
    // ...
    ticket, err := s.getTicket(ctx, s.client, ticketID)
    if err != nil {
        // ErrTicketNotFound is here in the deletion by TTL,
        // and it looks like deIndexTickets is necessary, but it is not
        // because deIndex is done behind the scenes by Backend's GetTickets call.
        return nil, err
    }
    // ...
}
```

**ポイント**:

- Frontend APIでは`deIndexTickets`を**呼ばない**
- なぜなら、Backend側の`GetTickets`が後で自動的に修復してくれるから
- これによりFrontendのパフォーマンスが向上

## なぜロックが必要か

`deIndexTickets`はインデックスを変更するので、ロックが必要です (`redis.go:437-444`):

```go
// Acquire locks to avoid race condition with GetActiveTicketIDs.
//
// Without locks, when the following order,
// The assigned ticket is fetched again by the other backend, resulting in overlapping matches.
//
// 1. (GetActiveTicketIDs) getAllTicketIDs
// 2. (deIndexTickets) ZREM and SREM from ticket index
// 3. (GetActiveTicketIDs) getPendingTicketIDs
```

ロックなしだと：

```
Backend A                          Backend B
---------------------------------------------------
1. getAllTicketIDs()
   → ["123", "456"]
                                   2. deIndexTickets(["123"])
                                      → "123"をインデックスから削除
3. getPendingTicketIDs()
   → []
4. "123"をPendingに設定
   → しかし"123"は既に削除済み！

結果: "123"が二重にマッチングされる可能性
```

## 他のdeIndexTickets呼び出し箇所

### 1. AssignTickets (`redis.go:306`)

```go
if len(assignedTicketIDs) > 0 {
    // de-index assigned tickets
    if err := s.deIndexTickets(ctx, assignedTicketIDs); err != nil {
        return notAssignedTicketIDs, fmt.Errorf("failed to deindex assigned tickets: %w", err)
    }
    // ...
}
```

**理由**: 割り当て済みチケットは今後のマッチングから除外する必要があるため

### 2. DeleteTicket (`redis.go:125-142`)

```go
func (s *RedisStore) DeleteTicket(ctx context.Context, ticketID string) error {
    lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
    // ...
    queries := []rueidis.Completed{
        s.client.B().Del().Key(redisKeyTicketData(...)).Build(),
        s.client.B().Srem().Key(redisKeyTicketIndex(...)).Build(),  // 手動削除
        s.client.B().Zrem().Key(redisKeyPendingTicketIndex(...)).Build(),
    }
    // ...
}
```

**理由**: ユーザーがチケットを削除した場合は、データとインデックス両方を同時に削除

## まとめ

### 結果整合性による自動修復の流れ

1. **問題発生**: TTLがチケットデータのみ削除、インデックスは残る
2. **検出**: `GetTickets`がデータ取得を試みて存在しないことを検出
3. **修復**: `deIndexTickets`が自動的にインデックスから削除
4. **結果**: 最終的に整合性が取れた状態になる

### メリット

1. **パフォーマンス**: データ読み取り時にロック不要
2. **スケーラビリティ**: Read Replicaからの並列読み取り可能
3. **シンプルさ**: TTLによる自動クリーンアップ + 後処理で修復

### トレードオフ

1. **一時的な不整合**: 削除されたチケットIDが一時的に取得される可能性
2. **後処理のコスト**: `deIndexTickets`の呼び出しオーバーヘッド
3. **結果整合性の受容**: 強整合性が必要なシステムには不向き

しかし、minimatchのようなマッチメイキングシステムでは、この程度の不整合は許容可能であり、パフォーマンスとのトレードオフとして合理的な設計判断です。
