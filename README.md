# fraud-analysis-go

Projeto de portfólio para análise de fraude em transações financeiras, escrito em Go.

> Estágio atual: API com `GET /health` e simulador de transações implementado. Regras de negócio serão definidas posteriormente.

## Stack

- Go 1.27 (apenas stdlib por enquanto: `net/http`, `log/slog`)
- PostgreSQL 17
- Redis 8
- Kafka 4 (modo KRaft, sem Zookeeper)
- Docker / Docker Compose

## Estrutura

```
cmd/api/              # entrypoint da API HTTP
cmd/simulator/        # CLI do simulador de transações
internal/config/      # configuração via variáveis de ambiente
internal/simulator/   # gerador de tráfego realista e adversarial
docs/decisions/       # registros de decisões de arquitetura (ADRs)
```

Visão geral da arquitetura e contrato da API: [ARCHITECTURE.md](ARCHITECTURE.md).

## Pré-requisitos

- Go 1.27+
- Docker e Docker Compose

## Como rodar

```bash
make env      # cria .env a partir de .env.example
```

Stack completa (app + infra) via Docker:

```bash
make up
curl localhost:8080/health   # {"status":"ok"}
make logs
make down
```

Apenas a infra no Docker e a API local:

```bash
make infra
make run
```

Rode `make help` para ver todos os comandos.

## Simulador de transações

Gera tráfego realista (clientes com perfil) e adversarial (fraudes, payloads malformados, ataques, duplicados, timestamps inválidos, picos de carga) contra `POST /transactions`.

```bash
go run ./cmd/simulator scenarios            # lista os cenários e o status esperado
go run ./cmd/simulator run --count=50       # dry-run: imprime NDJSON no stdout
go run ./cmd/simulator run --target=http --rate=50 --duration=1m --truth=truth.ndjson
```

O destino HTTP vem de `SIMULATOR_TARGET_URL` (padrão `http://localhost:8080`). O relatório (status, latências p50/p95/p99, violações de expectativa) sai no stderr. Detalhes em [internal/simulator/README.md](internal/simulator/README.md).

## Portas

| Serviço    | Porta host |
|------------|------------|
| API        | 8080       |
| PostgreSQL | 5432       |
| Redis      | 6379       |
| Kafka      | 29092      |
