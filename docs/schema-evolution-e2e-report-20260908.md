# Schema Evolution E2E Report - 2026-09-08

## Scope

- Center: `192.168.10.103`, `server-001`, MySQL 8.0.38 and RabbitMQ in Docker.
- Edge: `192.168.10.105`, `edge-001`, MySQL 8.4 and NodeBridge v0.46.4.
- Transport: SSH stdio MCP for rules, process control, schema inspection, health and queue checks; SSH/MySQL for isolated DDL and DML.
- Tables: `scada_edge.nb_schema_e2e`, `scada_edge.nb_schema_crud_e2e` and matching center tables.
- Production tables were not changed. Test rules were removed after verification; test tables remain as read-only evidence.

## Product Boundary

NodeBridge does not replicate or execute MySQL DDL. Canal DDL entries are intentionally skipped. NodeBridge only normalizes and transfers INSERT, UPDATE and DELETE row events.

This boundary prevents a remote edge from executing arbitrary `CREATE TABLE`, `ALTER TABLE` or `DROP TABLE` on the center. Schema migrations must therefore be deployed separately and in a compatible order. A DML event that cannot be applied remains unacknowledged and requeued until the target schema or rule is corrected.

## Results

| Case | Expected | Actual | Result |
| --- | --- | --- | --- |
| MCP handshake and inventory | Both nodes expose management tools | MCP 2025-11-25 initialized; 27 edge tools available | PASS |
| Create source table while running, target absent | No automatic target DDL; DML retained | Target table stayed absent; ingress queue held 1 message; dead queue stayed 0 | PASS |
| Create missing target table | Queued DML resumes | Row `910001` recovered in 2.54 s; apply log written once | PASS |
| Add nullable column on both sides | New value transfers | Row `910002` and `added-ok` arrived in 19.36 s | PASS |
| Add source-only column without a rule exclusion | Apply must fail without partial write | Target row absent; ingress held 1; dead queue stayed 0 | PASS |
| Add source-only column to `exclude_columns` through MCP | Existing queued event resumes without target column | Row `910003` recovered; excluded value was not written | PASS |
| Rename a source column with `column_mappings` | Source value maps to existing target name | `renamed_source -> source_name` transferred correctly | PASS |
| Drop source column, retain nullable target column | Later DML omits the column safely | Row `910005` arrived; retained target column was NULL | PASS |
| Add target-only NOT NULL column without a default | Apply must block | Row `910006` stayed absent and ingress held 1 | PASS |
| Relax target-only column to nullable | Queued event resumes | Row `910006` recovered and queue returned to 0 | PASS |
| Drop a mapped target column | Apply must block instead of losing data | Row `910007` stayed absent and ingress held 1 | PASS |
| Restore the mapped target column | Queued event resumes with mapped value | Row `910007` recovered in 9.8 s | PASS |
| Narrow target type on append-only path before fix | Data error must block | `value-too-long` was silently truncated to `value`, ACKed and logged as success | FAIL, FB-045 |
| Narrow target type after FB-045 fix | No row/apply log until type is widened | Row/apply log stayed 0 and ingress held 1; after widening, full `fixed-long-value` recovered in 5.5 s | PASS |
| CRUD insert, update and delete | Ordered DML and soft delete work | Insert/update values matched; deletes set `is_deleted=1`, `deleted_at` and `deleted_by_node=edge-001` | PASS |
| CRUD add column and update it | Added value transfers on INSERT and UPDATE | `runtime-v1` and `runtime-v2` arrived | PASS |
| CRUD source drops a column while target retains it | Other updates continue and target value remains | Payload advanced to v3; retained `runtime-v2` was unchanged | PASS |
| CRUD target drops a source column | Update blocks, then resumes after target repair | Target stayed at version 3 with ingress 1; restored column applied version 4 | PASS |
| CRUD source column rename and mapping | Updates use the mapped target column | `renamed_payload -> payload` applied version 5 | PASS |
| Drop and recreate source table with the same name | Target is retained and new row events resume | Existing target row remained; row `920003` arrived from recreated table | PASS |
| Rename source primary key and map it | INSERT/UPDATE/DELETE use mapped target key | `source_id -> id` passed for all three operations on row `920004` | PASS |
| One backlog spans old and new column sets | Both row shapes apply after target migration | Rows `910010` and `910011` arrived; pre-DDL value was NULL and post-DDL value was preserved | PASS |

Final evidence: append-only target rows `12`, append-only apply logs `12`, CRUD target rows `4`, CRUD apply logs `12`, soft-deleted CRUD rows `2`, failed ACK rows `0`. All server queues ended at ready `0` and unacknowledged `0`; both MCP overviews reported Agent, CDC, MySQL and RabbitMQ as `running`.

After restoring the original single `nb_e2e_probe` rule, final probe `1788840146648 / post-schema-1788840146648` reached the center in 17.18 s and all queues returned to zero.

## Defect Fixed

`internal/apply/worker.go` used `INSERT IGNORE` for append-only batches. MySQL therefore converted truncation and some constraint failures into warnings, while NodeBridge committed the apply log and ACKed the message.

The batch SQL now uses plain `INSERT`. Delivery replay idempotency remains enforced by `sync_apply_log.event_id`. A distinct event that violates a business key, width, type or nullability constraint now blocks and remains requeued instead of being silently changed. Unit coverage verifies that a batch data error rolls back and does not produce an applied result. The real narrow-column E2E verified complete recovery after the target column was widened.

## Production Migration Order

1. Create table: create and validate the center target first, deploy the rule to both nodes, restart/reload agents, then enable edge writes.
2. Add column: add a nullable or defaulted target column first, then add the source column. Update mappings or exclusions before source applications write the field.
3. Rename column: deploy a compatible target column and `column_mappings` first, reload both rule sets, then rename the source column.
4. Drop column: exclude or stop producing the source field first, wait for queues to reach zero, drop it on the source, then drop it on the target later.
5. Change type: widen the target before widening the source. Never narrow the target while wider source values are possible.
6. Add NOT NULL: add it nullable or with a safe default, backfill target rows, update producers, then enforce NOT NULL.
7. Change primary key: update target key/indexes and NodeBridge key mappings as one controlled migration; pause writes during the incompatible interval.
8. Drop/recreate table: treat it as a new migration. Target data is not deleted by source DDL, and reused business keys may conflict with retained rows.

## Remaining Coverage

This run covers the core row-schema compatibility matrix. Separate release tests are still required for multi-edge concurrent migrations, generated/invisible columns, partition DDL, foreign keys, unique indexes other than the primary key, collation/charset conversion, ENUM/DECIMAL precision changes, very large online ALTER operations and Server-to-Edge schema rollouts.

The existing v0.46.4 installer was built before FB-045 and must not be represented as containing this fix. The verified fixed center binary is the current working-tree `build/bin/SyncAgent.exe`; a new versioned installer is required before external distribution.
