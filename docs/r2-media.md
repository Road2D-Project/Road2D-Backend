# Private media on Cloudflare R2

All objects use the single bucket in `R2_BUCKET_NAME`. Keep public access disabled:
neither a custom domain nor `R2_PUBLIC_BASE_URL` is required. The API signs uploads
for 15 minutes and viewing URLs for one hour. Refetch the profile or trip to refresh
an expired viewing URL. Anyone holding a viewing URL can read that object until
the link expires or the object is deleted.

## Configuration

Set `R2_ACCOUNT_ID`, `R2_BUCKET_NAME`, `R2_ACCESS_KEY_ID`, and
`R2_SECRET_ACCESS_KEY`. The existing names `R2_ACCESS_TOKEN` and
`R2_SECRET_ACCESS_TOKEN` also work as fallbacks. These must contain the R2 S3
Access Key ID and Secret Access Key, not a general Cloudflare bearer token.
The credentials need object read/write access to this bucket.

The default S3 endpoint is `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` with
region `auto`. Set `R2_ENDPOINT` for a jurisdiction-specific S3 endpoint.

Size limits are decimal bytes and can be overridden through the environment:

```dotenv
MEDIA_MAX_IMAGE_BYTES=5000000
MEDIA_MAX_VIDEO_BYTES=25000000
MEDIA_MAX_AUDIO_BYTES=5000000
```

Invalid or nonpositive configured limits fail validation rather than disabling it.
Images allow PNG, JPEG, and WebP. The reusable media service also allows MP4/WebM
video and MP3, M4A, AAC, Ogg, WebM, and WAV audio. Avatars and trip covers continue
to accept images only.

## Browser CORS

In the R2 bucket settings, configure CORS for your actual frontend origins. For
example, replace the origin in this policy before applying it:

```json
[
  {
    "AllowedOrigins": ["http://localhost:3000"],
    "AllowedMethods": ["PUT", "GET", "HEAD"],
    "AllowedHeaders": ["Content-Type", "Cache-Control", "Range"],
    "ExposeHeaders": ["ETag", "Content-Length", "Content-Range", "Accept-Ranges"],
    "MaxAgeSeconds": 3600
  }
]
```

Do not add application JWTs, Supabase API keys, or R2 credentials to requests sent
to the signed storage URL. Your usual application authentication is still needed
when requesting or confirming uploads through this backend.

## Avatar and trip-cover upload

1. Request `POST /v1/auth/user/avatar?contentType=image/png&sizeBytes=12345`,
   or `POST /v1/trips/<tripID>/image?contentType=image/png&sizeBytes=12345`.
   Use the exact file size. A missing, zero, or invalid size returns 400; a size
   above the configured limit returns 413.
2. The response contains `url`, `path`, `method`, `headers`, and `expiresAt`.
   PUT the raw file bytes to `url` using the supplied headers and exact file size.
   Do not use `FormData`. The URL signs the content type and content length.
3. Confirm with `POST /v1/auth/user/avatar/confirm`, or
   `POST /v1/trips/<tripID>/image/confirm`, and JSON `{"path":"<returned path>"}`.
4. The backend checks ownership, actual stored size, and Content-Type metadata.
   Invalid files are deleted and their keys are not saved. A successful response
   contains a temporary viewing `url`.

Example direct upload after obtaining `signed` from the backend:

```js
const headers = { ...signed.headers };
// Browsers supply this forbidden request header themselves from the file body.
for (const name of Object.keys(headers)) {
  if (name.toLowerCase() === "content-length") delete headers[name];
}
const uploaded = await fetch(signed.url, {
  method: signed.method,
  headers,
  body: file,
});
if (!uploaded.ok) throw new Error("Upload failed");
// Now confirm signed.path through the authenticated backend endpoint.
```

The actual R2 key is `<namespace>/<ownerID>/<uploadUUID>.<extension>`, with separate
`avatar` and `trip` prefixes. Each replacement uses a new key. The previous object
is deleted only after the new key is saved successfully.

Profiles and trip responses generate fresh viewing URLs. Graph responses are not
cached as serialized JSON because they contain signed URLs. Database columns
`users.avatar_url` and `trips.image_url` are retained but now store permanent keys;
their Go model fields are `AvatarPath` and `ImagePath`. Internal paths never appear
in profile/trip JSON. No schema or existing-image migration is performed.

## Future video and audio attachments

The shared service supports the agreed limits without accepting video as an avatar:

```go
media := objectStorage.NewMediaService("attachments", r2Client, objectStorage.MediaExt)
signed, err := media.RequestUpload(ctx, ownerID, "video/mp4", sizeBytes)
// Return signed to the uploader and wait for the upload to finish.
viewURL, err := media.Confirm(ctx, ownerID, signed.Path)
// Save signed.Path in the feature's attachment record only after Confirm succeeds.
// Generate later viewing links with r2Client.DownloadURL(ctx, "attachments", savedPath).
```

The future feature must authorize the owner and record relationship on every
request. This change does not introduce an attachment table, chat API, transcoding,
or video/audio routes before there is a feature to own those records.

Content-Type checking validates metadata, not the media bytes or codec. Direct
uploads can leave unconfirmed objects if a user abandons the flow. Per-file limits
are not total account quotas; retention and aggregate quotas can be added by the
owning feature.

References: [R2 Go SDK](https://developers.cloudflare.com/r2/examples/aws/aws-sdk-go/),
[presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/),
[CORS](https://developers.cloudflare.com/r2/buckets/cors/).
