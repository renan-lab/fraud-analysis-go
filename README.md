# fraud-analysis-go

Projeto de portfólio para análise de fraude em transações financeiras, escrito em Go.

> Estágio atual: esqueleto inicial. Arquitetura e regras de negócio serão definidas posteriormente.

## Stack

- Go 1.27 (apenas stdlib por enquanto: `net/http`, `log/slog`)
- PostgreSQL 17
- Redis 8
- Kafka 4 (modo KRaft, sem Zookeeper)
- Docker / Docker Compose

## Estrutura

```
cmd/api/            # entrypoint da API HTTP
internal/config/    # configuração via variáveis de ambiente
```

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

## Portas

| Serviço    | Porta host |
|------------|------------|
| API        | 8080       |
| PostgreSQL | 5432       |
| Redis      | 6379       |
| Kafka      | 29092      |
