# minimatchのマッチメイキング実現方法の解説

## 概要

minimatchは、OpenMatchを参考にした簡易化されたマッチメイキングシステムです。OpenMatchと互換性のあるAPIを提供しながらも、単一のGoプロセスで動作し、状態管理にRedisを使用することでスケーラビリティを実現しています。

## アーキテクチャ

minimatchは主に2つのコンポーネントから構成されます：

1. **Frontend Service**: チケット作成・削除・取得・アサインメント監視を行うAPIサーバー
2. **Backend Service**: チケットを取得し、マッチメイキングを実行するジョブ

## 主要コンポーネント

### 1. Frontend Service (`frontend.go`)

#### 役割

- プレイヤーからのチケット作成リクエストを受け付ける
- チケットの状態照会・削除を行う
- マッチング完了後のアサインメント（接続情報）をプレイヤーに通知する

#### 主要な処理フロー

**チケット作成** (`CreateTicket` - frontend.go:70)

```text
1. リクエストからチケットをクローン
2. ユニークなTicket IDを生成 (xid.New())
3. 作成時刻とTTL（Time To Live）を設定
4. RedisStoreにチケットを保存
```

**アサインメント監視** (`WatchAssignments` - frontend.go:121)

```text
1. クライアントからのストリーミング接続を受け入れる
2. 100msごとにRedisからアサインメント情報をポーリング
3. アサインメントが変更された場合、クライアントに通知
4. アサインメントが割り当てられるまで継続
```

### 2. Backend Service (`backend.go`)

#### 役割

- アクティブなチケットを取得
- MatchFunctionを実行してマッチを生成
- Evaluatorでマッチを評価（オプション）
- Assignerでゲームサーバー情報を割り当て

#### 処理サイクル（Tick）の詳細フロー

Backend Serviceは、`tickRate`（例：1秒）ごとに以下のTickサイクルを実行します (backend.go:137):

```text
┌─────────────────────────────────────────────┐
│ 1. fetchActiveTickets                       │
│    - アクティブなチケットをRedisから取得    │
│    - ロックを取得して同時実行を防止         │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│ 2. makeMatches                              │
│    - 各MatchProfileに対して並列実行         │
│    - チケットをPoolでフィルタリング         │
│    - MatchFunctionでマッチを生成            │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│ 3. evaluateMatches (オプション)             │
│    - 複数のMatchから最適なものを選択        │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│ 4. unmatchedチケットをリリース              │
│    - マッチしなかったチケットを解放         │
│    - 次のTickで再度マッチング対象にする     │
└──────────────────┬──────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────┐
│ 5. assign                                   │
│    - Assignerでゲームサーバー情報を取得     │
│    - チケットの存在を検証（オプション）     │
│    - Redisにアサインメント情報を保存        │
│    - チケットをインデックスから削除         │
└─────────────────────────────────────────────┘
```

### 3. State Store (Redis) (`pkg/statestore/redis.go`)

#### Redisに保存されるデータ構造

1. **Ticket Index** (`allTickets`): すべてのアクティブなチケットIDを管理するSet
2. **Pending Ticket Index** (`proposed_ticket_ids`): 処理中のチケットIDを管理するSorted Set（タイムスタンプでスコアリング）
3. **Ticket Data** (`{ticketID}`): 各チケットの詳細データ（Protocol Buffer形式）
4. **Assignment Data** (`assign:{ticketID}`): 各チケットに割り当てられたゲームサーバー情報

#### チケットのライフサイクル

```text
[作成] CreateTicket (redis.go:101)
  │
  ├─ SET ticket:{id} (データ保存、TTL設定)
  └─ SADD allTickets {id} (インデックスに追加)
  │
  ▼
[取得] GetActiveTicketIDs (redis.go:198)
  │
  ├─ ロック取得 (fetchTicketsLock)
  ├─ SRANDMEMBER allTickets (ランダムに取得)
  ├─ ZRANGEBYSCORE proposed_ticket_ids (処理中チケット確認)
  ├─ 差分計算（全チケット - 処理中チケット）
  └─ ZADD proposed_ticket_ids (処理中に設定)
  │
  ▼
[マッチング成功] AssignTickets (redis.go:287)
  │
  ├─ SET assign:{id} (アサインメント保存、TTL設定)
  ├─ ZREM proposed_ticket_ids {id} (処理中から削除)
  ├─ SREM allTickets {id} (インデックスから削除)
  └─ EXPIRE ticket:{id} (有効期限を短く設定)
  │
  ▼
[削除] TTLによる自動削除 or DeleteTicket
```

### 4. Match Function (`matchfunction.go`)

#### インターフェース定義

```go
type MatchFunction interface {
    MakeMatches(ctx context.Context, profile *pb.MatchProfile,
                poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error)
}
```

#### 組み込み実装: Simple1vs1 (minimatch.go:105)

```text
1. 各Poolのチケットリストを取得
2. チケットリストから2つずつペアを作成
3. 各ペアをMatchオブジェクトに変換
4. 余ったチケット（奇数の場合）は次のTickまで待機
```

#### カスタムMatchFunctionの実装

ユーザーは独自のマッチングロジックを実装可能：

- スキルベースマッチング（ELOレーティングなど）
- レイテンシベースマッチング（地域別）
- 複雑な条件（パーティーサイズ、プレイヤーランクなど）

### 5. Evaluator (`evaluator.go`)

#### 役割

複数のMatchProfileから生成された多数のMatchの中から、最適なものを選択します。

#### インターフェース定義

```go
type Evaluator interface {
    Evaluate(ctx context.Context, matches []*pb.Match) ([]string, error)
}
```

#### 使用ケース

- 重複チケットの競合解決（1つのチケットが複数のMatchに含まれる場合）
- 品質スコアに基づくマッチの優先順位付け
- リソース制約に基づくマッチの選択

### 6. Assigner (`assigner.go`)

#### 役割

確定したMatchに対して、実際のゲームサーバー接続情報を割り当てます。

#### インターフェース定義

```go
type Assigner interface {
    Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)
}
```

#### 典型的な実装

- Agonesとの統合（ゲームサーバー割り当て）
- カスタムゲームサーバー管理システムとの統合
- ダミー実装（テスト用、simple1vs1の例 - simple1vs1.go:41）

## データの流れ

### 1. プレイヤー参加時

```text
プレイヤーA → Frontend.CreateTicket() → Redis (allTickets + ticket data)
プレイヤーB → Frontend.CreateTicket() → Redis (allTickets + ticket data)
```

### 2. マッチング実行（Backend Tick）

```text
Backend.Tick()
  │
  ├─ GetActiveTicketIDs() → Redis
  │   │
  │   └─ [ticketA, ticketB] (ロック取得、pending状態に設定)
  │
  ├─ filterTickets() → Poolでフィルタリング
  │   │
  │   └─ {"pool1": [ticketA, ticketB]}
  │
  ├─ MatchFunction.MakeMatches()
  │   │
  │   └─ Match{tickets: [ticketA, ticketB]}
  │
  ├─ Evaluator.Evaluate() (オプション)
  │   │
  │   └─ [matchID] (選択されたマッチ)
  │
  └─ Assigner.Assign()
      │
      └─ AssignmentGroup{ticketIDs: [ticketA, ticketB],
                          assignment: {connection: "gameserver-123:7777"}}
```

### 3. アサインメント保存と通知

```text
Backend.AssignTickets()
  │
  ├─ Redis.SetAssignment(ticketA) → assign:ticketA
  ├─ Redis.SetAssignment(ticketB) → assign:ticketB
  ├─ Redis.DeindexTickets([ticketA, ticketB])
  │   ├─ ZREM proposed_ticket_ids
  │   └─ SREM allTickets
  │
  ▼
プレイヤーA ← Frontend.WatchAssignments() ← Redis (assign:ticketA)
プレイヤーB ← Frontend.WatchAssignments() ← Redis (assign:ticketB)
```

## 重要なアルゴリズムとメカニズム

### 1. 分散ロックによる同時実行制御

複数のBackendインスタンスが同じチケットを処理しないよう、Redis分散ロック（rueidislock）を使用 (redis.go:126, 201):

```text
fetchTicketsLock → 同時にチケット取得できるのは1つのBackendのみ
```

### 2. Pending状態管理

チケットが処理中かどうかをSorted Setで管理 (redis.go:260):

```text
ZADD proposed_ticket_ids {score: timestamp} {member: ticketID}

- スコア（タイムスタンプ）により、タイムアウト処理が可能
- pendingReleaseTimeout（デフォルト1分）を超えたチケットは自動解放
```

### 3. TTLによる自動クリーンアップ

```text
- チケット作成時: defaultTicketTTL = 10分
- アサインメント保存時: assignedDeleteTimeout = 1分
```

これにより、プレイヤーが切断した場合でも自動的にリソースが解放されます。

### 4. チケット検証（Validation）

アサインメント前にチケットの存在を確認（デフォルト有効、backend.go:237）:

```text
1. AssignmentGroupの全チケットIDを抽出
2. Redis.GetTickets() で存在確認
3. 存在しないチケットは除外してアサインメント
```

これにより、TTLで削除されたチケットへの不正なアサインメントを防ぎます。

### 5. Read ReplicaサポートによるRead負荷分散

オプションでRedis Read Replicaを設定可能 (redis.go:146):

```text
GetTicket() → まずRead Replicaを確認 → 失敗時のみPrimaryに問い合わせ
```

## 使用例: Simple 1vs1マッチング

simple1vs1の例 (examples/simple1vs1/simple1vs1.go) では以下のように実装されています：

```text
1. MiniMatchインスタンス作成（miniredis使用）
2. MatchProfileを定義（pools: ["test-pool"]）
3. MatchFunction登録（MatchFunctionSimple1vs1）
4. Backend起動（tickRate: 1秒）
5. Frontend起動（:50504）

マッチング処理:
- 2つのチケットが揃ったら即座にマッチ
- dummyAssign()でランダムな接続文字列を割り当て
```

## OpenMatchとの主な違い

1. **単一プロセス**: Kubernetes不要、シンプルなデプロイ
2. **状態管理**: すべての状態をRedisに保存（Backendはステートレス）
3. **スケーラビリティ**: 複数Backendインスタンスの並列実行が可能
4. **シンプルさ**: OpenMatchの複雑なコンポーネント（Query Service、Synchronizerなど）を省略

詳細は `docs/differences.md` を参照。

## パフォーマンス特性

- **チケット取得制限**: defaultFetchTicketsLimit = 10000（OOM防止）
- **Tick間隔**: 推奨1秒（カスタマイズ可能）
- **メトリクス**: OpenTelemetry形式でメトリクス公開

詳細は `docs/metrics.md` を参照。

## まとめ

minimatchは、以下の要素を組み合わせてマッチメイキングを実現しています：

1. **Frontend**: プレイヤーとの接点（チケット作成・アサインメント通知）
2. **Backend**: 定期的なTick処理でマッチング実行
3. **Redis**: チケット・アサインメント状態の永続化と同期
4. **MatchFunction**: カスタマイズ可能なマッチングロジック
5. **Evaluator**: マッチの優先順位付け（オプション）
6. **Assigner**: ゲームサーバー割り当て

この設計により、OpenMatch互換のAPIを保ちながら、シンプルで高スケーラブルなマッチメイキングシステムを実現しています。
