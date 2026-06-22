# Files API

## 1. Ownership

Files owns the contracts for immutable file collections, immutable file metadata, local FS blob writing, uploading, downloading, and deleting.

```text
Path prefix: /api/files
Module: internal/files
Common rules: ../API.md
```

---

## 2. Current Status

`Implemented`. The current version implements the minimum closed loop of `Collection -> File`; tag projections reuse `ObjectRef`:

```text
After creation, a Collection cannot be updated, it only allows appending Files or deletion.
After upload, a File cannot be updated, it only allows individual deletion or deletion along with the Collection.
Both Collection and File are registered as FIL-* ObjectRefs; object_type distinguishes between file_collection and file.
Tags are written to the corresponding `object_refs.tags` for the Collection or File.
```

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/files/collections` | Bearer JWT | `Implemented` | List collections readable by the current actor |
| `POST` | `/api/files/collections` | Bearer JWT | `Implemented` | Create a collection |
| `GET` | `/api/files/collections/{ref_code}` | Bearer JWT | `Implemented` | Read collection metadata |
| `DELETE` | `/api/files/collections/{ref_code}` | Bearer JWT | `Implemented` | Delete a collection, cascading file deletions via the unified File delete process |
| `GET` | `/api/files?collection_ref_code=<FIL>&tag=<name>` | Bearer JWT | `Implemented` | List files, filterable by collection or tag |
| `POST` | `/api/files/collections/{ref_code}/files` | Bearer JWT | `Implemented` | Multipart upload a file to a collection |
| `GET` | `/api/files/{ref_code}` | Bearer JWT | `Implemented` | Read file metadata |
| `GET` | `/api/files/objects/{ref_code}/download` | Bearer JWT | `Implemented` | Download file after blob validation |
| `DELETE` | `/api/files/{ref_code}` | Bearer JWT | `Implemented` | Delete an individual file |

---

## 4. Create Collection

```http
POST /api/files/collections
Content-Type: application/json
```

```json
{
  "name": "Receipts",
  "description": "Tax year 2026",
  "tags": ["tax", "receipt"]
}
```

Field rules:

| Field | Required | Rule |
| --- | --- | --- |
| `name` | Yes | Must not be empty after trimming |
| `description` | No | Saved after trimming; defaults to an empty string |
| `tags` | No | Trimmed, empty values removed, and deduplicated before associating with the Collection; defaults to an empty array |

Successfully returns `201 Created`:

```json
{
  "collection": {
    "ref_code": "FIL-00000001",
    "name": "Receipts",
    "description": "Tax year 2026",
    "status": "active",
    "tags": ["tax", "receipt"],
    "created_at": "2026-05-30T00:00:00Z",
    "updated_at": "2026-05-30T00:00:00Z"
  }
}
```

## 5. Upload File

```http
POST /api/files/collections/FIL-00000001/files
Content-Type: multipart/form-data
```

Form fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `file` | yes | Uploaded file content; filename comes from the multipart file header |
| `tags` | no | Comma-separated tags, e.g., `tax,receipt`; trimmed, empty values removed, and deduplicated before associating with the File |

The server saves the blob to:

```text
./objects/{FILE_REFCODE}/blob
```

For example:

```text
./objects/FIL-00000002/blob
```

During the upload write, the server calculates `sha256` and `blake3`, and writes them to the file metadata. Successfully returns `201 Created`:

```json
{
  "file": {
    "ref_code": "FIL-00000002",
    "collection_ref_code": "FIL-00000001",
    "original_name": "receipt.pdf",
    "mime_type": "application/pdf",
    "size_bytes": 12345,
    "sha256": "64 lowercase hex chars",
    "blake3": "64 lowercase hex chars",
    "status": "active",
    "tags": ["tax", "receipt"],
    "metadata": {
      "original_name": "receipt.pdf",
      "mime_type": "application/pdf",
      "size_bytes": 12345,
      "sha256": "64 lowercase hex chars",
      "blake3": "64 lowercase hex chars"
    },
    "created_at": "2026-05-30T00:00:00Z",
    "updated_at": "2026-05-30T00:00:00Z"
  }
}
```

## 6. List Query

```text
collections: limit, offset
files:       collection_ref_code, tag, limit, offset
```

| Parameter | Rule |
| --- | --- |
| `collection_ref_code` | Optional Collection `FIL-*` reference code filter |
| `tag` | Optional; exact filter by associated tag name after trimming for File |
| `limit` | Defaults to `25`, range `1..100` |
| `offset` | Defaults to `0`, must be a non-negative integer |

List endpoints reject undefined query parameters. Collections are sorted by `created_at DESC, ref_code DESC`; Files are sorted by `created_at DESC, ref_code DESC`.

## 7. Download File

```http
GET /api/files/objects/FIL-00000002/download
```

Before downloading, the server must reread the local blob and validate:

```text
size_bytes
sha256
blake3
```

Transmission to the client only begins after all match. The response headers include:

```text
Content-Type
Content-Length
Content-Disposition
X-Content-SHA256
X-Content-BLAKE3
```

When integrity checks pass and the download is prepared, a visible system audit log record must be written:

```text
action = EXPORT
result = SUCCESS
reason = download
target_ref_code = file_ref_code
```

If any check fails, it returns:

```http
409 Conflict
```

```json
{
  "error": {
    "code": "integrity_check_failed",
    "message": "File integrity check failed"
  }
}
```

If any check fails, the download must be rejected, and a visible system audit log record must be written:

```text
action = EXPORT
result = FAILED
reason = integrity_check_failed
target_ref_code = file_ref_code
```

## 8. Delete Semantics

Individual file delete:

```text
DELETE /api/files/{file_ref_code}
audit.reason = direct_file_delete
```

Delete collection:

```text
DELETE /api/files/collections/{collection_ref_code}
```

The server must first list all files under the collection, and invoke the unified File delete module one by one; the audit reason for each file delete is:

```text
cascade_collection_delete
```

The audit reason for the collection's own deletion is:

```text
collection_delete
```

After creation, Collection and File metadata or tags cannot be modified; if you need to adjust a File tag, you must delete and re-upload it.

## 9. Errors

| HTTP | code | Meaning |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid request parameters, ref_code, or multipart body |
| `401` | `unauthorized` | Unauthenticated |
| `404` | `not_found` | Resource does not exist or is inaccessible to the current actor |
| `409` | `integrity_check_failed` | Blob integrity check failed prior to download |
| `500` | `files_unavailable` | Files service is unavailable |

Single resource non-existence and inaccessibility uniformly return `404 / not_found`.
