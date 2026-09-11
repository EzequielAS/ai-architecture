# Quality Gate

Single binary that runs three checks on TS/JS code and **exits with code 1** if
it finds violations — built for use in CI or locally.

| Check | What it verifies |
| --- | --- |
| `file-size` | files above the line limit |
| `duplication` | code blocks copied between (or within) files |
| `effects` | `useEffect` anti-patterns ([you-might-not-need-an-effect](https://react.dev/learn/you-might-not-need-an-effect)) |

## Build

```bash
./build.sh          # bin/quality-gate for the current machine
./build.sh all      # binaries for Linux, macOS and Windows (amd64 and arm64)
```

## Usage

```bash
quality-gate [flags] [path ...]
```

Paths can be files or directories (default: `.`). Only `.ts`, `.tsx`,
`.js` and `.jsx` are analyzed; `node_modules` and hidden directories are always
ignored.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--file-size N` | `500` | line limit per file |
| `--dup-tokens N` | `50` | minimum matching tokens to flag duplication |
| `--dup-lines N` | `5` | minimum lines to flag duplication |
| `--ignore GLOB` | — | glob to ignore; repeatable or comma-separated |
| `--check NAME` | all | run only `file-size`, `duplication` or `effects`; repeatable |
| `--skip NAME` | — | skip `file-size`, `duplication` or `effects`; repeatable or comma-separated |

In globs, `**` crosses directories, `*` stays within a single segment, and `?`
matches one character. A glob without `/` also matches the file name at any
level.

```bash
# full gate
quality-gate --file-size 300 --dup-tokens 60 --ignore '**/*.test.*,**/*.d.ts' src

# duplication only, ignoring generated code
quality-gate --check duplication --dup-tokens 80 --ignore '**/generated/**' src

# everything except effects
quality-gate --skip effects src

# skipping more than one check
quality-gate --skip effects,duplication src
```

`--skip` starts from all checks and removes the ones listed. Combined with
`--check`, it is applied afterward: `--check file-size,effects --skip effects`
runs only `file-size`. Skipping every check is a usage error.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | no violations |
| `1` | violations found |
| `2` | usage error (e.g. invalid `--check`/`--skip`, or `--skip` removing every check) |

## useEffect rules

| Rule | When it fires |
| --- | --- |
| `effect-derives-state` | the Effect only does `setState` with a value computed from its dependencies |
| `effect-resets-state` | the Effect only resets state to a constant value |
| `effect-notifies-parent` | the Effect calls a parent `onX` callback |
| `effect-external-store` | the Effect subscribes to ambient browser state (`online`, `storage`, …) |
| `effect-fetch-no-cleanup` | the Effect fetches data and does `setState` without cleanup |
| `effect-chain` | the Effect depends on state that another Effect updates |
