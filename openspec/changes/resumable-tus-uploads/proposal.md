# Proposal

## Why
`Upload` and `UploadFile` send a file in one presigned `PUT`, so a dropped connection late in a large video means sending the whole file again. The `media-upload` service speaks tus 1.0.0, which lets the client resume from the server's offset.

## What Changes
- Add `Client.UploadResumable` and `Client.UploadFileResumable`, an opt-in tus client: create with `Upload-Length` and `Upload-Metadata`, read the allocated `fileId` from `HEAD`, then send chunks with `PATCH`.
- Retry network errors and 423/429/500/502/503/504 with jittered backoff, resync from `HEAD` on 409 or an implausible offset, fail fast on other 4xx.
- Add `ResumeStore` (in-memory by default) keyed by fingerprint, so uploads resume across calls and, with a durable store, across processes.
- Add `Client.TerminateUpload`, options `WithChunkSize`, `WithRetries`, `WithProgress`, `WithResumeStore`, `WithFingerprint`, the client option `WithUploadEndpoint`, and `*TusError`.
- Leave the presigned `Upload` path unchanged.

## Capabilities

### New Capabilities
- `resumable-upload`: tus-based resumable uploads to media-upload.

### Modified Capabilities
None.

## Impact
- Public API: new methods, options and types in `sdk`. No breaking changes.
- Default endpoint `https://uploads.rixl.com/media/v1/uploads/`, which still needs an ingress in media-upload.
- New example `examples/advanced/resumable`.
