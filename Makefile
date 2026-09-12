.PHONY: ajuda broker broker-parar broker-logs chaves estoque pagamento principal entrega promocoes c1 c2 observador teste build limpar

ajuda:
	@echo "Broker"
	@echo "  make broker          sobe o RabbitMQ (painel em http://localhost:15672, guest/guest)"
	@echo "  make broker-parar    derruba o RabbitMQ"
	@echo "  make broker-logs     acompanha o log do broker"
	@echo ""
	@echo "Chaves"
	@echo "  make chaves          gera os pares RSA e distribui as chaves publicas"
	@echo ""
	@echo "Microsservicos (um por terminal)"
	@echo "  make estoque         sobe o microsservico Estoque"
	@echo "  make pagamento       sobe o microsservico Pagamento"
	@echo "  make principal       sobe o microsservico Principal (menu do sistema)"
	@echo "  make entrega         sobe o microsservico Entrega        (parte da dupla)"
	@echo "  make promocoes       sobe o microsservico Promocoes      (parte da dupla)"
	@echo "  make c1              sobe o consumidor C1 de promocoes   (parte da dupla)"
	@echo "  make c2              sobe o consumidor C2 de promocoes   (parte da dupla)"
	@echo ""
	@echo "Desenvolvimento"
	@echo "  make observador      imprime todos os eventos da exchange eCommerce"
	@echo "  make teste           roda os testes"
	@echo "  make build           compila tudo em ./bin"
	@echo "  make limpar          apaga os binarios"

broker:
	docker compose up -d
	@echo "RabbitMQ subindo. Painel: http://localhost:15672 (guest / guest)"

broker-parar:
	docker compose down

broker-logs:
	docker compose logs -f rabbitmq

chaves:
	go run ./cmd/gerar-chaves

estoque:
	go run ./cmd/estoque

pagamento:
	go run ./cmd/pagamento

principal:
	go run ./cmd/principal

entrega:
	go run ./cmd/entrega

promocoes:
	go run ./cmd/promocoes

c1:
	go run ./cmd/consumidor-c1

c2:
	go run ./cmd/consumidor-c2

observador:
	go run ./cmd/observador

teste:
	go test ./...

build:
	go build -o bin/ ./cmd/...
	@ls bin/

limpar:
	rm -rf bin/
