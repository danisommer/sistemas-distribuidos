### Como rodar

Pré-requisitos: Go 1.22+ e Docker.

```bash
# 1. broker (painel em http://localhost:15672, usuário e senha guest)
make broker

# 2. chaves RSA, uma vez só
make chaves

# 3. um microsserviço por terminal
make estoque
make pagamento

# 4. o menu, no seu próprio terminal
make principal
```

O Principal é a interface do sistema. Ele mostra a vitrine, monta o pedido,
publica `pedido.criado` e vai imprimindo as mudanças de status conforme os
eventos de resposta chegam:

```
  >> PED-3f9a1c → estoque reservado (itens reservados, seguindo para o pagamento)
  >> PED-3f9a1c → pagamento aprovado (transação TX-9b2e4f, R$ 9.000,00)
```

O log do broker no Principal não vai para a tela, senão brigaria com o menu.
Ele fica em `principal.log`. Para acompanhar o tráfego ao vivo, abra outro
terminal e rode `tail -f principal.log`.

Duas ferramentas de desenvolvimento ajudam a testar partes isoladas, sem
passar pelo menu:

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
