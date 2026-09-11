# Output Format

The final review is a single JSON at `$WORK/review.json`. Each finding becomes an
inline comment; the same emoji and label from `comment-types.md` go in the `body`.

```json
{
  "event": "REQUEST_CHANGES",
  "summary": "## 📋 Summary — PR #<number>: <title>\n\n<overall summary, without naming a file or line>\n\n### Points of attention\n- ...\n\n### Spec coverage\n<only when a spec was found; otherwise omit this section>",
  "inline_comments": [
    {
      "path": "src/client/users-api.ts",
      "line": 42,
      "body": "🚨 **Critical** — Short title\n\nExplanation..."
    }
  ]
}
```

`event`: `"REQUEST_CHANGES"` when there is at least one 🚨 **Critical** (including a
build or test failure); `"COMMENT"` otherwise.

`summary`: write for a cold reader — what was reviewed, the overall quality signal, the
result of the gates (lint/build/test), and the relevant patterns. Don't repeat the
inline findings. Max ~250 words.

`inline_comments[].path`: path relative to the repo root, without `./` and without a
leading slash — the format each provider requires is assembled at post time.
