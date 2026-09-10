## Code style

- Functions: 4-20 lines. Split if longer.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Exception messages must include the offending value and expected shape.

## Comments

- Only comment what is absolutely necessary
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Reference issue numbers / commit SHAs when a line exists because
  of a specific bug or upstream constraint.

## Tests

- Every new function gets a test. Bug fixes get a regression test.
- Mock external I/O (API, DB, filesystem) with named fake classes,
  not inline stubs.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.

## Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

## Structure

- Prefer small focused modules over god files.
- Predictable paths: controller/model/view, src/lib/test, etc.
- Always uses path alias if you could. If don't have any kind, suggest a pattern.

## Logging

- Structured JSON when logging for debugging / observability.
- Plain text only for user-facing CLI output.

## Required check before finishing a task

Before considering a task done (and before any commit), run this:

```bash
# Sensors (code scanners, lint, typecheck, tests, build)
npm run quality
```
