# Migrate object storage from MinIO to Fly.io Tigris (S3-compatible)

This guide shows how to switch your app from MinIO to Tigris using your existing MinIO-compatible code (minio-go v7). No Dockerfile or code changes are required in most cases.

Your Tigris credentials (from the "Create bucket" screen):
- AWS_ENDPOINT_URL_S3: https://fly.storage.tigris.dev
- AWS_ACCESS_KEY_ID: <keep secret>
- AWS_SECRET_ACCESS_KEY: <keep secret>
- AWS_REGION: auto
- BUCKET_NAME: delicate-lake-6581

Do NOT commit credentials to git. Use Fly secrets.

---

## 1) Map Tigris values to your app env (MINIO_*)

Your app expects these envs (already supported by the code):
- MINIO_ENDPOINT: host only (no scheme). Use the host from AWS_ENDPOINT_URL_S3.
  - From https://fly.storage.tigris.dev → MINIO_ENDPOINT = fly.storage.tigris.dev
- MINIO_ACCESS_KEY: use AWS_ACCESS_KEY_ID
- MINIO_SECRET_KEY: use AWS_SECRET_ACCESS_KEY
- MINIO_BUCKET: use BUCKET_NAME (delicate-lake-6581)
- MINIO_USE_SSL: true (Tigris endpoint is HTTPS)

Optional (only if you hit signature/region errors later):
- MINIO_REGION: auto (requires a tiny code patch to pass Region into minio.Options; not needed for most setups)

---

## 2) Set Fly.io secrets (safe, recommended)

Run on your machine (replace placeholders with your real values; do NOT commit them):

```bash
fly secrets set \
  MINIO_ENDPOINT="fly.storage.tigris.dev" \
  MINIO_ACCESS_KEY="<AWS_ACCESS_KEY_ID>" \
  MINIO_SECRET_KEY="<AWS_SECRET_ACCESS_KEY>" \
  MINIO_BUCKET="delicate-lake-6581" \
  MINIO_USE_SSL="true"
```

Verify they’re in place:

```bash
fly secrets list
```

You can update these any time with `fly secrets set ...` again. No need to touch Dockerfile.

---

## 3) Configure CORS on the Tigris bucket (for browser uploads)

If your client uploads directly using the presigned PUT URL, allow PUT/GET/HEAD from your frontend origin in the Tigris UI (or via s3api). As a starting point while testing:

```json
[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "PUT", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 300
  }
]
```

Then tighten AllowedOrigins to your real domain in production.

---

## 4) Deploy and verify

```bash
# Deploy your app
fly deploy

# Watch logs
fly logs
```

Look for a line like:
```
MinIO service initialized successfully
```

This means the minio-go client connected to Tigris with your new secrets.

---

## 5) Test end-to-end

1) Init upload (same API as before):
```bash
curl -X POST https://<your-app>.fly.dev/api/v1/files \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "filename":"test.txt",
    "size":1024,
    "mime_type":"text/plain"
  }'
```
The response should include an `upload_url` (presigned PUT).

2) Upload the file using the presigned URL:
```bash
curl -X PUT "<upload_url>" \
  -H "Content-Type: text/plain" \
  --data-binary @test.txt
```
Expect 200/204 from Tigris.

3) (Optional) Presigned GET if your app exposes it:
Use your existing GET presign method to fetch the object and verify.

---

## 6) Troubleshooting

- Signature/region errors:
  - Tigris often works without explicitly setting Region. If you see "Signature mismatch"/"Invalid region", introduce `MINIO_REGION=auto` and (if needed) extend the constructor in `internal/storage/minio_repo.go` to pass `Region: "auto"` into `minio.Options`. Ask us and we’ll provide a tiny patch.

- Connection errors:
  - Ensure `MINIO_ENDPOINT` is the host only: `fly.storage.tigris.dev` (no `https://`).
  - Ensure `MINIO_USE_SSL=true`.

- CORS/Browser PUT blocked:
  - Make sure PUT/GET/HEAD are allowed, and origins are configured correctly in Tigris’ CORS settings.

---

## 7) What you do NOT need to change

- Dockerfile: no change required.
- App code: no change in typical cases. Your minio-go client is S3-compatible.

---

## 8) Safety notes

- Never commit AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY to git.
- Manage secrets with `fly secrets set` only.
- Rotate keys periodically in Tigris.

---

## Quick reference (copy/paste)

```bash
# Map your Tigris values to app envs
# - AWS_ENDPOINT_URL_S3: https://fly.storage.tigris.dev
# - BUCKET_NAME: delicate-lake-6581

fly secrets set \
  MINIO_ENDPOINT="fly.storage.tigris.dev" \
  MINIO_ACCESS_KEY="<AWS_ACCESS_KEY_ID>" \
  MINIO_SECRET_KEY="<AWS_SECRET_ACCESS_KEY>" \
  MINIO_BUCKET="delicate-lake-6581" \
  MINIO_USE_SSL="true"

fly deploy
fly logs
```

