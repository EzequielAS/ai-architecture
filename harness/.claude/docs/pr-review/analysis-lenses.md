# Analysis Lenses

Each lens is applied file by file, against the file's **current content** (not just
the diff). For each finding, record: path, line (HEAD version), comment type, and
a short, actionable message.

---

**Lens A — Architecture.** Look for an architecture document in the `docs/` folder of
the repo under review (e.g. `docs/architecture.md`, `docs/software-architecture.md`,
`docs/adr/*.md`). Read it in full and check every rule/decision against the changed
code — layer violations, forbidden imports, broken contracts, decisions that
contradict an ADR. If no document exists, skip this lens and record "no architecture
doc".

**Lens B — Spec adherence** (only when a related spec was found). For each P1 user
story in the spec: are the acceptance criteria (WHEN/THEN/SHALL) covered? If the PR
is partial, point out what's covered and what's pending. Don't penalize incomplete
P2/P3 stories in a partial implementation.

**Lens C — General quality signals.**
- Is the error handled at the right layer (service, not scattered)? Are known errors
  mapped to an appropriate code, and unexpected errors kept from leaking internal
  detail?
- Is input validated and sanitized before reaching business logic?
- Are there obvious edge cases the code ignores?
- Is there unnecessary complexity for what the change actually needs to do?
- Does the change come with tests proportional to the risk?
