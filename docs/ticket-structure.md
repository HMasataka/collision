# pb.Ticket の構造詳細解説

## 概要

`pb.Ticket` は、マッチメイキングに参加するプレイヤーを表す Protocol Buffer メッセージです。
OpenMatch の Ticket と互換性があり、プレイヤーの検索条件、状態、メタデータを含みます。

## Protocol Buffer 定義

**ファイル:** `api/openmatch/messages.proto:8`

```protobuf
message Ticket {
  string id = 1;
  Assignment assignment = 3;
  SearchFields search_fields = 4;
  map<string, google.protobuf.Any> extensions = 5;
  map<string, google.protobuf.Any> persistent_field = 6;
  google.protobuf.Timestamp create_time = 7;
  reserved 2;
}
```

## フィールド詳細

### 1. id (string)

**用途:** チケットの一意識別子

**生成方法:** Frontend Service が `xid` ライブラリで自動生成 (frontend.go:75)

**形式:** `"cko1234abc"`（xid形式: タイムスタンプベースの短いID）

**具体例:**

```go
// Frontend での生成
ticket.Id = xid.New().String()
// 結果: "cko1234abc", "ckp5678def" など
```

**特徴:**

- プレイヤーが指定することはできない（サーバー側で生成）
- ソート可能（生成時刻順）
- 衝突確率が極めて低い（マイクロ秒精度 + ランダム要素）

### 2. assignment (\*Assignment)

**用途:** マッチング完了後のゲームサーバー接続情報

**初期状態:** `nil`（未アサイン）

**設定タイミング:** Backend の `AssignTickets` 実行後 (redis.go:377)

**データ構造:**

```protobuf
message Assignment {
  string connection = 1;
  map<string, google.protobuf.Any> extensions = 4;
}
```

**具体例:**

```go
// Assigner による設定例
assignment := &pb.Assignment{
    Connection: "gameserver-123.example.com:7777",
    Extensions: map[string]*anypb.Any{
        "sessionId": anypb.New(wrapperspb.String("session-xyz789")),
        "region": anypb.New(wrapperspb.String("us-west-1")),
        "matchId": anypb.New(wrapperspb.String("match-abc123")),
    },
}
```

**Connection フィールド:**

- ゲームサーバーへの接続文字列
- 形式は自由（IP:Port、URL、任意の文字列など）

**Extensions の使用例:**

- セッションID、トークン
- マッチID、ルームID
- サーバーリージョン情報
- 暗号化キー

### 3. search_fields (\*SearchFields)

**用途:** マッチング条件を定義する検索フィールド

**重要度:** ★★★ マッチメイキングの核心部分

**データ構造:**

```protobuf
message SearchFields {
  map<string, double> double_args = 1;
  map<string, string> string_args = 2;
  repeated string tags = 3;
}
```

#### 3-1. double_args (map<string, double>)

**用途:** 数値ベースのマッチング条件（スキル、レイテンシなど）

**使用例:**

```go
SearchFields: &pb.SearchFields{
    DoubleArgs: map[string]float64{
        "skill":    1500.0,    // MMR/ELO レーティング
        "latency":  50.0,      // ミリ秒
        "playTime": 120.5,     // プレイ時間（時間）
        "level":    25.0,      // プレイヤーレベル
        "winRate":  0.625,     // 勝率（62.5%）
    },
}
```

**Pool でのフィルタリング:**

```go
// MatchProfile で定義
pools: []*pb.Pool{
    {
        Name: "skilled-players",
        DoubleRangeFilters: []*pb.DoubleRangeFilter{
            {
                DoubleArg: "skill",
                Min:       1400.0,
                Max:       1600.0,
                Exclude:   pb.DoubleRangeFilter_NONE, // [1400, 1600]
            },
            {
                DoubleArg: "latency",
                Min:       0.0,
                Max:       100.0,
                Exclude:   pb.DoubleRangeFilter_MAX,  // [0, 100)
            },
        },
    },
}
```

**Exclude オプション (messages.proto:34):**

- `NONE`: `[min, max]`（両端含む）
- `MIN`: `(min, max]`（最小値除く）
- `MAX`: `[min, max)`（最大値除く）
- `BOTH`: `(min, max)`（両端除く）

**フィルタリングロジック (filter.go:79-105):**

```go
// チケットの skill が 1500.0 の場合
// Filter: {DoubleArg: "skill", Min: 1400, Max: 1600, Exclude: NONE}
v := ticket.SearchFields.DoubleArgs["skill"] // 1500.0
if v >= 1400 && v <= 1600 {
    // マッチ！
}
```

#### 3-2. string_args (map<string, string>)

**用途:** 文字列ベースのマッチング条件（言語、プラットフォームなど）

**使用例:**

```go
SearchFields: &pb.SearchFields{
    StringArgs: map[string]string{
        "language":  "ja",           // 言語
        "region":    "asia",         // リージョン
        "platform":  "pc",           // プラットフォーム
        "gameMode":  "ranked",       // ゲームモード
        "partySize": "2",            // パーティーサイズ
    },
}
```

**Pool でのフィルタリング:**

```go
pools: []*pb.Pool{
    {
        Name: "japanese-players",
        StringEqualsFilters: []*pb.StringEqualsFilter{
            {StringArg: "language", Value: "ja"},
            {StringArg: "platform", Value: "pc"},
        },
    },
}
```

**フィルタリングロジック (filter.go:107-115):**

```go
// 完全一致のみ（部分一致不可）
v := ticket.SearchFields.StringArgs["language"] // "ja"
if filter.Value == v {
    // マッチ！
}
```

#### 3-3. tags ([]string)

**用途:** フラグベースのマッチング条件（複数の属性）

**使用例:**

```go
SearchFields: &pb.SearchFields{
    Tags: []string{
        "mode:casual",           // カジュアルモード
        "region:asia",           // アジアリージョン
        "platform:pc",           // PCプラットフォーム
        "voiceChat:enabled",     // ボイスチャット有効
        "crossPlay:disabled",    // クロスプレイ無効
        "premium:true",          // プレミアムユーザー
    },
}
```

**Pool でのフィルタリング:**

```go
pools: []*pb.Pool{
    {
        Name: "casual-with-voice",
        TagPresentFilters: []*pb.TagPresentFilter{
            {Tag: "mode:casual"},
            {Tag: "voiceChat:enabled"},
        },
    },
}
```

**フィルタリングロジック (filter.go:117-125):**

```go
// すべての Tag が存在する必要がある（AND条件）
for _, filterTag := range pool.TagPresentFilters {
    found := false
    for _, ticketTag := range ticket.SearchFields.Tags {
        if ticketTag == filterTag.Tag {
            found = true
            break
        }
    }
    if !found {
        return false // いずれか1つでも欠けていればマッチしない
    }
}
```

**命名規則のベストプラクティス:**

```text
"key:value" 形式を推奨:
  - "mode:casual"
  - "region:asia"
  - "platform:pc"

フラグのみの場合:
  - "premium"
  - "newPlayer"
  - "tutorial"
```

### 4. extensions (map<string, google.protobuf.Any>)

**用途:** カスタムメタデータの保存（マッチング条件には使用されない）

**特徴:**

- MatchFunction 内で自由に参照可能
- フィルタリングには使用されない
- 任意の Protocol Buffer メッセージを保存可能

**使用例:**

```go
SearchFields: &pb.SearchFields{
    Extensions: map[string]*anypb.Any{
        "playerName": anypb.New(wrapperspb.String("Alice")),
        "avatarId": anypb.New(wrapperspb.Int64(12345)),
        "guildId": anypb.New(wrapperspb.String("guild-xyz")),
        "customData": anypb.New(&CustomMessage{...}),
    },
}
```

**MatchFunction での活用:**

```go
func MakeMatches(ctx context.Context, profile *pb.MatchProfile,
                 poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
    for _, ticket := range poolTickets["pool1"] {
        // Extensions からプレイヤー名を取得
        if nameAny, ok := ticket.Extensions["playerName"]; ok {
            var nameVal wrapperspb.StringValue
            nameAny.UnmarshalTo(&nameVal)
            playerName := nameVal.Value
            // カスタムロジックに使用
        }
    }
}
```

### 5. persistent_field (map<string, google.protobuf.Any>)

**用途:** システム内部で使用する永続フィールド

**使用箇所:** minimatch では TTL 情報の保存に使用 (frontend.go:77-83)

**具体例:**

```go
// Frontend での設定
ttlVal, _ := anypb.New(wrapperspb.Int64(s.options.ticketTTL.Nanoseconds()))
ticket.PersistentField = map[string]*anypb.Any{
    "ttl": ttlVal, // 600000000000 (10分のナノ秒表現)
}
```

**用途の違い:**

- `extensions`: アプリケーション層のメタデータ（ユーザー定義）
- `persistent_field`: システム層のメタデータ（minimatch内部使用）

### 6. create_time (google.protobuf.Timestamp)

**用途:** チケット作成日時

**設定タイミング:** `CreateTicket` 時に自動設定 (frontend.go:76)

**形式:** RFC3339形式のタイムスタンプ

**具体例:**

```go
ticket.CreateTime = timestamppb.Now()
// 結果: "2024-01-01T12:34:56.789Z"
```

**Pool でのフィルタリング:**

```go
pools: []*pb.Pool{
    {
        Name: "recent-tickets",
        CreatedAfter:  timestamppb.New(time.Now().Add(-30*time.Second)),
        CreatedBefore: timestamppb.New(time.Now()),
    },
}
```

**フィルタリングロジック (filter.go:61-77):**

```go
ct := ticket.CreateTime.AsTime()
// 30秒前以降に作成されたチケットのみマッチ
if ct.After(pool.CreatedAfter) && ct.Before(pool.CreatedBefore) {
    // マッチ！
}
```

**使用例:**

- 長時間待機したチケットを優先的にマッチング
- 新規チケットと古いチケットを分離
- タイムアウト処理

## 実際の使用例

### 例1: スキルベースマッチング

```go
ticket := &pb.Ticket{
    SearchFields: &pb.SearchFields{
        DoubleArgs: map[string]float64{
            "skill": 1520.0,  // MMRレーティング
        },
        StringArgs: map[string]string{
            "region": "asia",
        },
        Tags: []string{
            "mode:ranked",
            "platform:pc",
        },
    },
}

// MatchProfile 定義
profile := &pb.MatchProfile{
    Name: "ranked-match",
    Pools: []*pb.Pool{
        {
            Name: "asia-skilled",
            DoubleRangeFilters: []*pb.DoubleRangeFilter{
                {
                    DoubleArg: "skill",
                    Min:       1400.0,
                    Max:       1600.0,
                },
            },
            StringEqualsFilters: []*pb.StringEqualsFilter{
                {StringArg: "region", Value: "asia"},
            },
            TagPresentFilters: []*pb.TagPresentFilter{
                {Tag: "mode:ranked"},
                {Tag: "platform:pc"},
            },
        },
    },
}
```

### 例2: パーティーマッチング

```go
// 4人パーティーのリーダーが作成
ticket := &pb.Ticket{
    SearchFields: &pb.SearchFields{
        StringArgs: map[string]string{
            "partySize": "4",
            "gameMode":  "team-deathmatch",
        },
        Tags: []string{
            "partyPlay:true",
            "voiceChat:enabled",
        },
    },
    Extensions: map[string]*anypb.Any{
        "partyId": anypb.New(wrapperspb.String("party-abc123")),
        "partyLeader": anypb.New(wrapperspb.String("player-xyz")),
        "partyMembers": anypb.New(&PartyMembersMessage{
            Members: []string{"player-1", "player-2", "player-3", "player-4"},
        }),
    },
}

// 4人パーティー同士をマッチング
profile := &pb.MatchProfile{
    Pools: []*pb.Pool{
        {
            Name: "4v4-teams",
            StringEqualsFilters: []*pb.StringEqualsFilter{
                {StringArg: "partySize", Value: "4"},
                {StringArg: "gameMode", Value: "team-deathmatch"},
            },
        },
    },
}
```

### 例3: クロスプレイマッチング (intergration_test.go:114-145)

```go
// PCプレイヤー
t1 := &pb.Ticket{
    SearchFields: &pb.SearchFields{
        Tags: []string{"mode:crossplay", "platform:pc"},
    },
}

// コンソールプレイヤー
t2 := &pb.Ticket{
    SearchFields: &pb.SearchFields{
        Tags: []string{"mode:crossplay", "platform:console"},
    },
}

// クロスプレイ許可プール
profile := &pb.MatchProfile{
    Pools: []*pb.Pool{
        {
            Name: "crossplay-enabled",
            TagPresentFilters: []*pb.TagPresentFilter{
                {Tag: "mode:crossplay"},
            },
        },
    },
}
// → t1 と t2 がマッチング可能
```

### 例4: レイテンシベースマッチング

```go
ticket := &pb.Ticket{
    SearchFields: &pb.SearchFields{
        DoubleArgs: map[string]float64{
            "latency":   45.0,     // ms
            "skill":     1500.0,
        },
        StringArgs: map[string]string{
            "preferredServer": "us-west-1",
        },
    },
}

// 低レイテンシプール
profile := &pb.MatchProfile{
    Pools: []*pb.Pool{
        {
            Name: "low-latency",
            DoubleRangeFilters: []*pb.DoubleRangeFilter{
                {
                    DoubleArg: "latency",
                    Min:       0.0,
                    Max:       60.0,  // 60ms以下
                },
            },
        },
    },
}
```

## チケットのライフサイクル

### 1. 作成フェーズ

```go
// プレイヤーがチケット作成リクエスト
req := &pb.CreateTicketRequest{
    Ticket: &pb.Ticket{
        SearchFields: &pb.SearchFields{
            DoubleArgs: map[string]float64{"skill": 1500.0},
            Tags: []string{"mode:casual"},
        },
    },
}

// Frontend が ID と CreateTime を設定
ticket.Id = xid.New().String()
ticket.CreateTime = timestamppb.Now()
ticket.PersistentField = map[string]*anypb.Any{
    "ttl": anypb.New(wrapperspb.Int64(600000000000)),
}
```

### 2. 待機フェーズ

```text
Redis に保存:
- allTickets Set に追加
- ticket:{id} として保存（TTL: 10分）

プレイヤーは WatchAssignments() で待機
```

### 3. マッチングフェーズ

```go
// Backend が取得
activeTickets := store.GetActiveTicketIDs(ctx, 10000)
tickets := store.GetTickets(ctx, activeTicketIDs)

// Pool でフィルタリング
poolTickets := filterTickets(profile, tickets)
// {"asia-skilled": [ticket1, ticket2, ticket3, ticket4]}

// MatchFunction 実行
matches := mmf.MakeMatches(ctx, profile, poolTickets)
// Match{tickets: [ticket1, ticket2]}
```

### 4. アサインメントフェーズ

```go
// Assigner がゲームサーバー情報を設定
asgs := assigner.Assign(ctx, matches)
// AssignmentGroup{
//   TicketIds: ["ticket1", "ticket2"],
//   Assignment: {Connection: "gameserver:7777"},
// }

// Redis に保存
store.AssignTickets(ctx, asgs)

// チケットに Assignment が設定される
ticket.Assignment = &pb.Assignment{
    Connection: "gameserver-123:7777",
}
```

### 5. 完了フェーズ

```text
WatchAssignments() が Assignment を受信
プレイヤーがゲームサーバーに接続

1分後に自動削除（TTL期限切れ）
```

## ベストプラクティス

### 1. SearchFields の設計

**DO:**

- 数値条件は `double_args` を使用
- 文字列条件は `string_args` を使用
- フラグ条件は `tags` を使用
- カスタムデータは `extensions` を使用

**DON'T:**

- `tags` に数値を入れない（`"skill:1500"` ✗）
- `double_args` に文字列を無理やり入れない
- マッチング不要なデータを SearchFields に入れない

### 2. Tags の命名規則

```go
// 推奨: "key:value" 形式
Tags: []string{
    "mode:casual",
    "region:asia",
    "platform:pc",
}

// 非推奨: 曖昧な命名
Tags: []string{
    "casual",      // 何が casual?
    "asia",        // region? server?
    "pc",          // platform? 明示的に
}
```

### 3. Extensions の活用

```go
// マッチング後の処理で使用するデータ
Extensions: map[string]*anypb.Any{
    "playerName": anypb.New(wrapperspb.String("Alice")),
    "playerLevel": anypb.New(wrapperspb.Int64(25)),
    "avatarUrl": anypb.New(wrapperspb.String("https://...")),
    "guildInfo": anypb.New(&GuildInfo{...}),
}
```

### 4. Pool 設計

```go
// 良い例: 明確な条件分け
pools := []*pb.Pool{
    {
        Name: "beginners",
        DoubleRangeFilters: []*pb.DoubleRangeFilter{
            {DoubleArg: "skill", Min: 0, Max: 1000},
        },
    },
    {
        Name: "intermediate",
        DoubleRangeFilters: []*pb.DoubleRangeFilter{
            {DoubleArg: "skill", Min: 1000, Max: 2000},
        },
    },
    {
        Name: "advanced",
        DoubleRangeFilters: []*pb.DoubleRangeFilter{
            {DoubleArg: "skill", Min: 2000, Max: 3000},
        },
    },
}
```

## まとめ

`pb.Ticket` は以下の要素で構成されます：

1. **id**: システム生成の一意識別子
2. **assignment**: マッチング後のゲームサーバー情報
3. **search_fields**: マッチング条件の中核
   - `double_args`: 数値条件（スキル、レイテンシなど）
   - `string_args`: 文字列条件（言語、プラットフォームなど）
   - `tags`: フラグ条件（モード、機能など）
4. **extensions**: カスタムメタデータ
5. **persistent_field**: システム内部データ（TTLなど）
6. **create_time**: 作成日時

これらを適切に設計することで、柔軟で高性能なマッチメイキングシステムを構築できます。
