# Magic Item Ontology SQL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将已批准的 magic-item ontology 落成可重复 provision 的 Postgres schema、sqlc 类型和可验证的关系写入路径；本计划不包含 `cmd/extract`。

**Architecture:** 保留 `foundation.sql` 提供的 `object`、UUID 时间戳和 `touch()`，在其上按依赖顺序增加 vocabulary types、`magic_item`、`effect`、`limitation` 四个 schema 文件。所有跨对象关系用外键表达；Limitation 的可选 Effect 关系用复合外键 `(effect_id, item_id)` 保证 Effect 与 limitation 属于同一个 MagicItem。

**Tech Stack:** PostgreSQL 18, Go 1.26, pgx/v5, sqlc v1.31.1, Docker Compose。

## Global Constraints

- 只实现 `database/schema/`、`database/query/`、`database/sqlc.yaml`、`database/generated/` 和 ontology focused test；不得修改或设计 `cmd/extract`。
- 每个新 schema 文件首个有效内容前包含准确的 `-- requires:` 头，并按 `foundation, types, magic_item, effect, limitation` 依赖顺序 provision 和生成。
- 每个对象表 `inherits (object)`、声明 `primary key (id)`、包含有意义的 table/column/type comments，并创建 `touch` trigger。
- `name`、所有 source-grounded description 和 `raw_description` 必须拒绝空白文本；`MagicItem.name` 不设唯一约束。
- `usage_mode = worn` 必须有 `wear_slot`，其他 usage mode 必须没有；`requires_attunement = false` 时 `attunement_requirement` 必须为 null。
- `Limitation.effect_id` 为 null 表示 item-wide limitation；非 null 时数据库必须拒绝引用其他 MagicItem 的 Effect。
- 不增加设计文档已明确否决的 CreatureTarget、EnvironmentTarget、详细 effect kind、gold price 或 source-page 对象。

## File Map

- Create: `database/schema/types.sql` — rarity、source item type、usage mode、wear slot、effect category 的 closed enum vocabulary。
- Create: `database/schema/magic_item.sql` — catalog item object、来源字段、使用/穿戴和 attunement 一致性约束。
- Create: `database/schema/effect.sql` — item-owned effect object、单一 effect category 和 cascade relationship。
- Create: `database/schema/limitation.sql` — item-wide/effect-specific limitation、同 item integrity 的复合外键。
- Modify: `database/sqlc.yaml` — 以 foundation → types → magic_item → effect → limitation 的顺序列出 schema。
- Modify: `database/query/queries.sql` — named inserts and relationship reads used by generated Go and tests。
- Generate: `database/generated/` — only via `cd database && sqlc generate`，不得手写 generated Go。
- Create: `database/ontology_test.go` — reset/apply live schema, typed insert smoke test, and constraint/relationship assertions。

### Task 1: Add canonical ontology vocabulary

**Files:** Create `database/schema/types.sql`。

- [ ] **Step 1: Write the failing schema assertion**

在 `database/ontology_test.go` 的 schema smoke test 中先读取 `pg_type`，断言后续需要的五个 enum 存在；当前运行时应因 types.sql 尚不存在而失败。

Run: `go test ./database -run TestOntologyVocabulary -v`

Expected: FAIL because the enum types are not provisioned yet。

- [ ] **Step 2: Add the exact enums**

创建以下 PostgreSQL enum：`rarity` 值 `common, uncommon, rare, very_rare, legendary, artifact, varies`；`source_item_type` 值 `wondrous_item, weapon, armor, potion, ring`；`usage_mode` 值 `worn, held, portable, consumed`；`wear_slot` 值 `head, neck, torso, outerwear, hands, feet, finger`；`effect_category` 值 `offensive, defensive, utility`。为每个 type 添加说明其 canonical vocabulary 语义的 `comment on type`。

- [ ] **Step 3: Run the vocabulary assertion**

Run: `go test ./database -run TestOntologyVocabulary -v`

Expected: PASS。

- [ ] **Step 4: Commit the vocabulary**

```bash
git add database/schema/types.sql database/ontology_test.go
git commit -m "feat: add magic item ontology vocabulary"
```

### Task 2: Add MagicItem object and invariants

**Files:** Create `database/schema/magic_item.sql`。

- [ ] **Step 1: Add failing invariant cases**

在测试中准备合法 item，并分别断言以下 insert/update 抛出 PostgreSQL error：空白 `name`、空白 `raw_description`、worn 且无 slot、held 且有 slot、未 attuned 但有 requirement。测试还要断言合法 worn item、portable item 和有特殊 attunement requirement 的 item 能插入。

Run: `go test ./database -run 'TestMagicItem(Valid|Constraints)' -v`

Expected: FAIL because `magic_item` table does not exist。

- [ ] **Step 2: Create the object table**

`magic_item` 字段为：`name text not null`、`source_item_type source_item_type not null`、`source_item_subtype text`、`rarity rarity not null`、`usage_mode usage_mode not null`、`wear_slot wear_slot`、`requires_attunement boolean not null default false`、`attunement_requirement text`、`raw_description text not null`、`needs_review boolean not null default false`。加 `primary key (id)`、非空白 check、上述两组一致性 check、table/column comments、touch trigger；对 `source_item_subtype` 与 `attunement_requirement` 加 `btrim` 的非空约束（非 null 时）。

- [ ] **Step 3: Run MagicItem tests**

Run: `go test ./database -run 'TestMagicItem(Valid|Constraints)' -v`

Expected: PASS。

- [ ] **Step 4: Commit the object**

```bash
git add database/schema/magic_item.sql database/ontology_test.go
git commit -m "feat: add magic item object schema"
```

### Task 3: Add Effect object and typed relationship queries

**Files:** Create `database/schema/effect.sql`; Modify `database/query/queries.sql`。

- [ ] **Step 1: Add failing effect relationship cases**

测试先通过 `InsertMagicItem` 插入 item，再验证 `InsertEffect` 可插入 effect；空白 description 被拒绝；删除 item 后 effect 不存在。

Run: `go test ./database -run TestEffectRelationship -v`

Expected: FAIL because `effect` and generated query methods are absent。

- [ ] **Step 2: Create Effect**

创建 `effect`，字段 `item_id uuid not null references magic_item(id) on delete cascade`、`category effect_category not null`、`description text not null`，加 primary key、blank check、comments、touch trigger 和 `effect_item_idx`。

- [ ] **Step 3: Add named queries**

在 `database/query/queries.sql` 添加：

```sql
-- name: InsertMagicItem :one
insert into magic_item (name, source_item_type, source_item_subtype, rarity, usage_mode, wear_slot, requires_attunement, attunement_requirement, raw_description, needs_review)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) returning id;

-- name: InsertEffect :one
insert into effect (item_id, category, description) values ($1,$2,$3) returning id;

-- name: EffectsForMagicItem :many
select id, category, description from effect where item_id = $1 order by id;
```

- [ ] **Step 4: Generate and run tests**

Run: `cd database && sqlc generate`; then `go test ./database -run TestEffectRelationship -v`。

Expected: sqlc exits 0; test PASS。

- [ ] **Step 5: Commit Effect and generated/query output**

```bash
git add database/schema/effect.sql database/query/queries.sql database/generated
git commit -m "feat: add magic item effects and queries"
```

### Task 4: Add Limitation with same-item integrity

**Files:** Create `database/schema/limitation.sql`; Modify `database/query/queries.sql`。

- [ ] **Step 1: Add failing integrity cases**

测试创建两个 item、各自一个 effect；断言 item-wide limitation（null effect_id）成功；同 item effect limitation 成功；把 item A 的 limitation 指向 item B 的 effect 失败；删除 item A cascade 删除其 effects/limitations；空白 limitation description 失败。

Run: `go test ./database -run TestLimitationIntegrity -v`

Expected: FAIL until table, composite uniqueness, and foreign keys exist。

- [ ] **Step 2: Create Limitation**

`limitation` 字段为 `item_id uuid not null references magic_item(id) on delete cascade`、`effect_id uuid`、`description text not null`。在 `effect` 增加 `unique (id, item_id)`（或等价设计）后，在 limitation 上使用 `foreign key (effect_id, item_id) references effect(id, item_id) on delete cascade`；这使 effect_id 非 null 时自动要求两者属于同一 item，null 仍代表 whole-item limitation。加 primary key、blank check、comments、touch trigger、`limitation_item_idx` 和 `limitation_effect_idx`。

- [ ] **Step 3: Add limitation queries**

```sql
-- name: InsertLimitation :one
insert into limitation (item_id, effect_id, description) values ($1,$2,$3) returning id;

-- name: LimitationsForMagicItem :many
select id, effect_id, description from limitation where item_id = $1 order by id;
```

- [ ] **Step 4: Generate and run integrity tests**

Run: `cd database && sqlc generate`; then `go test ./database -run TestLimitationIntegrity -v`。

Expected: sqlc exits 0; same-item, cascade, null-effect and blank-text assertions PASS。

- [ ] **Step 5: Commit Limitation and generated/query output**

```bash
git add database/schema/limitation.sql database/query/queries.sql database/generated
git commit -m "feat: enforce magic item limitation relationships"
```

### Task 5: Provision and full verification

**Files:** Modify `database/sqlc.yaml`; Modify `database/ontology_test.go` only if prior test helpers need consolidation。

- [ ] **Step 1: Register dependency-ordered schemas**

把 `database/sqlc.yaml` schema 列表改为 `schema/foundation.sql`, `schema/types.sql`, `schema/magic_item.sql`, `schema/effect.sql`, `schema/limitation.sql`。

- [ ] **Step 2: Generate from database directory**

Run: `cd database && sqlc generate`。

Expected: exit 0 and generated models/queries include all four ontology objects and named methods。

- [ ] **Step 3: Verify local Postgres readiness**

Run: `go run ./cmd/verify`。

Expected: `postgres is ready`; if Docker/Postgres is unavailable, record that prerequisite separately rather than treating it as a schema failure。

- [ ] **Step 4: Run focused and repository tests**

Run: `go test ./database -v` and `go test ./...`。

Expected: ontology tests and existing tests pass; failures must be classified with their actual output。

- [ ] **Step 5: Inspect scope and commit configuration**

Run: `git diff --check`; `git status --short`; `git diff --stat`。

Expected: only ontology SQL/query/generated/test/config files are changed or newly tracked; `.agents/` and `skills-lock.json` remain untouched and unstaged。

```bash
git add database/sqlc.yaml database/schema database/query database/generated database/ontology_test.go
git commit -m "feat: provision magic item ontology SQL"
```

## Self-review checklist

- Spec coverage: vocabulary, four object files, comments, inheritance, primary keys, touch triggers, foreign keys/indexes, all consistency checks, same-item composite relationship, named typed queries, sqlc generation, live provision, valid/invalid inserts, cascades, and null effect limitation are covered by Tasks 1–5.
- Placeholder scan: no `TBD`, `TODO`, “implement later”, or unspecified validation step appears in the plan.
- Type consistency: query parameters will be inferred from the enum/domain columns; generated methods are `InsertMagicItem`, `InsertEffect`, `EffectsForMagicItem`, `InsertLimitation`, and `LimitationsForMagicItem`, matching the test plan.
- Scope guard: no task creates, edits, or enters `cmd/extract`; PDF audit metadata remains outside the business ontology.
- Execution gate: after this plan is approved, use `superpowers:executing-plans` task-by-task with fresh verification at each checkpoint.
