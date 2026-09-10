# Output Format

O review final é um único JSON em `$WORK/review.json`. Cada finding vira um comentário
inline; o mesmo emoji e label de `comment-types.md` vão no `body`.

```json
{
  "event": "REQUEST_CHANGES",
  "summary": "## 📋 Resumo — PR #<number>: <title>\n\n<resumo geral, sem citar arquivo ou linha>\n\n### Pontos de atenção\n- ...\n\n### Cobertura da spec\n<só quando uma spec foi encontrada; caso contrário, omita a seção>",
  "inline_comments": [
    {
      "path": "src/client/users-api.ts",
      "line": 42,
      "body": "🚨 **Crítico** — Título curto\n\nExplicação..."
    }
  ]
}
```

`event`: `"REQUEST_CHANGES"` quando houver ao menos um 🚨 **Crítico** (inclusive falha de
build ou teste); `"COMMENT"` caso contrário.

`summary`: escreva para quem lê frio — o que foi revisado, o sinal geral de qualidade, o
resultado dos gates (lint/build/test) e os padrões relevantes. Não repita os findings
inline. Máximo ~250 palavras.

`inline_comments[].path`: caminho relativo à raiz do repo, sem `./` e sem barra inicial —
o formato exigido por cada provider é montado na hora de postar.
