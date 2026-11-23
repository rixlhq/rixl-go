## ADDED Requirements

### Requirement: Chunked tus upload
`UploadResumable` SHALL create a tus upload with the project ID, kind and file name in `Upload-Metadata`, read the allocated `fileId` from `HEAD`, and send the body in `PATCH` chunks of the configured size (default 50 MiB), carrying the client's auth headers.

#### Scenario: Clean upload
- **WHEN** a caller uploads a file and the server accepts every chunk
- **THEN** the call returns the file ID, upload URL and size, and the resume entry is deleted

### Requirement: Resume after interruption
The client SHALL resume from the server-reported offset instead of resending bytes the server already holds.

#### Scenario: Connection drops mid-chunk
- **WHEN** a `PATCH` fails with a network error or 423/429/5xx
- **THEN** the client backs off, reads the offset with `HEAD`, and continues from it

#### Scenario: Offset mismatch
- **WHEN** a `PATCH` returns 409 or an implausible offset
- **THEN** the client resyncs with `HEAD` without waiting

#### Scenario: Non-retryable rejection
- **WHEN** the server returns another 4xx such as 403
- **THEN** the call fails with a `*TusError` and does not retry

#### Scenario: Retry limit reached
- **WHEN** retryable errors exceed the configured consecutive retry limit
- **THEN** the call fails

### Requirement: Resume across calls
Upload URLs SHALL be kept in a `ResumeStore` by fingerprint until the upload succeeds.

#### Scenario: Cancelled then retried
- **WHEN** the context is cancelled mid-upload and the caller uploads the same file again
- **THEN** the client continues the stored upload without a new `POST`

#### Scenario: Stale stored upload
- **WHEN** a stored upload URL returns 404 or 410
- **THEN** the entry is dropped and a new upload is created
