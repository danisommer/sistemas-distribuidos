package consumidor

import (
	"context"
	"ecommerce/internal/evento"
	"fmt"
)

type Servico struct{}

func NovoServico() *Servico {
	return &Servico{}
}

func (s *Servico) Tratar(ctx context.Context, env evento.Envelope) error {
	var dadosPromocao evento.DadosPromocao

	if err := env.DecodificarDados(&dadosPromocao); err != nil {
		return fmt.Errorf("Falha ao lidar com nova promocao. %w", err)
	}

	fmt.Printf("Nova promocao recebida! Categoria: %s\n", dadosPromocao.Categoria)
	fmt.Printf("Dados: %+v\n\n", dadosPromocao)

	return nil
}
