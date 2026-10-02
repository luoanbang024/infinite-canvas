# R10 provider-neutral submission guard

TYPE = OUR_EXTENSION / RELIABILITY_WIRING
STATE = AUDITED / INTEGRATED
BASE_OUR_COMMIT = f4c0cd1176bf9f48a49f57281065f224e3288276
BRANCH = feature/p0-b-r10-submission-guard

A frozen PREPARED Generation can acquire durable ownership once. BeginSubmission returns its exact frozen request and newly created UNBOUND TaskBinding, committing both binding creation and SUBMITTING state in one SQLite transaction under the Workspace writer lock. Existing schema/version 1, synchronous=FULL and records are reused. An existing binding or non-PREPARED state rejects before transport.

RecordSubmissionAccepted validates the same Generation, unique binding ownership, connection/protocol/provider facts and at least one opaque task ID; it transactionally persists BOUND and SUBMITTED. Identical persisted acceptance is idempotent; replacement IDs and late acceptance after UNKNOWN are rejected. Task IDs use the safe-name/credential-literal and recognized credential-prefix checks, not URLs or secret-bearing response bodies. Future provider identifier compatibility still requires review.

The internal service SubmitFrozenGeneration requires an injected SubmissionTransport, has no default implementation or production HTTP route, and never resolves CredentialRef. It validates local input/cancellation before Begin, then passes only that transaction's frozen Generation snapshot to one transport call. It never reads Canvas configuration. Error, panic, empty/unsafe acceptance or accepted-record failure returns explicit manual-reconciliation-required ErrHNSubmissionUnknown; arbitrary transport error/panic text is not persisted/returned. If unknown-state persistence fails, durable SUBMITTING remains blocked and the next Open reconciles it.

Open validates existing frozen records and reconciles persisted SUBMITTING to SUBMISSION_UNKNOWN; the UNBOUND binding remains historical evidence. No transport is invoked on reopen. SUBMITTED and UNKNOWN cannot Begin again; user retry requires a newly prepared/frozen Generation ID, possibly for the same Shot. Existing MarkSubmissionUnknown remains a no-send/no-resend recorder; no reset/retry operation was added.

Reuse of the existing global HN writer serializes Open/Begin/transport/acceptance/Close for same-process services. Injected transport must not re-enter HN writer operations. A separately opened process can conservatively classify another process's live SUBMITTING as UNKNOWN; this sacrifices acceptance availability while retaining at-most-once safety. This is not a multi-process availability/lease protocol. No background retry, polling or provider recovery is implemented.

Automated proof includes atomic rollback, invalid/frozen/hash/project/state/owner preconditions, accepted ownership/ID immutability, 16 competing Foundation calls, two independent handles, two service callers, two separate processes, subprocess exits before send and after fake acceptance before local record, and reopen invariance. Synthetic R6 smoke has one Shot and three Generations: SUBMITTED/BOUND, UNKNOWN/UNBOUND, UNKNOWN/UNBOUND; no Result/ArchiveJob.

PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED; real Provider/paid calls = 0; real credential read = NONE. No Result, ArchiveJob, remote download, frontend submit UI, router change or dependency/lockfile/schema change. R6 preparation SourceBaseline remains ff32dc249811130a3db69be456e295be100b6e9f; all request-defining/frozen facts and historical Generations unchanged. R7/R8 original verification fixes and R9 audited validation record remain unchanged. GPT Audit PASS; original feature normal push and fast-forward integration authorized, with one governance-only closeout commit. Stable SHA and final remote equality recorded in closeout Completion. Wait that Completion Review before choosing any next decision gate.

## Audited integration closeout

GPT_AUDIT = PASS
R10_FEATURE_HEAD = 444a5d7a7c7e8a8fe26953c9f1e9e34a587ebce6
R10_COMPLETION_SHA256 = 84e571250114418bef21088110c83e4434428c15e6cac3dbf5912ba2736ab242
R10_IMPLEMENTATION_STATE = AUDITED / INTEGRATED
AT_MOST_ONCE_SUBMISSION_GUARD = AUDITED
BEGIN_SUBMISSION = ATOMIC
TASKBINDING_OWNER = ONE_PER_GENERATION_ATTEMPT
SUBMITTING_REOPEN = SUBMISSION_UNKNOWN
SAME_GENERATION_AUTO_RESEND = NONE
NEW_RETRY_REQUIRES_NEW_GENERATION_ID = YES
MULTIPROCESS_AVAILABILITY = NOT_CLAIMED
TASK_RECOVERY = DEFERRED
REAL_PROVIDER_IDENTIFIER_COMPATIBILITY = DEFERRED
REAL_TRANSPORT_RETRY_POLICY_REVIEW = REQUIRED_BEFORE_PROVIDER_BINDING
PRIMARY_VIDEO_PROVIDER / ACCOUNT_MODE = DEFERRED
REAL_PROVIDER_CALLS / PAID_CALLS / REAL_CREDENTIAL_READ = NONE
PRODUCTION_SUBMIT_ROUTE / POLLING / RESULT_WIRING / ARCHIVEJOB_WIRING / REMOTE_DOWNLOAD = NONE
GENERATION_SOURCE_BASELINE_CHANGE / HISTORICAL_GENERATION_REWRITE = NONE

Four audited commits and all implementation/test bytes preserved. The existing broad MarkSubmissionUnknown recorder remains unchanged; future integration must call it only for genuinely ambiguous outcomes. No manual task recovery, leases, Provider-specific ID mapping or transport retry policy is supplied by this integration. R7/R8 remain independent original verification fixes and R9 remains an unchanged audited validation.
