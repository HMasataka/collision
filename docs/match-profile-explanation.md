# MatchProfile 詳細解説

## 概要

**MatchProfile**は、minimatchにおけるマッチメイキングの定義を表す構造体です。Open Matchとの互換性を保ちながら、マッチング条件や対象プレイヤープールを定義します。

## 定義

### Protocol Buffers定義

```protobuf
message MatchProfile {
  string name = 1;
  repeated Pool pools = 3;
  map<string, google.protobuf.Any> extensions = 5;
  reserved 2, 4;
}
```

(`api/openmatch/messages.proto:62`)

### Go構造体

```go
type MatchProfile struct {
    Name       string                // MatchProfileの識別名
    Pools      []*Pool               // マッチング対象のプールリスト
    Extensions map[string]*anypb.Any // 拡張用のカスタムデータ
}
```

(`gen/openmatch/messages.pb.go:542`)

## 構成要素

### 1. Name (string)

- MatchProfileを一意に識別する名前
- マッチング結果のMatchオブジェクトにもこの名前が記録される
- 例: `"simple-1vs1"`, `"ranked-match"`

### 2. Pools ([]\*Pool)

- マッチング対象となるチケットを分類するためのプールの配列
- 各Poolは、チケットをフィルタリングするための条件を持つ
- 複数のプールを定義することで、異なる条件のプレイヤーグループを作成可能

#### Poolの構造

```protobuf
message Pool {
  string name = 1;
  repeated DoubleRangeFilter double_range_filters = 2;
  repeated StringEqualsFilter string_equals_filters = 4;
  repeated TagPresentFilter tag_present_filters = 5;
  google.protobuf.Timestamp created_before = 6;
  google.protobuf.Timestamp created_after = 7;
}
```

各Poolは以下のフィルターを持つことができます：

- **DoubleRangeFilter**: 数値範囲でフィルタ（例: MMRが1000〜2000）
- **StringEqualsFilter**: 文字列の完全一致でフィルタ（例: region="asia"）
- **TagPresentFilter**: 特定のタグを持つチケットのみ抽出
- **created_before/created_after**: チケットの作成時刻でフィルタ

### 3. Extensions (map[string]\*anypb.Any)

- カスタムデータを格納するための拡張フィールド
- 独自のメタデータやパラメータを追加可能
- Protocol Buffersの`Any`型を使用して任意のデータ型を格納

## 使用例

### シンプルな1vs1マッチング

```go
matchProfile := &pb.MatchProfile{
    Name: "simple-1vs1",
    Pools: []*pb.Pool{
        {Name: "test-pool"},
    },
}
```

(`examples/simple1vs1/simple1vs1.go:14`)

このプロファイルは：

- 単一のプール`"test-pool"`を定義
- フィルター条件なし（すべてのチケットが対象）
- MatchFunctionがこのプールからチケットを取得してマッチングを実行

### 複数プールの例

```go
matchProfile := &pb.MatchProfile{
    Name: "ranked-match",
    Pools: []*pb.Pool{
        {
            Name: "beginners",
            DoubleRangeFilters: []*pb.DoubleRangeFilter{
                {DoubleArg: "mmr", Min: 0, Max: 1000},
            },
        },
        {
            Name: "advanced",
            DoubleRangeFilters: []*pb.DoubleRangeFilter{
                {DoubleArg: "mmr", Min: 1000, Max: 2000},
            },
        },
    },
}
```

## MatchProfileの登録とマッチング処理フロー

### 1. 登録

```go
mm, _ := minimatch.NewMiniMatchWithRedis()
mm.AddMatchFunction(matchProfile, minimatch.MatchFunctionSimple1vs1)
```

(`minimatch.go:58`)

- `AddMatchFunction`メソッドでMatchProfileとMatchFunctionをペアで登録
- 内部的には`map[*pb.MatchProfile]MatchFunction`として保持される

### 2. バックエンド処理

バックエンドは以下の流れでMatchProfileを使用します：

1. **チケット取得** (`backend.go:137-188`)
   - アクティブなチケットをRedisから取得

2. **プールフィルタリング** (`backend.go:304-321`)
   - `filterTickets`関数が各MatchProfileのPoolsに基づいてチケットを分類
   - 各Poolの条件に合致するチケットを抽出
   - `map[string][]*pb.Ticket`形式（プール名 -> チケットリスト）でMatchFunctionに渡される

3. **MatchFunction実行** (`backend.go:190-221`)
   - 各MatchProfileに紐づくMatchFunctionが並行実行される
   - MatchFunctionはプール別に分類されたチケットを受け取る
   - マッチング結果（Match）を生成

4. **評価と割り当て**
   - Evaluatorによる評価（オプション）
   - Assignerによるゲームサーバーの割り当て

## 実装における重要なポイント

### MatchProfileはポインタで管理

```go
mmfs map[*pb.MatchProfile]MatchFunction
```

(`minimatch.go:24`)

- MatchProfileはポインタをキーとしてmapで管理される
- 同じ内容でも異なるインスタンスは別のMatchProfileとして扱われる
- 登録時のポインタを使用する必要がある

### 並行処理

```go
for profile, mmf := range mmfs {
    profile := profile
    mmf := mmf
    eg.Go(func() error {
        poolTickets, err := filterTickets(profile, activeTickets)
        // ...
    })
}
```

(`backend.go:196-210`)

- 複数のMatchProfileが登録されている場合、errgroupを使用して並行実行
- 各MatchProfileは独立してマッチングを実行できる

## まとめ

MatchProfileは、minimatchにおけるマッチメイキングの「設計図」であり：

- **Name**: マッチングの識別名
- **Pools**: チケットをフィルタリングし分類する条件の集合
- **Extensions**: カスタムメタデータの格納領域

を提供します。これにより、柔軟で拡張可能なマッチメイキングシステムを構築できます。
