# R14 MiniMax H3 successful provider result/archive

TYPE = OUR_EXTENSION / ADAPTER_WIRING
STATE = AUDITED_INTEGRATED
GPT_AUDIT = PASS
BASE_OUR_COMMIT = 965afe86d22d83b7cfa04f58e3a7e769d252ca87
BRANCH = feature/p0-b-r14-minimax-h3-result-archive
RESULT_ARCHIVE_ADAPTER_ENABLED_BY_DEFAULT = NO

ArchiveMiniMaxH3OfficialResult and RetryMiniMaxH3OfficialResultArchive are compiled explicit service boundaries with no production callers, route/UI or worker. Tests supply synthetic authenticated channel resolution and loopback TLS seams; no real resolver/account/key/API/CDN access is performed.

R13 read-only accepted-task gate/snapshot and package-private strict response parser are reused unchanged. Require frozen SUBMITTED/SUBMITTED Generation, one exact BOUND binding, safe persisted ProviderTaskID and exact metaso / minimax-official-global-v2 / MiniMax-H3 connection facts; validate frozen request hash and R12 T2V mapping. Current authenticated owner/channel resolution is required. Any persisted nonempty terminal ErrorClass rejects. UNKNOWN and unrelated SUBMITTING are rejected before Foundation.Open; no submission/poll fact mutation or resubmit.

A new attempt or explicit retry sends exactly one pinned Query Task GET and requires succeeded; id/model/type and any reported modality/resolution/duration/ratio match audited R13/R12 validation. The raw location is privately extracted only after strict parsing. Queued/running/failed/cancelled or failed query yields no media request or new metadata. The official [Query Task](https://platform.minimax.io/docs/api-reference/video-generation-v2-query) shape was rechecked; List Tasks is never invoked, no URL TTL inferred, 7-day history does not guarantee long-term recovery.

Media accepts HTTPS, DNS-shaped host and default/443 port only. Reject whitespace/control, userinfo/fragment, IP literal, local/internal names or excessive length. Reject locations containing the runtime API credential, including escaped forms. Production resolves all DNS answers, rejects nonpublic/special-use addresses, pins one approved IP without failover and retains the original TLS ServerName. No proxy, keepalive, HTTP2, compression, redirect following or automatic retry; fresh bounded transport with 30-second DNS/media deadline and 16 KiB headers. Credentials/cookies are not forwarded. Test-only private seams replace resolution/dial/trust to authenticate loopback while preserving validated public-looking URL and original media TLS hostname. Public APIs accept no network overrides.

Media response must be HTTP 200, exact video/mp4, known Content-Length in 1..64 MiB, no transfer/content encoding. A bounded prefix passes http.DetectContentType=video/mp4 before new metadata. The prefix is reconstructed with a length-limited body; Foundation consumes supplied bytes and verifies actual length/hash. This verifies MIME signature and archive integrity, not full codec validity or decode. Truncated streams after the prefix preserve one failed Result/job. All upstream/network/body errors are static safe errors before reaching Foundation or public output.

R14 Result binds exact GenerationID/TaskBindingID, ProviderResultID=ProviderTaskID, ResultKind=video, SourceURLRef empty, known frozen duration. No URL or alias is stored. Re-read Generation/binding and existing result/job immediately before metadata. Existing Foundation CreateResult/CreateArchive/RunArchive writes generated/<generationId>/<resultId>/media.mp4 with hash/byte/receipt authority. R5 ArchiveLocalResult remains a separate unchanged local-only path.

The existing per-process HN writer is held through bounded I/O and streaming, so a concurrent successful create sees the completed Result/job before requesting media. No multiprocess availability claim. Existing matching ARCHIVED facts return only after Open verifies bytes/receipt, with zero query/download. Existing failed/nonarchived job requires explicit retry by the same job ID, fresh strict succeeded query and transient media URL; no second Result/job. An exact Result without a job repairs the narrow metadata gap with one new job for that same Result. Duplicate/conflicting result/job ownership rejects. Corrupt final bytes or immutable receipts are not overwritten; explicit retry remains the same job but safe failure is retained if Foundation refuses publication. Recovery of such corrupted final artifacts is not claimed.

RAW_RESULT_URL_PUBLIC_RETURN / PERSISTENCE / LOGGING / COMPLETION = NONE. Generation and TaskBinding, including LastPolledAt/ErrorClass, remain unchanged. No Candidate, Generation, TaskBinding or submit is created by R14. The archived repeat can reopen in a separate process without query/media access; explicit failed-stream retry obtains a fresh location instead of depending on persisted URLs.

NEW_GENERATION_SOURCE_BASELINE remains 16047f46e2186373ea824e12e84ae8dfa2ccde32; historical Generation is unchanged. Foundation, R5/R10/R12/R13, R7/R8 independent fixes and R9 validation remain audited and byte-identical. Dependency/lockfile/Auth/upstream DB schema unchanged.

PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED. REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED = 0. REAL_CREDENTIAL_READ / REAL_REMOTE_MEDIA_DOWNLOAD = NONE. LIVE_VALIDATION_AUTHORIZATION = NOT_AUTHORIZED. Live Provider/CDN compatibility and account entitlement NOT_YET_VALIDATED; proxy-required environment NOT_VALIDATED; MEDIA_URL_TTL NOT_INFERRED; MULTIPROCESS_AVAILABILITY NOT_CLAIMED. Redirect/chunked/no-length/other MIME/>64 MiB live compatibility is deferred. Audit PASS; authorized closeout fast-forwards our-main and normally pushes only the audited R14 feature and governance-closed our-main. Await GPT Review of audit closeout Completion; no R15 or live validation.

## Audited integration closeout

R14_FEATURE_HEAD = 9299b497007c979d35fee436d9deb1ecf280b1ce
R14_COMPLETION_SHA256 = bcdb9092219a016b157c8a4a58f8eac4d00f00fc41439218be8e244840cc9f4e
REVIEWED_COMMITS = b6750dc45de29ef195ba015e65903953573651e1, 9299b497007c979d35fee436d9deb1ecf280b1ce
NEW_GENERATION_SOURCE_BASELINE = 16047f46e2186373ea824e12e84ae8dfa2ccde32

The two reviewed commits and protected implementation/test bytes remain unchanged. Actual stable closeout SHA is recorded only in Completion. Governance/progress closure does not authorize live Provider/API/CDN, credentials/tasks or R15.

```text
LIVE_PROVIDER_COMPATIBILITY = NOT_YET_VALIDATED
LIVE_CDN_COMPATIBILITY = NOT_YET_VALIDATED
MINIMAX_ACCOUNT_ENTITLEMENT = NOT_YET_VALIDATED
MEDIA_URL_TTL = NOT_INFERRED
PROVIDER_QUERY_HISTORY_WINDOW = 7_DAYS_DOCUMENTED
HN_LONG_TERM_RECOVERY_AFTER_PROVIDER_HISTORY_EXPIRY = NOT_CLAIMED
MULTIPROCESS_AVAILABILITY = NOT_CLAIMED
PROXY_REQUIRED_ENVIRONMENT = NOT_VALIDATED
FULL_CODEC_DECODE_VALIDATION = NOT_CLAIMED
CORRUPT_IMMUTABLE_FINAL_ARTIFACT_OVERWRITE = REJECTED
CORRUPT_IMMUTABLE_FINAL_ARTIFACT_RECOVERY = NOT_CLAIMED
```
