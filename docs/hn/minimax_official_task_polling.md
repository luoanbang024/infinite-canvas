# R13 MiniMax H3 known-task polling/recovery

TYPE = OUR_EXTENSION / ADAPTER_WIRING
STATE = IMPLEMENTED_PENDING_GPT_REVIEW
BASE_OUR_COMMIT = 6e8ca04b7bf20241a90a0c1b602b3bea1a23adac
BRANCH = feature/p0-b-r13-minimax-h3-task-polling

PollMiniMaxH3OfficialTask is explicit and has no scheduler, route, UI, startup worker or live caller. Caller supplies an authenticated owner and the existing R12 exact channel resolver boundary; tests inject synthetic channels only. No actual credential/config lookup occurs during verification.

Preflight requires safe project/binding/owner IDs; an existing, frozen SUBMITTED/SUBMITTED Generation; one exact BOUND binding with a safe persisted ProviderTaskID; matching project/schema/GenerationID/connection/protocol/provider identity; metaso / minimax-official-global-v2 / MiniMax-H3. Existing FreezeGeneration validates an already frozen hash and snapshot/reference integrity without rewriting it. R12's unchanged request mapper yields the expected resolution/duration/ratio.

Foundation.Open already reconciles SUBMITTING. A read-only SQLite URI gate checks the existing project store through Foundation.Resolve before Open, under hnReferenceWriter. Nonaccepted targets and projects with any SUBMITTING record reject without opening/reconciling/creating a workspace. No Foundation code/schema/behavior changed; accepted tasks continue through normal Workspace APIs. This conservative busy rejection does not offer cross-process availability. Workspace ownership must remain trusted, as in existing Foundation.

Network runs outside the writer. Destination is pinned HTTPS api.minimax.io /v2/query/video_generation/{persisted_task_id}; safe ID plus PathEscape, never channel BaseURL. Fresh dedicated transport: Proxy=nil, keepalive/HTTP2 off, 30s total/header/TLS timeout, 16 KiB headers, 64 KiB body, redirect rejection, one Do and no retry/failover/SDK. Private TCP/TLS test options connect solely to authenticated loopback while preserving official Host/path. Public entrypoint has no destination override.

Bounded JSON requires an object containing task; duplicate keys and trailing JSON rejected at root/task/content. Exact id, MiniMax-H3 model, generation task_type and five-case status are mandatory. Optional reported modality must be video; resolution/duration/ratio, when present, must equal frozen mapped request facts. Null/type/mismatches reject; absent optional facts are not invented. Extra provider metadata is ignored and never copied into observations/storage.

| State | Terminal | Result location available | Safe class |
|---|---|---|---|
| queued | false | false | empty |
| running | false | false | empty |
| succeeded | true | true | empty |
| failed | true | false | MINIMAX_TASK_FAILED |
| cancelled | true | false | MINIMAX_TASK_CANCELLED |

Succeeded requires nonempty absolute HTTP(S) content.url with host and no userinfo/whitespace; syntax establishes presence only. Raw location/body/provider error/key stays in private runtime locals and is discarded. There is no downloader, trust decision or result/archive record. Public HNProviderTaskObservation contains binding/task IDs, State, Terminal, ResultLocationAvailable, PolledAt and SafeFailureClass only.

After validated 200 response, the writer is reacquired and current frozen Generation and binding ownership are checked again. Only existing TaskBinding LastPolledAt and static terminal ErrorClass are updated through UpdateTaskBinding; its UpdatedAt advances normally. Generation remains byte-identical. Any existing nonempty ErrorClass must equal the incoming terminal class; stale queued/running/succeeded or failed/cancelled replacement returns POLL_STATE_CONFLICT with no durable mutation. Same-class terminal polls remain explicit and valid. A failed durable update returns controlled state-conflict; no submit/retry/UNKNOWN transition.

| Poll error | Classification | Durable effect |
|---|---|---|
| invalid state/ownership/hash/channel/input | preflight | GET 0, no update |
| 401/403 | auth | no update |
| 404 | not found/unresolvable | no update |
| 429/5xx/timeout/drop/cancel | transient polling failure | no update |
| redirect/other HTTP/malformed/oversized/unknown/mismatched facts | protocol | no update |
| stale terminal conflict/changed ownership/persistence failure | state conflict | no poll update |

SUBMISSION_UNKNOWN is never polled or recovered by list/search. Polling failure leaves known acceptance SUBMITTED / BOUND; caller may explicitly query again, which is a GET, never a submit retry. No generic upstream VideoTask DB/poller, task-status column, Foundation lease or second credential store.

The public [Query Task](https://platform.minimax.io/docs/api-reference/video-generation-v2-query) and [List Tasks](https://platform.minimax.io/docs/api-reference/video-generation-v2-list) contract was rechecked. PROVIDER_QUERY_HISTORY_WINDOW = 7_DAYS_DOCUMENTED; HN_LONG_TERM_RECOVERY_AFTER_PROVIDER_HISTORY_EXPIRY = NOT_CLAIMED. No media URL TTL is inferred. The list endpoint is documentation reference only and is never invoked.

R7/R8 original independent verification fixes, R9 validation, Foundation, R10 submission guard, R12 submit/identity files and tests remain unchanged. GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE; new preparation still binds 16047f46e2186373ea824e12e84ae8dfa2ccde32, not this governance baseline.

PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0; REAL_CREDENTIAL_READ = NONE; LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED. POLL_ADAPTER_ENABLED_BY_DEFAULT = NO. Live provider compatibility/account entitlement, proxy-required environment and multiprocess availability remain unvalidated/not claimed. Await GPT Review / Audit of Completion; no merge/push or R14 work.
