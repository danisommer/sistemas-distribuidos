// Comando gerar-chaves cria um par de chaves RSA para cada microsserviço e
// espalha as chaves públicas nas pastas de todos eles, montando o layout que
// o pacote cripto espera.
//
//	go run ./cmd/gerar-chaves
//
// Por padrão ele preserva as chaves que já existem, então rodar de novo é
// seguro. Use -forcar para regerar tudo do zero.
package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
)

func main() {
	log.SetFlags(0)

	raiz := flag.String("raiz", "chaves", "pasta raiz onde as chaves são gravadas")
	forcar := flag.Bool("forcar", false, "regera as chaves mesmo que já existam")
	flag.Parse()

	if err := executar(*raiz, *forcar); err != nil {
		log.Fatalf("erro: %v", err)
	}
}

func executar(raiz string, forcar bool) error {
	publicas := make(map[string]*rsa.PublicKey, len(evento.Servicos))

	for _, servico := range evento.Servicos {
		caminho := filepath.Join(raiz, servico, cripto.ArquivoPrivada)

		privada, err := obterPrivada(caminho, forcar)
		if err != nil {
			return err
		}
		publicas[servico] = &privada.PublicKey
	}

	for _, dono := range evento.Servicos {
		pasta := filepath.Join(raiz, dono, cripto.PastaPublicas)

		for outro, publica := range publicas {
			destino := filepath.Join(pasta, outro+cripto.ExtensaoPublica)
			if err := cripto.SalvarPublica(destino, publica); err != nil {
				return err
			}
		}
		log.Printf("%s: %d chaves públicas em %s", dono, len(publicas), pasta)
	}

	fmt.Println()
	fmt.Println("Chaves prontas. Confira com:")
	fmt.Printf("  find %s -type f | sort\n", raiz)
	return nil
}

// obterPrivada devolve a chave privada do serviço, gerando uma nova só se
// ainda não houver arquivo ou se a regeração tiver sido pedida.
func obterPrivada(caminho string, forcar bool) (*rsa.PrivateKey, error) {
	if !forcar {
		if _, err := os.Stat(caminho); err == nil {
			privada, err := cripto.CarregarPrivada(caminho)
			if err != nil {
				return nil, fmt.Errorf("%s já existe mas não pôde ser lido: %w (use -forcar para regerar)", caminho, err)
			}
			log.Printf("%s: chave privada preservada", caminho)
			return privada, nil
		}
	}

	privada, err := cripto.GerarPar()
	if err != nil {
		return nil, err
	}
	if err := cripto.SalvarPrivada(caminho, privada); err != nil {
		return nil, err
	}
	log.Printf("%s: chave privada RSA de %d bits gerada", caminho, cripto.TamanhoChaveBits)
	return privada, nil
}
