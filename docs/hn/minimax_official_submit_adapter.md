# R12 MiniMax H3 official submit adapter

TYPE = OUR_EXTENSION / ADAPTER_WIRING
STATE = IMPLEMENTED_PENDING_GPT_REVIEW
BASE_OUR_COMMIT = 16047f46e2186373ea824e12e84ae8dfa2ccde32
BRANCH = feature/p0-b-r12-minimax-h3-official-submit

HN prepare keeps Protocol=metaso. Only an explicitly selected local metaso channel with exact MiniMax-H3 model membership and an exact official global root freezes ProviderIdentity=minimax-official-global-v2. Root equivalence: HTTPS api.minimax.io, optional default :443/trailing slash, case/outer whitespace normalization; no userinfo, other host/port/path/query/fragment or URL-normalization tricks. Other providers retain their previous identity behavior; gateways never acquire official identity. No full config, BaseURL or credential is serialized.

R12 provider identity freeze review advances matching backend/frontend new-generation SourceBaseline constants and affected tests to 16047f46e2186373ea824e12e84ae8dfa2ccde32. Foundation's default and historical records are untouched. Existing old-record immutability assertions remain; an isolated old-baseline byte-equality regression is added. This is planned adapter wiring, not a verification fix or upstream backport.

SubmitMiniMaxH3OfficialGeneration has no production route/UI/default caller. A trusted caller supplies an authenticated owner and injected resolver. ResolveHNMiniMaxOfficialChannel reuses SelectUserLocalModelChannelForModel for that owner's exact ConnectionID and checks enabled channel, protocol/model/official root and nonempty header-safe runtime key. No admin/random/failover lookup, environment credentials or second store. Tests never invoke the real resolver.

Preflight opens under the existing HN writer, checks frozen PREPARED status/hash and deterministic request/channel validation before R10 Begin. Invalid requests create no binding/send no HTTP. The transport remaps the exact owned snapshot and independently re-resolves the current channel before sending; a post-preflight race can consume the attempt as UNKNOWN with zero POST. R10 ownership/acceptance/reopen semantics are unchanged; adapter never re-enters the HN writer.

Only T2V MiniMax-H3 with empty references. Exact PromptSnapshot -> one text content item. Canonical integer duration 4..15; no rounding/clamp/fallback. Finite vquality aliases 480/720/768 (optional p) -> 768P; 1080 (optional p)/2k/4k -> 2K, consistent with adopted normalization for these known values. Six concrete ratios and enumerated existing UI pixel presets -> official ratio; arbitrary pixels/adaptive reject. Unsupported controls accept only absent/empty/current defaults: std mode, false multishot, intelligence shot, true audio, false watermark, video orientation, empty negative/multiprompt. Nondefault requests reject. CredentialRef is empty or the same opaque ConnectionID.

The HTTP destination is internally pinned POST https://api.minimax.io/v2/video_generation; BaseURL is identity-only. Each attempt creates a dedicated fresh HTTP/1 Transport with Proxy=nil, no keepalive/HTTP2/SDK/retry/failover, one Do, one-shot GetBody=nil, no replay headers and redirect rejection. Total/header/TLS timeout=30s, headers <=16 KiB, body <=64 KiB; idle resources close. The private test dial/TLS seam connects solely to authenticated loopback fake TLS while retaining the official request URL/Host. Public entrypoint has no network override.

Acceptance requires HTTP 200 and exactly one JSON task_id string, safe under unchanged R10 rules, with no extra/duplicate/trailing fields. Bytes are preserved; ProviderTaskID only, no local alias. Stable errors contain no arbitrary key/provider/resolver/network text. Errors/panic/ambiguous response/acceptance-persistence failure flow to R10 UNKNOWN; no same-ID resend. Existing broad AI-call logger/upstream task DB/poller are not called.

Wire tests count actual server POSTs, including eight concurrent callers, capable HTTP2 peer still seeing HTTP1, 307/308 target count zero, 429/500/drop/timeout/cancel, malformed/unsafe/oversized task acceptance, channel race, persistence faults and process exit after local wire acceptance before local record. Smoke uses one project/Shot and two Generations, reopening BOUND/SUBMITTED and UNBOUND/UNKNOWN, one POST each, no Result/Archive/Candidate. Full regression and final scope/secret evidence are in Completion.

PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE=DEFERRED; ADAPTER_ENABLED_BY_DEFAULT=NO; PRODUCTION_LIVE_ENTRYPOINT=NONE; LIVE_VALIDATION_AUTHORIZATION=NOT_AUTHORIZED. REAL_PROVIDER_CALLS / PAID_CALLS / REAL_TASK_CREATED=0; REAL_CREDENTIAL_READ=NONE. No polling/result/download/archive, references/H3-Max, Auth/schema/dependency/lockfile/upstream adapter changes, merge/push. R7/R8 independent fixes, R9 audited validation and R10 production/tests remain unchanged. Any live gate requires later explicit authorization; next candidate is fake-only polling/recovery review.

Public contract rechecked against [MiniMax official H3 V2 Create](https://platform.minimax.io/docs/api-reference/video-generation-v2-create). No actual API request or live-capability claim.
