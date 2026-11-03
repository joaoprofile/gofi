---
name: absolute-rules
description: Regras que nunca se quebram num backend Axum + sqlx com gofi — clippy sem warning, sem unwrap fora do main, dinheiro em i64 de centavos, segredos fora do código, teste em arquivo próprio, rastreio gofi:implements e gofi:tests
keywords: [regras, absolutas, rust, clippy, unwrap, dinheiro, centavos, i64, segredo, testes, gofi:implements, gofi:tests]
---

# Regras absolutas — Axum + sqlx

1. **`cargo clippy -- -D warnings` limpo.** Warning é erro: o sensor estático
   do gofi o trata assim.
2. **Sem `unwrap()`/`expect()`** fora do `main.rs` e dos testes; erro é `Result`.
3. **Dinheiro é `i64` em centavos.** Nunca `f64` para valor monetário.
4. **Segredo e configuração vêm do ambiente**, lidos no `main.rs`.
5. **Teste em arquivo próprio**, `{camada}_tests.rs`. **Quem cria a camada
   declara no `mod.rs`, junto com ela, o `#[cfg(test)] mod {camada}_tests;`** e
   cria o arquivo vazio: o `mod.rs` é código, e quem escreve os testes não o
   edita — sem a declaração, os testes nem compilam.
6. **Rastreio.** A função que realiza um requisito leva `// gofi:implements
   REQ-NN`; o `#[test]` que prova um cenário, `// gofi:tests REQ-NN.n` logo acima.
7. **Teste ignorado não existe.** `#[ignore]` reprova o sensor de testes.
