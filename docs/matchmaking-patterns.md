# 実践的なマッチングロジック実装ガイド

## 1. 現状分析

**Simple1vs1の実装内容:**

- プールごとのチケットを2つずつペアリング
- 待機順序（FIFO）のみを考慮
- スキルレベル、地域、レイテンシなどの属性を考慮しない

**MatchProfileのフィルタリング機構:**

- `DoubleRangeFilter`: 数値範囲条件（スキルレベル、ELO、レイテンシなど）
- `StringEqualsFilter`: 文字列マッチ（地域、言語、ゲームモードなど）
- `TagPresentFilter`: タグの存在確認（VIP、プレイ中、新規プレイヤーなど）
- 時間フィルタ: `CreatedBefore/CreatedAfter`

---

## 2. 実装時の重要な考慮点

### 2.1 マッチング品質と待機時間のトレードオフ

**スキルマッチング** → 完全なスキル一致を待つと待機時間が長くなる

- 解決策: 時間経過に応じてマッチング条件を緩和（段階的マッチング）

**待機時間の最小化** → クイックマッチは品質が低下

- 解決策: クライアント側で許容可能な待機時間を指定

### 2.2 公平性の確保

**長時間待機中のユーザーの優先化**

- 古いチケットを優先的にマッチングさせる仕組みが必要

**スキルレベルの分布**

- 低スキルプレイヤーが多い場合、高スキルプレイヤーが見つからない問題

### 2.3 スケーラビリティ

- 数千のチケットから効率的にマッチ可能なペアを見つける必要
- 複雑な計算は CPU 負荷が高くなる

---

## 3. 具体的な実装パターン

### パターン1: **スキルレベルマッチング（ゲーム系）**

**想定シーン:** FPS、MOBA、RTS などスキル差が大きい影響を与えるゲーム

```go
// 例: ELOレーティングベースのマッチング
type SkillMatchFunction struct {
    maxSkillGap float64  // 許容スキルの差
    maxWaitTime time.Duration
}

// 実装ポイント:
// 1. チケットをスキルスコア(DoubleArgs["elo"])でソート
// 2. スキル差がmaxSkillGap以内のペアを作成
// 3. 待機時間が長いほど許容スキルギャップを拡大
// 4. マッチできないチケットはバックフィル候補に

// プール設計:
// - InitialPool: elo 1000-1200, maxWait 10秒 → strict matching
// - EscalatedPool: elo 900-1300, maxWait 30秒 → relaxed matching
// - CasualPool: elo 800-2000, maxWait 60秒 → any skill level
```

### パターン2: **地域別・低レイテンシマッチング（モバイル・マルチプレイ）**

**想定シーン:** 対戦型モバイルゲーム、リアルタイムアクション

```go
// 例: 地域とレイテンシベースのマッチング
type GeoLatencyMatchFunction struct {
    preferredRegions map[string][]string  // プレイヤー地域 → マッチ可能地域
    maxLatencyMs     int
}

// 実装ポイント:
// 1. チケットの StringArgs["region"] で地理的に近いプレイヤーをグループ化
// 2. DoubleArgs["latency_ms"] がmaxLatencyMs以内のペアを優先
// 3. 同一地域内でペアリング、時間経過後に隣接地域と拡張
// 4. CrossRegionMatches フラグでマッチ結果にレイテンシ情報を付与

// プール設計:
// - Region: Asia/Japan, maxLatency: 30ms
// - Region: Asia (expanded), maxLatency: 50ms
// - Global: Any region, maxLatency: 100ms
```

### パターン3: **ロールベースのマッチング（チーム戦）**

**想定シーン:** MOBA、MMO レイドなどチーム構成が重要なゲーム

```go
// 例: チーム組成（Tank, DPS, Healer）
type RoleBasedMatchFunction struct {
    teamSize int
    roleRequirements map[string]int  // "tank" -> 1, "dps" -> 2, "healer" -> 1
}

// 実装ポイント:
// 1. チケットの Tags に role を含める（"role:tank", "role:dps"）
// 2. teamSize 人数を集めるまで待機
// 3. roleRequirements を満たすチーム構成を検出
// 4. 不完全なチーム（タンク不足など）は合成されない

// プール設計:
// - StrictRoles: role:tank, role:dps, role:healer すべてが必須
// - FlexRoles: role:dps が多い場合、flexibility:true なチケットで補完
```

### パターン4: **ランク帯別マッチメイキング（段階的拡張）**

**想定シーン:** 競技系ゲーム、レート戦

```go
// 例: 段階的なマッチング拡張
type RankedMatchFunction struct {
    tiers []TierBracket
    baseWaitTime time.Duration
}

type TierBracket struct {
    skillMin float64
    skillMax float64
    waitThreshold time.Duration  // この時間待ったら次の層へ
}

// 実装ポイント:
// 1. 最初のマッチング: 同じランク帯内のみ
// 2. baseWaitTime 経過後: ±1 ランク帯に拡張
// 3. 2x baseWaitTime 経過後: さらに拡張
// 4. マッチ時に実際のスキル差を Extensions に記録

// プール設計:
// - Tier1 (Bronze): elo 0-999, wait 10s → elo 500-1500
// - Tier2 (Silver): elo 1000-1999, wait 10s → elo 500-2500
// - Tier3 (Gold): elo 2000-2999, wait 10s → elo 1500-3500
```

### パターン5: **リセマッチング・チームシャッフル（継続セッション）**

**想定シーン:** 複数マッチの継続プレイ、キューレート系ゲーム

```go
// 例: 同じプレイヤーとの連続マッチを避ける
type ShuffleMatchFunction struct {
    maxConsecutiveMatches int
    excludeRecentPlayers  bool
}

// 実装ポイント:
// 1. PersistentField で前マッチのプレイヤーIDを記録
// 2. Backfill: 前マッチのチケットを再利用する場合、相手を新規に探す
// 3. 同一メンバーでのリマッチを避ける配下を使用

// プール設計:
// - FreshMatch: recentOpponents が空のチケットのみ
// - ShuffledMatch: 異なるプレイヤーを保証
```

### パターン6: **パーティーマッチング＆ボーナス点（グループプレイ）**

**想定シーン:** 協力型ゲーム、ソシャルゲーム

```go
// 例: パーティー単位でのマッチング
type PartyMatchFunction struct {
    maxPartySize int
    partyBonusMultiplier float64
}

// 実装ポイント:
// 1. PersistentField["party_id"] で同じパーティーのチケットをグループ化
// 2. partySize 人数でスキル計算を調整
// 3. Extensions に party bonus（例: 報酬1.5倍）を記録
// 4. Party full → 確定マッチ、Party incomplete → 他のプレイヤー募集可

// プール設計:
// - PartyFull: party_id が same で人数が maxPartySize に達したチケット
// - PartyRecruiting: party_id が same で人数 < maxPartySize
// - Solo: party_id が空のチケット
```

---

## 4. 実装の進め方

### ステップ1: マッチング戦略の決定

- ゲームジャンルと KPI を定義（マッチ品質スコア、待機時間、成功率）
- プールの層構造を設計

### ステップ2: MatchFunction の実装

```go
func NewCustomMatchFunction(config CustomConfig) entity.MatchFunction {
    return entity.MatchFunctionFunc(func(ctx context.Context, profile *entity.MatchProfile, poolTickets map[string]entity.Tickets) (entity.Matches, error) {
        // 1. バリデーション & サニタイズ
        // 2. ソート & グループ化
        // 3. マッチロジック実行
        // 4. Extensions に詳細情報を記録
        // 5. Backfill 候補を設定

        var matches []*entity.Match
        // ... マッチング処理
        return matches, nil
    })
}
```

### ステップ3: 段階的な待機時間の実装

- createdAt から経過時間を計算: `time.Now().Sub(ticket.CreatedAt)`
- 経過時間に基づいてマッチング条件を緩和する関数を用意

### ステップ4: Extensions の活用

```go
// Extensions に以下を記録:
// - matchQualityScore: 0-100 でマッチの品質スコア
// - skillGap: スキル差
// - regionLatency: レイテンシ実測値
// - waitTimeMs: 実際の待機時間
// - relaxationFactor: マッチング条件の緩和度（0.0-1.0）
```

### ステップ5: モニタリング

- 平均待機時間
- マッチ成功率（マッチ後に実際にゲーム開始した割合）
- スキル差分布
- リテンション への影響

---

## 5. パフォーマンス最適化のヒント

| 最適化方法       | 対象                                  | 効果            |
| ---------------- | ------------------------------------- | --------------- |
| インデックング   | ソート不要な場合、前もってグループ化  | O(n²) → O(n)    |
| キャッシング     | TicketRepository の検索結果キャッシュ | メモリ ↑、CPU ↓ |
| 並列処理         | pool ごとに並列マッチング             | マルチコア活用  |
| 段階的リラックス | 最初は厳しい条件、時間経過で緩和      | 平均待機時間 ↓  |
| バッチ処理       | 複数プール を同時処理                 | 遅延 ↓          |

---

## 6. 各パターンの適用マトリックス

| パターン       | スキル格差 | 地理的要件 | チーム構成 | 待機短縮 | 複雑度 |
| -------------- | ---------- | ---------- | ---------- | -------- | ------ |
| Simple1vs1     | ✗          | ✗          | ✗          | ◎        | 低     |
| Skill Matching | ◎          | ✗          | ✗          | ◎        | 中     |
| Geo + Latency  | ◎          | ◎          | ✗          | ◎        | 中     |
| Role-Based     | ✗          | ✗          | ◎          | ◎        | 中     |
| Ranked Tier    | ◎          | ✗          | ✗          | ◎        | 中     |
| Shuffle        | ✗          | ✗          | ✗          | ◎        | 低     |
| Party Matching | ◎          | ✗          | ◎          | ◎        | 高     |

---

## 7. 実装時の注意点

1. **タイムアウト処理**: 無限待機を避ける → Backfill 候補を早期に生成
2. **デッドロック**: マッチできないチケットの群を避ける → 定期的なチケット破棄
3. **公平性**: 特定プレイヤーが常にマッチするとは限らない → ログと分析が必須
4. **A/B テスト**: 複数戦略を並行検証 → MatchProfile を複数定義して実験
5. **ロールバック**: 品質低下時に戻す → マッチング履歴を記録
