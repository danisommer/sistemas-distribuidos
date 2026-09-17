# E-commerce orientado a eventos

Backend de e-commerce em Go dividido em microsserviços que conversam **só** por
eventos no RabbitMQ. Nenhum processo chama outro diretamente. Todo evento sai
assinado digitalmente (RSA) e é validado por quem o recebe.

## Processos

| Processo | Comando | O que faz |
|---|---|---|
| Principal | `make principal` | menu de terminal: vitrine, pedidos e status em tempo real |
| Estoque | `make estoque` | reserva os itens do pedido e devolve quando ele é cancelado |
| Pagamento | `make pagamento` | aprova ou recusa o pagamento (simulado) |
| Entrega | `make entrega` | emite nota fiscal e código de rastreio (simulado) |
| Promoções | `make promocoes` | publica uma promoção aleatória a cada 3 a 8 segundos |
| Consumidor C1 | `make c1` | recebe promoções das categorias A e B |
| Consumidor C2 | `make c2` | recebe promoções de todas as categorias |

## Como rodar

Pré-requisitos: Go 1.26+ (versão do `go.mod`), Docker e `make`.

```bash
make broker     # RabbitMQ; painel em http://localhost:15672 (guest / guest)
make chaves     # opcional: as chaves já vêm no repositório e rodar de novo não as troca
```

Depois, um processo por terminal:

```bash
make estoque
make pagamento
make entrega
make principal  # o menu; pede seu nome e mostra as opções
```

Para as promoções, suba os consumidores **antes** do gerador. A fila de cada
consumidor só passa a existir quando ele sobe pela primeira vez, e promoção
publicada sem fila vinculada se perde.

```bash
make c1
make c2
make promocoes
```

Os serviços tentam conectar no broker por até 1 minuto, então dá para subir tudo
enquanto o RabbitMQ ainda está iniciando. `Ctrl+C` encerra qualquer processo e
`make` sozinho lista todos os comandos.

## Fluxo dos eventos

Exchange `eCommerce`, tipo `direct`:

```
Principal ── pedido.criado ────────▶ Estoque
Estoque   ── pedido.estoque_ok ────▶ Pagamento e Principal
Estoque   ── estoque.indisponivel ─▶ Principal
Pagamento ── pagamento.aprovado ───▶ Entrega e Principal
Pagamento ── pagamento.recusado ───▶ Principal
Entrega   ── pedido.enviado ───────▶ Principal
Principal ── pedido.excluido ──────▶ Estoque
```

Quando o pedido é cancelado (sem estoque, pagamento recusado ou exclusão pelo
usuário), o Principal publica `pedido.excluido` e o Estoque devolve o que tinha
reservado.

Exchange `promocoes`, tipo `topic`:

```
Promoções ── promocao.categoria.A|B|C ─▶ C1 (binding A e B) e C2 (binding #)
```

Cada processo tem a sua fila durável: `principal.status`, `estoque.pedidos`,
`pagamento.estoque_ok`, `entrega.pagamentos`, `promocoes.c1` e `promocoes.c2`.

## Catálogo

O catálogo é fixo. O estoque fica em memória no Estoque: reiniciar o processo
volta às quantidades iniciais.

| Código | Produto | Categoria | Preço | Estoque inicial |
|---|---|---|---|---|
| P001 | Notebook Gamer 16GB | A | R$ 4.500,00 | 10 |
| P002 | Mouse sem fio | A | R$ 150,00 | 50 |
| P003 | Teclado mecânico | A | R$ 320,00 | 25 |
| P004 | Cafeteira expresso | B | R$ 280,00 | 15 |
| P005 | Liquidificador 900W | B | R$ 190,00 | 20 |
| P006 | Air Fryer 5L | B | R$ 450,00 | 8 |
| P007 | Camiseta algodão | C | R$ 79,90 | 100 |
| P008 | Tênis de corrida | C | R$ 350,00 | 30 |
| P009 | Mochila 30L | C | R$ 210,00 | 12 |

## Funcionalidades e como reproduzir

Os passos assumem o ambiente de [Como rodar](#como-rodar) no ar.

### 1. Ver a vitrine

No menu, opção `1`. Lista os produtos com código, categoria e preço. A
quantidade em estoque não aparece: ela pertence ao Estoque, e o Principal só
descobre se o pedido cabe quando o evento de resposta chega.

### 2. Fazer um pedido que chega até o envio

1. Opção `2`.
2. Informe o produto (número da linha ou código, ex. `1` ou `P001`) e a
   quantidade. Repita para adicionar mais itens; produto em branco fecha o pedido.
3. Confirme com `s`.

Em poucos segundos o status muda sozinho na tela:

```
  >> PED-3f9a1c → estoque reservado (itens reservados, seguindo para o pagamento)
  >> PED-3f9a1c → pagamento aprovado (transação TX-9b2e4f1a7c30, R$ 4500.00)
  >> PED-3f9a1c → enviado (nota 482913577, Correios, rastreio CR731904552BR)
```

O Pagamento aprova 70% dos pedidos por sorteio. Para garantir a aprovação, suba
com `TAXA_APROVACAO=1 make pagamento`.

### 3. Pedido cancelado por falta de estoque

Peça mais do que existe. O Air Fryer (`P006`) começa com 8 unidades, então peça 9:

```
  >> PED-a1b2c3 → cancelado: sem estoque (estoque insuficiente de Air Fryer 5L: pedidas 9, disponíveis 8)
```

Se um item não cabe, nenhum item do pedido é reservado.

### 4. Pedido cancelado no pagamento, com devolução ao estoque

Suba o Pagamento recusando tudo com `TAXA_APROVACAO=0 make pagamento` e faça um
pedido qualquer:

```
  >> PED-d4e5f6 → estoque reservado (itens reservados, seguindo para o pagamento)
  >> PED-d4e5f6 → cancelado: pagamento recusado (cartão expirado)
```

O terminal do Estoque mostra a devolução:

```
pedido PED-d4e5f6 excluído (pagamento recusado: cartão expirado): 1 item(ns) devolvidos. Estoque agora: P001=10 ...
```

### 5. Excluir um pedido

Só dá para excluir pedido que ainda não terminou (enviado ou cancelado, não).
Como o fluxo completo leva poucos segundos, o jeito simples é travar o pedido
no meio:

1. Pare o Pagamento (`Ctrl+C`).
2. Faça um pedido. Ele fica em `estoque reservado`.
3. Opção `3`, escolha o pedido e confirme com `s`.

O status vira `cancelado pelo usuário` e o terminal do Estoque mostra os itens
devolvidos.

Ao religar o Pagamento, ele ainda processa a mensagem desse pedido que ficou na
fila, porque o Pagamento não escuta `pedido.excluido`. Para evitar, esvazie a
fila antes no painel: *Queues* → `pagamento.estoque_ok` → *Purge Messages*.

### 6. Consultar pedidos e histórico

Opção `4`. Mostra os pedidos da sessão com o status atual. Escolhendo um número,
mostra itens, total e cada mudança de status com o horário.

Os pedidos ficam em memória no Principal: fechar o menu apaga a lista.

### 7. Promoções por categoria

Com `c1`, `c2` e `promocoes` no ar, o gerador sorteia um produto, aplica um
desconto aleatório e publica com a routing key da categoria do produto. Nos
terminais dos consumidores:

```
Nova promocao recebida! Categoria: B
Dados: {ProdutoID:P006 Nome:Air Fryer 5L Categoria:B PrecoDe:450 PrecoPor:283.5 Desconto:37 ValidaAte:2026-09-17T19:40:12Z}
```

- C2 recebe **todas** as promoções.
- C1 recebe só as das categorias **A e B**. Promoção da categoria C aparece no C2
  e não no C1.

A diferença está só nas binding keys: C1 se vincula a `promocao.categoria.A` e
`promocao.categoria.B`; C2 usa `promocao.categoria.#`, que casa com qualquer
categoria. A promoção é só um aviso, não muda o preço usado nos pedidos.

### 8. Assinatura digital dos eventos

Todo evento publicado carrega no envelope o campo `signature`: o hash SHA-256 do
evento (sem esse campo) assinado com a chave privada RSA de quem publicou. Quem
consome confere com a chave pública do `produtor` declarado no envelope e
descarta o evento se não bater.

**Ver funcionando.** Os logs de cada processo mostram quem assinou o que publicou
e só registram o recebimento depois da validação:

```
→ publicado pedido.criado (evento 3f9a1c2b8e4d7601, assinado por principal)
← recebido pedido.criado (evento 3f9a1c2b8e4d7601, produtor principal)
```

O log do Principal vai para `principal.log`, para não atrapalhar o menu. Acompanhe
com `tail -f principal.log`.

**Ver um evento falso sendo descartado.**

1. No painel do RabbitMQ, abra *Exchanges* → `eCommerce` → *Publish message*.
2. Em *Routing key*, `pedido.criado`. Em *Payload*:

   ```json
   {"id":"falso","tipo":"pedido.criado","produtor":"principal","timestamp":"agora","dados":{"pedido_id":"PED-FALSO","itens":[]},"signature":"AAAA"}
   ```

3. Clique em *Publish message*. O terminal do Estoque mostra:

   ```
   assinatura inválida no evento pedido.criado vindo de "principal", DESCARTADO: ...
   ```

Para comparar, suba o Estoque com `VERIFICAR_ASSINATURA=off make estoque` e
publique a mesma mensagem: agora ela é aceita e processada (e recusada por outro
motivo, `pedido sem itens`).

### 9. Mensagem repetida não baixa nem cobra duas vezes

Estoque e Pagamento guardam os pedidos já tratados e ignoram a reentrega. Com o
Pagamento aprovando tudo (`TAXA_APROVACAO=1 make pagamento`), publique o mesmo
pedido duas vezes:

```bash
go run ./cmd/teste-publicador -pedido PED-REPETIDO -itens P001:1
go run ./cmd/teste-publicador -pedido PED-REPETIDO -itens P001:1
```

Na segunda vez, o Estoque loga `já estava reservado, reentrega ignorada` (o saldo
de `P001` não muda) e o Pagamento loga `já foi cobrado, reentrega ignorada`.

## Ferramentas de desenvolvimento

**`make observador`** imprime todo evento que passa pela exchange `eCommerce`
(tipo, produtor e dados). Serve para ver o fluxo inteiro num terminal só.

**`teste-publicador`** publica `pedido.criado` ou `pedido.excluido` assinado, sem
passar pelo menu:

```bash
go run ./cmd/teste-publicador -itens P001:2,P004:1   # pedido que cabe no estoque
go run ./cmd/teste-publicador -itens P006:99         # pedido sem estoque
go run ./cmd/teste-publicador -tipo pedido.excluido -pedido PED-001 -motivo teste
```

| Flag | Padrão | Para que serve |
|---|---|---|
| `-tipo` | `pedido.criado` | `pedido.criado` ou `pedido.excluido` |
| `-pedido` | gerado pelo horário | ID do pedido |
| `-itens` | `P001:1` | itens no formato `PRODUTO:QTD,PRODUTO:QTD` |
| `-cliente` | `cliente-teste` | nome do cliente |
| `-motivo` | `cancelado no teste` | motivo do `pedido.excluido` |
| `-como` | `principal` | serviço cuja chave assina o evento |

## Testes

`make teste` roda `go test ./...`. Não precisa de broker: os serviços recebem o
publicador por interface e os testes injetam um falso. Cobrem as regras do
Estoque, do Pagamento, do menu e dos status do Principal, e a assinatura (evento
alterado ou validado com a chave de outro serviço é recusado).

## Chaves

```
chaves/
  <serviço>/
    privada.pem   assina o que o serviço publica
    publicas/     chaves públicas de todos, para validar o que chega
```

As chaves estão versionadas para todos usarem o mesmo conjunto. `make chaves`
preserva as existentes; `go run ./cmd/gerar-chaves -forcar` regera tudo.

C1 e C2 só consomem, então usam apenas `chaves/consumidores/publicas/`. Essa
pasta **não** é criada pelo `gerar-chaves`: se regerar com `-forcar`, copie as
públicas novas para lá, senão os consumidores descartam todas as promoções.

```bash
cp chaves/promocoes/publicas/*.pem chaves/consumidores/publicas/
```

## Variáveis de ambiente

| Variável | Padrão | Efeito |
|---|---|---|
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | endereço do broker |
| `CHAVES_DIR` | `chaves` | pasta raiz das chaves |
| `VERIFICAR_ASSINATURA` | ligada | `off` faz os consumidores aceitarem eventos sem validar |
| `TAXA_APROVACAO` | `0.7` | fração de pagamentos aprovados, de 0 a 1 |

Outros comandos: `make broker-parar`, `make broker-logs`, `make build` (binários
em `bin/`) e `make limpar`.
