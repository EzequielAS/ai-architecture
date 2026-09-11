---
description: Reviews a PR against spec, architecture doc, and project gates, and posts the findings as inline comments and a summary on the remote repository
argument-hint: <PR_NUMBER>
allowed-tools: Read, Glob, Grep, Bash(.claude/bin/pr-review/*), Bash(mkdir:*), Bash(printf:*), Bash(cat:*), Bash(tee:*), Bash(node:*), Bash(npm run:*), Bash(pnpm:*), Bash(yarn:*), Bash(git remote:*), Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git rev-parse:*), Bash(git fetch:*), Bash(gh:*), Bash(glab:*), Bash(az:*)
---

# PR Review

Reviews PR `$1` by combining three signal sources — the **binary's static analysis**, the
**related spec**, and the **architecture document** — and posts the result to the
remote repository.

**Invocation:** `/pr-review <PR_NUMBER>`

---

## Conventions

| Placeholder | Value |
|---|---|
| `$GATE` | `.claude/bin/pr-review/quality-gate-linux` on Linux, `.claude/bin/pr-review/quality-gate-darwin` on macOS (resolve using the OS reported by the environment) |
| `$WORK` | a temporary directory for this run, e.g. `<scratchpad>/pr-review-<PR_NUMBER>` |
| `$PR` | the PR number passed as argument |

If `$GATE` comes without the execute bit after a clone: `chmod +x .claude/bin/pr-review/*`.

Everything accumulates in `$WORK/review.md`. Interpretation happens **once**, at the
moment each output is produced — Step 7 only assembles the JSON, it never re-analyzes. The
file has two kinds of block:

- **`## Status:`** — result of the project's gates, e.g.:
  `## Status: lint PASS · build PASS · test FAIL`.
- **`## Finding:`** — the interpreted ledger. Append each finding the moment you
  confirm it, so nothing is lost to a context limit. Format:

  ```markdown
  ## Finding: <path>:<line> — <emoji> **<label>**
  - type: critical | warning | opportunity
  - source: lint | build | test | file-size | duplication | effects | lens:<a|b|c>

  <body — the final text of the inline comment; paste the relevant error output here>
  ```

  A finding with no concrete line (build/test failure) uses `line: —` and goes into
  the summary, not an inline comment.

---

## Step 1 — Detect the provider and check the tooling

```bash
git remote get-url origin
```

From the URL, pick the provider and confirm its CLI is installed **and**
authenticated. If either is missing, **stop** and instruct the user to install or
authenticate — do not try to work around it.

| Origin contains | Provider | CLI | Check |
|---|---|---|---|
| `dev.azure.com` / `.visualstudio.com` | Azure DevOps | `az` + `azure-devops` extension | `az account show` and `az extension list --query "[?name=='azure-devops']" -o tsv` |
| `github.com` | GitHub | `gh` | `gh auth status` |
| `gitlab` | GitLab | `glab` | `glab auth status` |

On Azure DevOps, derive the organization from the origin
(`.../v3/<ORG>/<PROJECT>/<REPO>` → `https://dev.azure.com/<ORG>`) and pass
`--organization <ORG_URL>` on **every** `az` command. Store this value as `$ORG`.

## Step 2 — Fetch the PR data

Get: `title`, source branch, target branch, author, and the **list of changed
files**.

**Azure DevOps**

```bash
az repos pr show --id $PR --organization $ORG -o json
```

From the JSON, keep `title`, `sourceRefName`, `targetRefName`, `createdBy.uniqueName`,
`repository.id`, and `repository.project.id` (the last two are needed in Step 8).
For the diff, use the merge commits:

```bash
SRC=$(az repos pr show --id $PR --organization $ORG --query lastMergeSourceCommit.commitId -o tsv)
TGT=$(az repos pr show --id $PR --organization $ORG --query lastMergeTargetCommit.commitId -o tsv)
git fetch -q origin "$SRC" "$TGT"
git diff --name-only "$TGT".."$SRC"
git diff "$TGT".."$SRC"
```

**GitHub**

```bash
gh pr view $PR --json title,headRefName,baseRefName,author,files
gh pr diff $PR
```

**GitLab**

```bash
glab mr view $PR --output json
glab mr diff $PR
```

## Step 3 — Read the changed files

Read the **current, full** content of every changed file with the `Read` tool. The diff
alone doesn't give enough context to judge architecture decisions, naming, or logic.

## Step 4 — Gather the spec and architecture document

1. **Related spec** — search with `Glob`: `specs/**/*.md`, `.specs/**/*.md`,
   `**/*.spec.md`. Match it against the change by changed-file names and by the PR's
   `title`/branch; confirm with `Grep`/`Read` when the match is weak. Read the matching
   spec in full. If none matches, record "no related spec".
2. **Architecture document** — search the repo's `docs/` folder: `docs/*.md` with an
   architecture-like name (`arquitetura`, `architecture`, `design`) and `docs/adr/*.md`.
   Read whatever exists in full. If none exists, record "no architecture doc".

## Step 5 — Run the project's gates

Read `package.json` and find the **lint**, **build**, and **test** scripts the
project actually defines (names vary: `lint`, `build`, `test`, `test:coverage`,
`typecheck`, or an aggregate like `ci`). Use the package manager declared in
`packageManager`/lockfile. Run each one that exists; skip with `SKIP` whatever doesn't
exist.

Record a `## Status:` line with the result of each gate and a `## Finding:` per
failure, classifying by the `.claude/docs/pr-review/comment-types.md` reference:

- **Lint FAIL** → ⚠️ **Warning** on the first reported file/line.
- **Build FAIL** → 🚨 **Critical** with `line: —`. Before recording, confirm the failure
  comes from the PR's code and not from missing local config (`.env`, `*.local`, `.npmrc`).
- **Test FAIL** → 🚨 **Critical** with `line: —`.

Paste the error output directly into the finding's body.

## Step 6 — Run the static analysis

The binary runs three checks on `.ts`/`.tsx`/`.js`/`.jsx`: `file-size`, `duplication`, and
`effects`. It **exits with code 1 when it finds violations** — chain `|| true` so the
step doesn't abort.

Run it over the **source root** (e.g. `src`), not just the changed files: the
duplication check needs the whole corpus to find the twin in existing code.
Then **filter** the output, keeping only violations that touch a file changed in the PR.

```bash
$GATE --ignore '**/*.test.*,**/*.d.ts' src 2>&1 | tee "$WORK/gate.txt" || true
```

Adjust the thresholds to the project when it makes sense: `--file-size N` (default 500),
`--dup-tokens N` (default 50), `--dup-lines N` (default 5), `--ignore GLOB`,
`--check NAME` / `--skip NAME` (`file-size`, `duplication`, `effects`). `$GATE --help`
lists everything.

**If the project has no React** (no `react` in `package.json` dependencies), add
`--skip effects` — the check targets `useEffect` anti-patterns and doesn't apply.

Interpret each block and record the findings:

- **`file-size`** — changed file above the line limit. Size alone isn't a defect: a
  long file that's genuinely cohesive (an exhaustive type map, generated code) is fine —
  **discard**. When the file mixes responsibilities that would read better separated,
  record 💡 **Opportunity** pointing at a concrete split.
- **`duplication`** — copied block that touches a changed file. Duplication depends on
  intent: read both fragments and discard when the repetition is incidental
  (similar type declarations, generated shapes, unrelated code that just
  tokenizes the same). When you keep it, it's 💡 **Opportunity** — escalating to ⚠️
  **Warning** for a large block or logic duplicated across layers. Point at extracting a
  shared function/module or, when the twin is existing code, reusing it.
- **`effects`** — `useEffect` anti-pattern
  ([you-might-not-need-an-effect](https://react.dev/learn/you-might-not-need-an-effect)).
  Confirm by reading the surrounding code; when the effect is legitimate, discard. When
  you keep it, it's ⚠️ **Warning** for `effect-fetch-no-cleanup` and `effect-chain`, 💡
  **Opportunity** for the rest.

## Step 7 — Analyze with the lenses

Go through the changed files one by one and apply **all** lenses from
`.claude/docs/pr-review/analysis-lenses.md` to each before moving to the next.
Each lens is checked against the file's current content, not just the diff.

Append a `## Finding:` with `source: lens:<a|b|c>` for each violation the moment you
confirm it.

## Step 8 — Assemble and post the review

Assemble `$WORK/review.json` from the ledger, following
`.claude/docs/pr-review/output-format.md`. No re-analysis: each finding's body is
already final.

- Each `## Finding:` with a concrete line → an item in `inline_comments[]`.
- Findings with `line: —` and the `## Status:` line → woven into the `summary`.
- `event` → `REQUEST_CHANGES` if there's any `critical` finding; otherwise `COMMENT`.

Before posting, **show the summary and comment count to the user and ask for
confirmation** — posting is an action visible to the team and can't be undone by itself.

**Azure DevOps** — each comment is a *thread*. For each item, write a JSON to
`$WORK/thread-<n>.json` and call the API:

```jsonc
// inline: with threadContext
{
  "comments": [{ "parentCommentId": 0, "commentType": 1, "content": "<body>" }],
  "status": 1,
  "threadContext": {
    "filePath": "/src/client/users-api.ts",
    "rightFileStart": { "line": 42, "offset": 1 },
    "rightFileEnd": { "line": 42, "offset": 1 }
  }
}
// summary: the same object without threadContext
```

```bash
az devops invoke --area git --resource pullRequestThreads \
  --route-parameters project=<project.id> repositoryId=<repository.id> pullRequestId=$PR \
  --http-method POST --api-version 7.1 --in-file "$WORK/thread-<n>.json" \
  --organization $ORG
```

Note the mandatory leading `/` in `filePath`. When `event` is `REQUEST_CHANGES`,
record the vote after the threads:

```bash
az repos pr set-vote --id $PR --vote reject --organization $ORG
```

**GitHub** — a single review carries the summary and inline comments. Convert
`review.json` into the API payload (`body`, `event`, `comments[]` with
`path`/`line`/`body`) at `$WORK/gh-review.json`:

```bash
gh api repos/{owner}/{repo}/pulls/$PR/reviews --method POST --input "$WORK/gh-review.json"
```

**GitLab** — post the summary as a note and each finding as a positioned discussion:

```bash
glab mr note $PR --message "<summary>"
glab api projects/:id/merge_requests/$PR/discussions --method POST --input "$WORK/discussion-<n>.json"
```

At the end, tell the user how many comments were posted, the `event` applied, and the
PR link.
