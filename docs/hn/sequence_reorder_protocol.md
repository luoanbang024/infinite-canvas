# R30 local Sequence reorder protocol

STATE = PENDING_GPT_AUDIT
TYPE = OUR_EXTENSION / LOCAL_ONLY / METADATA_PROTOCOL / NO_UI

This protocol is server metadata authority plus typed transport only. No Canvas action, browser reorder journal/controller, drag/drop, compound or export is wired.

A monotonic main Sequence revision prevents stale overwrite; a separately durable command identity/receipt attributes a lost response. Neither substitutes for the other. All upgraded main sequence writers reserve the actual SQLite writer with BEGIN IMMEDIATE. Mutation, revision and applicable receipt commit together.

## Routes

| Method | main Sequence suffix | Contract |
|---|---|---|
| POST | reorder-protocol/initialize | closed protocolVersion=1; existing store only; initial revision 0 at current state, not reconstructed history |
| GET | reorder-snapshot | complete <=256 nine-field items + canonical string revision; pinned readonly transaction |
| POST | reorder-commands | UUIDv4 intent, canonical decimal string expectedRevision, unique complete desiredSequenceItemIds |
| GET | reorder-commands/:intent | exact immutable terminal receipt or provisional NOT_OBSERVED |

Boundary runs before sequence validation: loopback peer/Host/Origin, X-HN-Local-Request marker, no Authorization/Cookie/query, GET empty body, closed JSON POST <=64KiB; no-store and static errors. Read operations use strict readonly OpenExisting, never initialize, reconcile, create dirs, access media or credentials. Absent protocol is REORDER_PROTOCOL_NOT_INITIALIZED, never fake revision 0 or NOT_OBSERVED.

## Command authority

K is project/main/protocol1/UUIDv4. Canonical P is JSON object in fixed key order: expectedRevision then desiredSequenceItemIds. Stored bytes and SHA both checked. Existing K/P is resolved before current state checks, so replay after later mutation returns original historical receipt and never restores old order. K reused with different P is REORDER_INTENT_CONFLICT.

New command with old expectedRevision records CONFLICT/SEQUENCE_REVISION_CONFLICT. Current revision with incomplete/foreign set records CONFLICT/SEQUENCE_SET_CONFLICT. Both have appliedRevision null and sequence delta 0. Accepted new command, including no-op, records COMMITTED at observed revision +1. Command rows cannot update/delete/replace. Partial schema or corrupt canonical/receipt/state fails closed; no silent repair, TTL or reconstruction.

On response loss, exact lookup can recover a terminal receipt; it does not prove its order remains current. NOT_OBSERVED is only the pinned snapshot and cannot rule out a still-running POST. Future caller must retain exact intent/payload; explicit same-key continuation is a mutation with one-effect semantics. Automatic POST retry = 0. No browser implementation exists in R30.

## Writer / compatibility policy

Legacy Add remains fresh/non-idempotent. First R28 placement insert, actual legacy Add/Delete and accepted legacy/new reorder advance main revision in the same write transaction. R28 replay/reject/conflict/rollback advances zero. Non-main mutations do not advance main. Overflow before effect is static SEQUENCE_REVISION_EXHAUSTED.

Legacy Reorder keeps sequenceItemIds request and safe nine-field ordered-array response. Full-set checks/only-order updates/revision/returned facts are captured under SQLite write reservation. It remains unconditional and NOT CAS SAFE; new clients do not fall back to it.

SequenceItem JSON and indexed fields other than orderIndex/order_index stay exact, including UpdatedAt, CreatedAt, unknown future safe JSON fields and owner IDs. Other business entities and frozen SourceBaseline do not change. R28 five-key /items and placement commands/receipts, existing v1/v2 controller/journal/UI remain.

## Guarantee and verification

Real separate OS processes share one disposable SQLite file and rendezvous after open. Covered races: initialization, same K/P, different intents/same revision with same or different orders, CAS vs legacy Add, CAS vs new placement, legacy reorder vs Add, delete vs CAS, and concurrent append chains with monotonic consistent readonly revision/vector snapshots. Guarantees exclude mixed old binaries, raw unauthorized DB writes, distributed copies and browser-wide orchestration.

Completion carries full root Go, module verify, focused Foundation/service/handler/router, R7/R28 and R20/R22/R24 preservation, frontend tests, independent tsc, isolated source-bound production build, protected bytes and exact-member secret scan. Initial failed test invocation/framework/corruption-observer checks are preserved with corrected final results.

API Key/account/channel/live remain deferred. Provider/paid/live calls zero; real credentials/env/media reads none; no dependency installation or update. Wait for independent GPT Audit; no push/merge or next stage.
