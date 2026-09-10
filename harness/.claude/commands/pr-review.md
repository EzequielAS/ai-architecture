---
description: Revisa um PR contra spec, doc de arquitetura e gates do projeto, e posta os findings como comentários inline e resumo no repositório remoto
argument-hint: <PR_NUMBER>
allowed-tools: Read, Glob, Grep, Bash(.claude/bin/pr-review/*), Bash(mkdir:*), Bash(printf:*), Bash(cat:*), Bash(tee:*), Bash(node:*), Bash(npm run:*), Bash(pnpm:*), Bash(yarn:*), Bash(git remote:*), Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git rev-parse:*), Bash(git fetch:*), Bash(gh:*), Bash(glab:*), Bash(az:*)
---

# PR Review

Revisa o PR `$1` juntando três fontes de sinal — a **análise estática do binário**, a
**spec relacionada** e o **documento de arquitetura** — e posta o resultado no
repositório remoto.

**Invocação:** `/pr-review <PR_NUMBER>`

---

## Convenções

| Placeholder | Valor |
|---|---|
| `$GATE` | `.claude/bin/pr-review/quality-gate-linux` no Linux, `.claude/bin/pr-review/quality-gate-darwin` no macOS (resolva pelo SO informado no ambiente) |
| `$WORK` | um diretório temporário desta execução, ex.: `<scratchpad>/pr-review-<PR_NUMBER>` |
| `$PR` | o número do PR passado como argumento |

Se `$GATE` vier sem bit de execução após um clone: `chmod +x .claude/bin/pr-review/*`.

Tudo é acumulado em `$WORK/review.md`. A interpretação acontece **uma vez**, no momento
em que cada saída é produzida — o Passo 7 só monta o JSON, nunca reanalisa. O arquivo
tem dois tipos de bloco:

- **`## Status:`** — resultado dos gates do projeto, ex.:
  `## Status: lint PASS · build PASS · test FAIL`.
- **`## Finding:`** — o ledger interpretado. Anexe cada finding no momento em que
  confirmá-lo, para nada se perder por limite de contexto. Formato:

  ```markdown
  ## Finding: <path>:<line> — <emoji> **<label>**
  - type: critico | alerta | oportunidade
  - source: lint | build | test | file-size | duplication | effects | lens:<a|b|c>

  <corpo — o texto final do comentário inline; cole aqui a saída de erro relevante>
  ```

  Finding sem linha concreta (falha de build/test) usa `line: —` e vai para o resumo,
  não para um comentário inline.

---

## Passo 1 — Detectar o provider e conferir o ferramental

```bash
git remote get-url origin
```

Pela URL, escolha o provider e confirme que a CLI dele está instalada **e**
autenticada. Se faltar qualquer uma das duas, **pare** e instrua o usuário a instalar ou
autenticar — não tente contornar.

| Origin contém | Provider | CLI | Verificação |
|---|---|---|---|
| `dev.azure.com` / `.visualstudio.com` | Azure DevOps | `az` + extensão `azure-devops` | `az account show` e `az extension list --query "[?name=='azure-devops']" -o tsv` |
| `github.com` | GitHub | `gh` | `gh auth status` |
| `gitlab` | GitLab | `glab` | `glab auth status` |

No Azure DevOps, derive a organização do origin
(`.../v3/<ORG>/<PROJECT>/<REPO>` → `https://dev.azure.com/<ORG>`) e passe
`--organization <ORG_URL>` em **todos** os comandos `az`. Guarde esse valor como `$ORG`.

## Passo 2 — Buscar os dados do PR

Pegue: `title`, branch de origem, branch de destino, autor e a **lista de arquivos
alterados**.

**Azure DevOps**

```bash
az repos pr show --id $PR --organization $ORG -o json
```

Do JSON, guarde `title`, `sourceRefName`, `targetRefName`, `createdBy.uniqueName`,
`repository.id` e `repository.project.id` (os dois últimos são necessários no Passo 8).
Para o diff, use os commits do merge:

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

## Passo 3 — Ler os arquivos alterados

Leia o conteúdo **atual e completo** de cada arquivo alterado com a tool `Read`. O diff
sozinho não dá contexto para avaliar decisão de arquitetura, nomeação ou lógica.

## Passo 4 — Reunir spec e documento de arquitetura

1. **Spec relacionada** — procure com `Glob`: `specs/**/*.md`, `.specs/**/*.md`,
   `**/*.spec.md`. Case contra a mudança pelo nome dos arquivos alterados e pelo
   `title`/branch do PR; confirme com `Grep`/`Read` quando o casamento for fraco. Leia a
   spec que casar por inteiro. Se nenhuma casar, registre "sem spec relacionada".
2. **Documento de arquitetura** — procure na pasta `docs/` do repo: `docs/*.md` com nome
   de arquitetura (`arquitetura`, `architecture`, `design`) e `docs/adr/*.md`. Leia por
   inteiro o que existir. Se não houver, registre "sem doc de arquitetura".

## Passo 5 — Rodar os gates do projeto

Leia o `package.json` e descubra os scripts de **lint**, **build** e **test** que o
projeto realmente define (nomes variam: `lint`, `build`, `test`, `test:coverage`,
`typecheck`, ou um agregado como `ci`). Use o gerenciador declarado em
`packageManager`/lockfile. Rode cada um que existir; pule com `SKIP` o que não existir.

Registre uma linha `## Status:` com o resultado de cada gate e um `## Finding:` por
falha, classificando pela referência `.claude/docs/pr-review/comment-types.md`:

- **Lint FAIL** → ⚠️ **Alerta** no primeiro arquivo/linha reportado.
- **Build FAIL** → 🚨 **Crítico** com `line: —`. Antes de registrar, confirme que a falha
  vem do código do PR e não de config local ausente (`.env`, `*.local`, `.npmrc`).
- **Test FAIL** → 🚨 **Crítico** com `line: —`.

Cole a saída de erro direto no corpo do finding.

## Passo 6 — Rodar a análise estática

O binário faz três checagens em `.ts`/`.tsx`/`.js`/`.jsx`: `file-size`, `duplication` e
`effects`. Ele **sai com código 1 quando encontra violações** — encadeie `|| true` para
não abortar o passo.

Rode sobre a **raiz do código-fonte** (ex.: `src`), não só sobre os arquivos alterados: a
checagem de duplicação precisa do corpus inteiro para achar o gêmeo em código existente.
Depois **filtre** a saída, mantendo só violações que tocam um arquivo alterado no PR.

```bash
$GATE --ignore '**/*.test.*,**/*.d.ts' src 2>&1 | tee "$WORK/gate.txt" || true
```

Ajuste os limiares ao projeto quando fizer sentido: `--file-size N` (padrão 500),
`--dup-tokens N` (padrão 50), `--dup-lines N` (padrão 5), `--ignore GLOB`,
`--check NOME` / `--skip NOME` (`file-size`, `duplication`, `effects`). `$GATE --help`
lista tudo.

**Se o projeto não tem React** (sem `react` nas dependências do `package.json`), adicione
`--skip effects` — a checagem é de anti-padrão de `useEffect` e não se aplica.

Interprete cada bloco e registre os findings:

- **`file-size`** — arquivo alterado acima do limite de linhas. Tamanho por si só não é
  defeito: um arquivo longo e genuinamente coeso (um mapa exaustivo de tipos, código
  gerado) está ok — **descarte**. Quando o arquivo mistura responsabilidades que leriam
  melhor separadas, registre 💡 **Oportunidade** apontando um corte concreto.
- **`duplication`** — bloco copiado que toca um arquivo alterado. Duplicação depende de
  intenção: leia os dois fragmentos e descarte quando a repetição for incidental
  (declarações de tipo parecidas, shapes gerados, código não relacionado que só
  tokeniza igual). Quando mantiver, é 💡 **Oportunidade** — escalando para ⚠️ **Alerta**
  em bloco grande ou lógica duplicada entre camadas. Aponte a extração de uma
  função/módulo comum ou, quando o gêmeo for código existente, o reuso dele.
- **`effects`** — anti-padrão de `useEffect`
  ([you-might-not-need-an-effect](https://react.dev/learn/you-might-not-need-an-effect)).
  Confirme lendo o código ao redor; quando o efeito for legítimo, descarte. Quando
  mantiver, é ⚠️ **Alerta** para `effect-fetch-no-cleanup` e `effect-chain`, 💡
  **Oportunidade** para os demais.

## Passo 7 — Analisar com as lentes

Percorra os arquivos alterados um a um e aplique **todas** as lentes de
`.claude/docs/pr-review/analysis-lenses.md` em cada um antes de passar para o próximo.
Cada lente é verificada contra o conteúdo atual do arquivo, não só contra o diff.

Anexe um `## Finding:` com `source: lens:<a|b|c>` para cada violação no momento em que
confirmá-la.

## Passo 8 — Montar e postar o review

Monte `$WORK/review.json` a partir do ledger, seguindo
`.claude/docs/pr-review/output-format.md`. Sem reanálise: o corpo de cada finding já
está final.

- Cada `## Finding:` com linha concreta → um item de `inline_comments[]`.
- Findings com `line: —` e a linha `## Status:` → tecidos no `summary`.
- `event` → `REQUEST_CHANGES` se houver qualquer finding `critico`; senão `COMMENT`.

Antes de postar, **mostre o resumo e a contagem de comentários ao usuário e peça
confirmação** — postar é uma ação visível para o time e não se desfaz sozinha.

**Azure DevOps** — cada comentário é uma *thread*. Para cada item, escreva um JSON em
`$WORK/thread-<n>.json` e invoque a API:

```jsonc
// inline: com threadContext
{
  "comments": [{ "parentCommentId": 0, "commentType": 1, "content": "<body>" }],
  "status": 1,
  "threadContext": {
    "filePath": "/src/client/users-api.ts",
    "rightFileStart": { "line": 42, "offset": 1 },
    "rightFileEnd": { "line": 42, "offset": 1 }
  }
}
// resumo: o mesmo objeto sem threadContext
```

```bash
az devops invoke --area git --resource pullRequestThreads \
  --route-parameters project=<project.id> repositoryId=<repository.id> pullRequestId=$PR \
  --http-method POST --api-version 7.1 --in-file "$WORK/thread-<n>.json" \
  --organization $ORG
```

Note o `/` inicial obrigatório no `filePath`. Quando o `event` for `REQUEST_CHANGES`,
registre o voto depois das threads:

```bash
az repos pr set-vote --id $PR --vote reject --organization $ORG
```

**GitHub** — um único review carrega resumo e comentários inline. Converta
`review.json` para o payload da API (`body`, `event`, `comments[]` com
`path`/`line`/`body`) em `$WORK/gh-review.json`:

```bash
gh api repos/{owner}/{repo}/pulls/$PR/reviews --method POST --input "$WORK/gh-review.json"
```

**GitLab** — poste o resumo como nota e cada finding como discussão posicionada:

```bash
glab mr note $PR --message "<summary>"
glab api projects/:id/merge_requests/$PR/discussions --method POST --input "$WORK/discussion-<n>.json"
```

Ao final, informe ao usuário quantos comentários foram postados, o `event` aplicado e o
link do PR.
