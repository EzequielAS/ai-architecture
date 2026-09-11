# Comment Types

Use only the types that genuinely apply. No comment type is mandatory.

| Type | Emoji | When to use |
|------|-------|-------------|
| **Critical** | 🚨 | Architecture violations, forbidden cross-layer imports, bugs that will cause runtime errors, build failures, security issues. Must be fixed before merge. |
| **Warning** | ⚠️ | Rule violations that reduce quality but don't break functionality. Lint errors. Should be fixed. |
| **Opportunity** | 💡 | Refactoring suggestions, simpler approaches, patterns that would improve readability. Optional. |

Inline comment format:

```
🚨 **Critical** — <short title>

<Explanation of the problem and why it matters.>

<If applicable: concrete suggestion or corrected snippet.>
```

Replace the emoji and type label to match the comment type being used.
