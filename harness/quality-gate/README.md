# Quality Gate

Binário único que roda três checagens em código TS/JS e **sai com código 1** se
encontrar violações — feito para usar em CI ou local.

| Checagem | O que verifica |
| --- | --- |
| `file-size` | arquivos acima do limite de linhas |
| `duplication` | blocos de código copiados entre (ou dentro de) arquivos |
| `effects` | anti-padrões de `useEffect` ([you-might-not-need-an-effect](https://react.dev/learn/you-might-not-need-an-effect)) |

## Build

```bash
./build.sh          # bin/quality-gate para a máquina atual
./build.sh all      # binários para Linux, macOS e Windows (amd64 e arm64)
```

## Uso

```bash
quality-gate [flags] [caminho ...]
```

Os caminhos podem ser arquivos ou diretórios (padrão: `.`). Só `.ts`, `.tsx`,
`.js` e `.jsx` são analisados; `node_modules` e diretórios ocultos são sempre
ignorados.

## Flags

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--file-size N` | `500` | limite de linhas por arquivo |
| `--dup-tokens N` | `50` | mínimo de tokens iguais para acusar duplicação |
| `--dup-lines N` | `5` | mínimo de linhas para acusar duplicação |
| `--ignore GLOB` | — | glob a ignorar; repetível ou separado por vírgula |
| `--check NOME` | todas | roda só `file-size`, `duplication` ou `effects`; repetível |
| `--skip NOME` | — | pula `file-size`, `duplication` ou `effects`; repetível ou separado por vírgula |

Nos globs, `**` atravessa diretórios, `*` fica dentro de um segmento e `?` é um
caractere. Um glob sem `/` também casa com o nome do arquivo em qualquer nível.

```bash
# gate completo
quality-gate --file-size 300 --dup-tokens 60 --ignore '**/*.test.*,**/*.d.ts' src

# só duplicação, ignorando código gerado
quality-gate --check duplication --dup-tokens 80 --ignore '**/generated/**' src

# tudo menos os efeitos
quality-gate --skip effects src

# pulando mais de uma checagem
quality-gate --skip effects,duplication src
```

`--skip` parte de todas as checagens e remove as indicadas. Combinado com
`--check`, é aplicado depois: `--check file-size,effects --skip effects` roda só
`file-size`. Pular todas as checagens é erro de uso.

## Códigos de saída

| Código | Significado |
| --- | --- |
| `0` | nenhuma violação |
| `1` | violações encontradas |
| `2` | erro de uso (ex.: `--check`/`--skip` inválido, ou `--skip` removendo todas) |

## Regras de useEffect

| Regra | Quando dispara |
| --- | --- |
| `effect-derives-state` | o Effect só faz `setState` com valor calculado das dependências |
| `effect-resets-state` | o Effect só reseta estado para um valor constante |
| `effect-notifies-parent` | o Effect chama um callback `onX` do pai |
| `effect-external-store` | o Effect assina estado ambiente do browser (`online`, `storage`, …) |
| `effect-fetch-no-cleanup` | o Effect busca dados e faz `setState` sem cleanup |
| `effect-chain` | o Effect depende de um estado que outro Effect atualiza |
