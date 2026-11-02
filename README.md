# fraud-analysis-go

Projeto de portfólio para análise de fraude em transações financeiras, escrito em Go.

> Estágio atual: API de ingestão (`POST /transactions` valida e publica no Kafka), worker de análise (regras + baseline estatístico por conta, estado no Redis, decisão `approve`/`review`/`block` publicada em `transactions.analyzed`), simulador de transações e avaliador (compara as decisões com o gabarito do simulador) implementados. Persistência e consulta de alertas virão depois.

## Stack

- Go 1.27 (stdlib primeiro: `net/http`, `log/slog`); dependências: `franz-go` (Kafka), `prometheus/client_golang` (métricas), `x/time/rate` (rate limit), `go-redis` (Redis)
- PostgreSQL 17
- Redis 8
- Kafka 4 (modo KRaft, sem Zookeeper)
- Docker / Docker Compose

## Estrutura

```
cmd/api/              # entrypoint da API HTTP
cmd/worker/           # entrypoint do worker de análise (consumidor Kafka)
cmd/simulator/        # CLI do simulador de transações
cmd/evaluator/        # CLI que avalia as decisões contra o gabarito do simulador
internal/ingestion/   # POST /transactions: validação e publicação no Kafka
internal/analysis/    # motor de fraude: regras, baseline no Redis, decisão
internal/simulator/   # gerador de tráfego realista e adversarial
internal/evaluation/  # métricas de cobertura e qualidade das decisões
internal/events/      # contratos dos eventos Kafka
internal/groundtruth/ # contrato do arquivo de gabarito (truth.ndjson)
internal/platform/    # infraestrutura compartilhada: config, kafka, redis, middlewares HTTP, ids
test/integration/     # testes com Kafka e Redis reais (build tag integration)
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

Stack completa (API, worker e infra) via Docker:

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
  "occurred_at": "2026-11-02T19:15:00Z",
  "location": {"lat": -23.5505, "lon": -46.6333}
}'
# 202 {"transaction_id":"..."}; erros vêm como application/problem+json
# "location" (onde a compra ocorreu) é opcional
```

Outros endpoints: `GET /health` (processo vivo), `GET /ready` (503 se o Kafka estiver inacessível) e `GET /metrics` (Prometheus).

Variáveis da ingestão (veja `.env.example`): `INGEST_PUBLISH_TIMEOUT`, `INGEST_RATE_LIMIT_RPS` (0 desativa), `INGEST_RATE_LIMIT_BURST`, `KAFKA_TOPIC_PARTITIONS`, `KAFKA_TOPIC_REPLICATION_FACTOR`.

Worker de análise local (com `make infra` no ar); no `make up` ele sobe como o serviço `worker`:

```bash
make run-worker
```

Consome `transactions.received`, avalia as regras (`velocity`, `card_testing`, `impossible_travel`, `amount_outlier`, `new_device_foreign`), soma os scores e publica a decisão em `transactions.analyzed`. Limiares, janelas e scores vêm das variáveis `ANALYSIS_*` (veja `.env.example`); o consumer group vem de `WORKER_CONSUMER_GROUP`. Detalhes em [internal/analysis/README.md](internal/analysis/README.md).

Testes:

```bash
make test                                                   # unitários
go test -race -tags=integration ./test/integration/...      # integração com Kafka e Redis (precisa de make infra)
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

## Avaliando as decisões

Com a stack no ar (`make up`), rode o simulador com `--truth` e depois o avaliador sobre o mesmo arquivo:

```bash
go run ./cmd/simulator run --target=http --rate=50 --duration=1m --truth=truth.ndjson
make evaluate                                                                  # mesmo que --truth=truth.ndjson (TRUTH=arquivo muda)
go run ./cmd/evaluator --truth=truth.ndjson --format=json                       # para scripts
go run ./cmd/evaluator --truth=truth.ndjson --min-sequence-recall=0.9 --max-fpr=0.05   # sai com código 1 se violar
```

O avaliador relê o tópico `transactions.analyzed` desde o início (sem consumer group, sem commit) e espera até `--wait` (30 s) pelas decisões que faltam. O relatório mostra cobertura e idempotência do pipeline, precisão/recall/taxa de falsos positivos por transação, detecção por sequência de fraude (cenário x cartão), disparos por regra e a causa das legítimas sinalizadas. Detalhes em [internal/evaluation/README.md](internal/evaluation/README.md).

## Portas

| Serviço    | Porta host |
|------------|------------|
| API        | 8080       |
| PostgreSQL | 5432       |
| Redis      | 6379       |
| Kafka      | 29092      |
