# fraud-analysis-go

Projeto de portfólio para análise de fraude em transações financeiras, escrito em Go.

> Estágio atual: API de ingestão (`POST /transactions` valida e publica no Kafka) e simulador de transações implementados. O worker de análise e as regras de negócio serão definidos posteriormente.

## Stack

- Go 1.27 (stdlib primeiro: `net/http`, `log/slog`); dependências: `franz-go` (Kafka), `prometheus/client_golang` (métricas), `x/time/rate` (rate limit)
- PostgreSQL 17
- Redis 8
- Kafka 4 (modo KRaft, sem Zookeeper)
- Docker / Docker Compose

## Estrutura

```
cmd/api/              # entrypoint da API HTTP
cmd/simulator/        # CLI do simulador de transações
internal/ingestion/   # POST /transactions: validação e publicação no Kafka
internal/simulator/   # gerador de tráfego realista e adversarial
internal/events/      # contratos dos eventos Kafka
internal/platform/    # infraestrutura compartilhada: config, kafka, middlewares HTTP, ids
test/integration/     # testes com Kafka real (build tag integration)
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

A API precisa do Kafka no startup: cria o tópico `transactions.received` e encerra se o broker não responder em 15 s.

Enviando uma transação:

```bash
curl -i -X POST localhost:8080/transactions -H 'Content-Type: application/json' -d '{
  "transaction_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "account_id": "acc_3f9a1c2b7d4e", "card_id": "card_9b2c4d6e8f10",
  "amount": 12990, "currency": "BRL",
  "merchant": {"id": "mer_51a2b3c4d5e6", "name": "Grocery 5e6f", "mcc": "5411", "country": "BR"},
  "channel": "card_present", "device_id": "dev_0a1b2c3d4e5f", "ip": "177.34.10.200",
  "occurred_at": "2026-11-02T19:15:00Z"
}'
# 202 {"transaction_id":"..."}; erros vêm como application/problem+json
```

Outros endpoints: `GET /health` (processo vivo), `GET /ready` (503 se o Kafka estiver inacessível) e `GET /metrics` (Prometheus).

Variáveis da ingestão (veja `.env.example`): `INGEST_PUBLISH_TIMEOUT`, `INGEST_RATE_LIMIT_RPS` (0 desativa), `INGEST_RATE_LIMIT_BURST`, `KAFKA_TOPIC_PARTITIONS`, `KAFKA_TOPIC_REPLICATION_FACTOR`.

Testes:

```bash
make test                                                   # unitários
go test -race -tags=integration ./test/integration/...      # integração (precisa de make infra)
```

Rode `make help` para ver todos os comandos.

## Simulador de transações

Gera tráfego realista (clientes com perfil) e adversarial (fraudes, payloads malformados, ataques, duplicados, timestamps inválidos, picos de carga) contra `POST /transactions`.

```bash
go run ./cmd/simulator scenarios            # lista os cenários e o status esperado
go run ./cmd/simulator run --count=50       # dry-run: imprime NDJSON no stdout
go run ./cmd/simulator run --target=http --rate=50 --duration=1m --truth=truth.ndjson
```

O destino HTTP vem de `SIMULATOR_TARGET_URL` (padrão `http://localhost:8080`). O relatório (status, latências p50/p95/p99, violações de expectativa) sai no stderr. Com rate limit ativo, respostas 429 aparecem como violações; para testes de carga deixe `INGEST_RATE_LIMIT_RPS=0`. Detalhes em [internal/simulator/README.md](internal/simulator/README.md).

## Portas

| Serviço    | Porta host |
|------------|------------|
| API        | 8080       |
| PostgreSQL | 5432       |
| Redis      | 6379       |
| Kafka      | 29092      |
