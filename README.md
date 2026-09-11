# E-commerce orientado a eventos

Avaliação 2 de Sistemas Distribuídos (UTFPR / DAINF). Backend de e-commerce em
cinco microsserviços que se comunicam **só** por eventos no RabbitMQ, com
assinatura digital RSA em todo evento publicado.

Nenhum processo chama outro diretamente. Não existe HTTP, RPC nem banco
compartilhado entre eles: se um evento não passa pelo broker, ele não
acontece.

## Divisão do trabalho

| Parte | Responsável | Situação |
|---|---|---|
| Microsserviço Estoque | Daniel | pronto |
| Microsserviço Pagamento | Daniel | pronto |
| Assinatura digital dos eventos | Daniel | pronto |
| Microsserviço Entrega | dupla | a fazer |
| Microsserviço Promoções | dupla | a fazer |
| Consumidores C1 e C2 de promoções | dupla | a fazer |
| Validação da assinatura | dupla | a fazer |
| Microsserviço Principal | em dupla | a fazer |

Cada parte a fazer tem o contrato escrito no comentário do próprio arquivo.
O que já está pronto serve de modelo: `cmd/pagamento/main.go` é o esqueleto
mais simples e `cmd/estoque/main.go` é o mais completo.

## Como rodar

Pré-requisitos: Go 1.22+ e Docker.

```bash
# 1. broker (painel em http://localhost:15672, usuário e senha guest)
make broker

# 2. chaves RSA, uma vez só
make chaves

# 3. um microsserviço por terminal
make estoque
make pagamento
```

Enquanto o Principal não existe, dá para exercitar o fluxo com duas
ferramentas de desenvolvimento:

```bash
# acompanha todos os eventos da exchange eCommerce
make observador

# pedido que cabe no estoque
go run ./cmd/teste-publicador -itens P001:2,P004:1

# pedido que estoura o estoque do Air Fryer, que só tem 8 unidades
go run ./cmd/teste-publicador -itens P006:99

# devolve ao estoque o que o pedido tinha reservado
go run ./cmd/teste-publicador -tipo pedido.excluido -pedido PED-001 -motivo teste
```

`make teste` roda a suíte. Os testes não precisam de broker: os serviços
recebem o publicador por interface, então o teste injeta um publicador falso
e confere a regra de negócio direto.

## Topologia no RabbitMQ

Duas exchanges, ambas duráveis. Não há exchange fanout.

**eCommerce**, tipo `direct`. A binding key tem de ser exatamente igual à
routing key do evento.

| Routing key | Publica | Consome |
|---|---|---|
| `pedido.criado` | Principal | Estoque |
| `pedido.excluido` | Principal | Estoque |
| `pedido.estoque_ok` | Estoque | Pagamento, Principal |
| `estoque.indisponivel` | Estoque | Principal |
| `pagamento.aprovado` | Pagamento | Entrega, Principal |
| `pagamento.recusado` | Pagamento | Principal |
| `pedido.enviado` | Entrega | Principal |

**promocoes**, tipo `topic`. Routing keys `promocao.categoria.A`,
`promocao.categoria.B` e `promocao.categoria.C`.

| Fila | Binding keys | Recebe |
|---|---|---|
| `promocoes.c1` | `promocao.categoria.A` e `promocao.categoria.B` | só as categorias A e B |
| `promocoes.c2` | `promocao.categoria.#` | todas as categorias |

O contraste entre C1 e C2 é o ponto da exchange topic. C1 precisa de uma
binding por categoria que interessa. C2 resolve tudo com uma binding só,
porque `#` casa zero ou mais palavras, e uma categoria D passaria a chegar
nele sem mudar nada.

Cada consumidor tem a sua própria fila, todas duráveis, todas com
confirmação manual (`ack`) e `prefetch` de 1. Duas filas vinculadas à mesma
routing key recebem cada uma a sua cópia da mensagem; dois consumidores numa
fila só dividiriam as mensagens entre si, e aí o Principal perderia metade
das atualizações de status.

Os nomes de filas e binding keys estão todos em `internal/evento/filas.go`.

## Fluxo de um pedido

```
Principal ──pedido.criado──▶ Estoque
                              │
              tem estoque?    ├──não──▶ estoque.indisponivel ──▶ Principal
                              │                                     │
                              └──sim──▶ pedido.estoque_ok           │
                                          │         │               │
                                    Principal   Pagamento           │
                                                    │               │
                                        sorteio ────┤               │
                                                    │               │
                            pagamento.recusado ◀────┤               │
                                    │               │               │
                                    ▼               └──▶ pagamento.aprovado
                                Principal                     │        │
                                    │                   Principal   Entrega
                                    │                                  │
                                    ▼                                  ▼
                              pedido.excluido ◀───────────────  pedido.enviado
                                    │                                  │
                                    ▼                                  ▼
                            Estoque devolve                       Principal
```

O Principal publica `pedido.excluido` nos dois caminhos de cancelamento:
estoque indisponível e pagamento recusado.

## Assinatura digital

Todo evento sai assinado e todo evento entra validado. O envelope é igual
para as duas exchanges:

```json
{
  "id": "3f9a1c2b8e4d7601",
  "tipo": "pedido.criado",
  "produtor": "principal",
  "timestamp": "2026-09-10T22:59:00.123456789-03:00",
  "dados": { "pedido_id": "PED-001", "itens": [] },
  "signature": "base64 da assinatura RSA"
}
```

Ao publicar (`internal/cripto/assinar.go`):

1. monta os **bytes canônicos** do evento, que é o envelope inteiro menos o
   campo `signature`;
2. calcula o SHA-256 desses bytes;
3. assina o hash com a chave privada do produtor, em RSASSA-PKCS#1 v1.5;
4. grava o resultado em base64 no campo `signature`.

Ao consumir (`internal/cripto/verificar.go`):

1. pega a chave pública do produtor declarado no envelope;
2. remonta os mesmos bytes canônicos;
3. confere a assinatura;
4. processa só se bater. Assinatura inválida, produtor desconhecido ou
   mensagem ilegível fazem o evento ser descartado com `Nack` sem reenfileirar.

Todo publicador passa por `mensageria.Publicador` e todo consumidor por
`mensageria.Consumidor`, então não existe caminho pelo qual um evento saia
sem assinatura ou entre sem validação.

**O ponto delicado é a reprodutibilidade dos bytes.** Produtor e consumidor
precisam gerar exatamente a mesma sequência de bytes, senão a validação falha
mesmo com a mensagem intacta. Por isso o envelope só tem campos string e o
payload fica como `json.RawMessage`: a ida e volta pelo `encoding/json`
devolve os bytes originais em vez de reserializar o objeto. O teste
`TestAssinaturaSobreviveAoTransporteJSON` existe para travar essa
propriedade.

### Chaves

`make chaves` gera um par RSA de 2048 bits por microsserviço e copia as
chaves públicas de todos para dentro da pasta de cada um:

```
chaves/
  estoque/
    privada.pem          ← só o Estoque usa, para assinar
    publicas/
      principal.pem      ← usadas para validar o que chega
      pagamento.pem
      entrega.pem
      promocoes.pem
      estoque.pem
```

Privada em PKCS#8, pública em PKIX, as duas em PEM. São os formatos que
Python e Java também leem, então dá para conferir uma assinatura por fora se
a professora pedir.

Rodar `make chaves` de novo **preserva** as chaves existentes. Use
`go run ./cmd/gerar-chaves -forcar` para regerar tudo.

As chaves estão versionadas de propósito: a dupla precisa do mesmo conjunto
nas duas máquinas, senão a assinatura de um não valida no outro. Em sistema
de verdade isso nunca se faz.

## Estrutura

```
cmd/
  principal/        microsserviço Principal        (a fazer em dupla)
  estoque/          microsserviço Estoque          ✓
  pagamento/        microsserviço Pagamento        ✓
  entrega/          microsserviço Entrega          (dupla)
  promocoes/        microsserviço Promoções        (dupla)
  consumidor-c1/    consumidor de promoções C1     (dupla)
  consumidor-c2/    consumidor de promoções C2     (dupla)
  gerar-chaves/     utilitário de chaves           ✓
  observador/       ferramenta de desenvolvimento  ✓
  teste-publicador/ ferramenta de desenvolvimento  ✓
internal/
  evento/           envelope, routing keys, filas, payloads
  cripto/           chaves, assinatura, validação
  mensageria/       conexão, publicador, consumidor
  catalogo/         lista fixa de produtos
  estoque/          regra de negócio do Estoque
  pagamento/        regra de negócio do Pagamento
```

`internal/evento` é o contrato entre os cinco microsserviços. Mudança ali
precisa ser combinada entre a dupla.

O catálogo em `internal/catalogo` é dado estático compartilhado, como um
arquivo de configuração que cada processo lê ao iniciar. Não é chamada entre
processos: o Estoque o usa para iniciar as quantidades e o Principal para
montar a vitrine, sem um falar com o outro.

## Variáveis de ambiente

| Variável | Padrão | Para que serve |
|---|---|---|
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | endereço do broker |
| `CHAVES_DIR` | `chaves` | raiz das pastas de chaves |
| `VERIFICAR_ASSINATURA` | ligado | `off` desliga a validação |
| `TAXA_APROVACAO` | `0.7` | fração de pagamentos aprovados na simulação |

`TAXA_APROVACAO=0` faz todo pagamento ser recusado, o que é o jeito curto de
demonstrar o estorno do estoque na defesa.

`VERIFICAR_ASSINATURA=off` existe só para destravar o desenvolvimento
enquanto a validação é um stub. Na entrega e na defesa, deixe ligado.
