# Serviço A - Input

## Descrição
Serviço responsável por receber requisições de CEP e encaminhar para o Serviço B.

## Funcionalidades
- Recebe requisições POST com CEP
- Valida se o CEP tem 8 dígitos
- Encaminha requisição válida para o Serviço B
- Retorna resposta do Serviço B

## Endpoints

### POST /
Recebe um CEP e retorna informações de clima.

**Request Body:**
```json
{
  "cep": "01001000"
}
```

**Responses:**

**200 OK** - Sucesso
```json
{
  "city": "São Paulo",
  "temp_C": 28.5,
  "temp_F": 83.3,
  "temp_K": 301.5
}
```

**422 Unprocessable Entity** - CEP inválido
```json
{
  "message": "invalid zipcode"
}
```

**404 Not Found** - CEP não encontrado
```json
{
  "message": "can not find zipcode"
}
```

## Configuração

### Variáveis de Ambiente
Copie o arquivo `.env.example` para `.env` e configure:

```bash
SERVICE_B_URL=http://localhost:8081
PORT=8080
```

## Como executar

```bash
# Instalar dependências
go mod tidy

# Executar
go run main.go
```

## Exemplo de uso

```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"cep":"01001000"}'
```
