Bug-fix issue. References existing FAL-error handling in `fal.go` (no specific AC -- this is error-presentation polish on top of the generic generation flow).

## Bug

When the FAL API returns a non-200 response, pix surfaces the entire raw response body as the error:

```go
return nil, "", fmt.Errorf("FAL API error (HTTP %d): %s", resp.StatusCode, string(body))
```

For `edit` endpoints (and any endpoint with `image_urls`), the response body from FAL includes the request payload -- which contains the full base64-encoded reference image. A user editing a 200 KB JPEG sees ~270 KB of base64 text scroll past their terminal followed by the actual API error message, far beyond scrollback. The diagnostic message is unrecoverable.

Reproduction (Tadg, 2026-05-16): invoking pix with a kontext-style edit model + a reference image triggers some API-level rejection. Terminal output is dominated by base64; the actual `error.message` never appears in viewport. Cannot debug.

## Solution

The FAL `/v1/models` OpenAPI doc (already cited in conversation) shows the standard FAL error envelope:

```json
{
  "error": {
    "type": "validation_error|authorization_error|not_found|rate_limited|server_error|not_implemented",
    "message": "Human-readable error message",
    "docs_url": "https://...",
    "request_id": "..."
  }
}
```

In `fal.go::generateImageWithPayload`, when `resp.StatusCode != http.StatusOK`:

1. Attempt to parse the response body as the standard FAL error envelope.
2. If the envelope parses and `error.message` is non-empty: surface `"FAL API error (HTTP <status>): <type>: <message>"`, optionally appending `(request_id: <id>)` when present.
3. If parsing fails OR `error.message` is empty: fall back to surfacing a truncated raw body (first 500 bytes, with a "... (truncated)" suffix when over the limit). The raw body might still be useful for non-envelope errors (HTML 502s from upstream proxies, etc.) but truncating prevents the base64-payload pathology.

The truncation cap is a small constant in `fal.go` -- 500 bytes is enough to read a short JSON error or the start of an HTML error page, and short enough to fit in any terminal viewport.

## Acceptance Criteria

| ID | AC | Tests |
|---|---|---|
| AC17.1 | When the FAL API returns a non-200 response with a body that parses as the FAL standard error envelope, pix's error message contains the value of `error.message` from that envelope. | ⏳ RT-17.1: FAL stub returns 400 with `{"error":{"type":"validation_error","message":"prompt too long"}}` -- pix stderr contains `prompt too long`<br>⏳ RT-17.2: FAL stub returns 401 with the envelope -- pix stderr contains the envelope's `message` |
| AC17.2 | When the FAL error envelope includes `error.type`, pix's error message includes that classifier. | ⏳ RT-17.3: envelope with `type: "validation_error"` -- pix stderr contains `validation_error` |
| AC17.3 | When the FAL error envelope includes `error.request_id`, pix's error message includes that ID so support requests can reference it. | ⏳ RT-17.4: envelope with `request_id: "req-abc"` -- pix stderr contains `req-abc` |
| AC17.4 | When the FAL response body does not parse as the FAL error envelope (HTML page, plaintext, malformed JSON), pix's error message includes the first 500 bytes of the body with a truncation indicator if the body exceeds that length. | ⏳ RT-17.5: FAL stub returns 502 with a 10KB HTML error page -- pix stderr contains the first 500 bytes followed by `... (truncated)`; pix stderr does NOT contain bytes past offset 500<br>⏳ RT-17.6: FAL stub returns 400 with a short (under 500 bytes) plaintext body -- pix stderr contains the full body, no truncation indicator |
| AC17.5 | The request body (and any embedded `image_urls` base64 payloads) is never written to stderr on FAL error paths. | ⏳ RT-17.7: FAL stub echoes the request body back as the error body. pix is invoked with a base64-encoded reference image. Stderr does NOT contain `data:image/png;base64,` or the JPEG/PNG header byte sequence as a base64 substring |

**Key:** ✅ passing · ⏳ pending · ❌ failing · ~~🚫 removed~~

## Out of scope

- Pretty-printing or colouring the error. Plain stderr text remains the convention.
- Retrying on rate-limit (`429`) automatically. Surfacing the message is enough; retry policy is a separate decision.
- Translating FAL `error.type` codes into pix-specific suggestions ("did you mean...?"). The raw `type: message` is already the diagnostic the user needs.
