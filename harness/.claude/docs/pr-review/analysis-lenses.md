# Analysis Lenses

Cada lente é aplicada arquivo a arquivo, contra o **conteúdo atual** do arquivo (não só
o diff). Para cada finding registre: caminho, linha (versão HEAD), tipo de comentário e
uma mensagem curta e acionável.

---

**Lens A — Arquitetura.** Procure um documento de arquitetura na pasta `docs/` do repo
sendo revisado (ex.: `docs/arquitetura.md`, `docs/software-architecture.md`,
`docs/adr/*.md`). Leia-o por inteiro e verifique cada regra/decisão contra o código
alterado — violação de camada, import proibido, contrato quebrado, decisão de ADR
contrariada. Se não houver documento, pule esta lente e registre "sem doc de arquitetura".

**Lens B — Aderência à spec** (só quando uma spec relacionada foi encontrada). Para cada
user story P1 da spec: os critérios de aceite (WHEN/THEN/SHALL) estão cobertos? Se o PR
for parcial, aponte o que está atendido e o que está pendente. Não penalize stories
P2/P3 incompletas numa implementação parcial.

**Lens C — Sinais gerais de qualidade.**
- O erro é tratado na camada certa (service, não espalhado)? Erro conhecido mapeado para
  código adequado e erro inesperado sem vazar detalhe interno?
- Entrada validada e saneada antes de chegar na regra de negócio?
- Há casos de borda óbvios que o código ignora?
- Há complexidade desnecessária para o que a mudança precisa de fato fazer?
- A mudança vem com testes proporcionais ao risco?
