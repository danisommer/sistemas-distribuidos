// Package cripto concentra a geração, o armazenamento e o uso dos pares de
// chaves RSA que assinam e validam os eventos.
//
// Divisão do trabalho na dupla:
//   - assinar.go   → geração da assinatura digital
//   - verificar.go → validação da assinatura
//
// Layout em disco, montado pelo comando cmd/gerar-chaves:
//
//	chaves/
//	  estoque/
//	    privada.pem            ← só este processo pode ler
//	    publicas/
//	      principal.pem        ← chave pública dos demais microsserviços
//	      pagamento.pem
//	      entrega.pem
//	      promocoes.pem
//	      estoque.pem
package cripto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TamanhoChaveBits é o tamanho das chaves RSA geradas. 2048 bits é o mínimo
// considerado seguro hoje e é rápido o bastante para assinar cada evento.
const TamanhoChaveBits = 2048

// Nomes dos arquivos dentro da pasta de cada microsserviço.
const (
	ArquivoPrivada  = "privada.pem"
	PastaPublicas   = "publicas"
	ExtensaoPublica = ".pem"
)

// GerarPar cria um novo par de chaves RSA.
func GerarPar() (*rsa.PrivateKey, error) {
	chave, err := rsa.GenerateKey(rand.Reader, TamanhoChaveBits)
	if err != nil {
		return nil, fmt.Errorf("gerando par de chaves RSA: %w", err)
	}
	return chave, nil
}

// SalvarPrivada grava a chave privada em PEM, no formato PKCS#8, com
// permissão de leitura apenas para o dono.
func SalvarPrivada(caminho string, chave *rsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(chave)
	if err != nil {
		return fmt.Errorf("serializando chave privada: %w", err)
	}
	bloco := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return gravar(caminho, pem.EncodeToMemory(bloco), 0o600)
}

// SalvarPublica grava a chave pública em PEM, no formato PKIX. É o formato
// que Python e Java também leem, caso a dupla queira conferir a assinatura
// com outra ferramenta.
func SalvarPublica(caminho string, chave *rsa.PublicKey) error {
	der, err := x509.MarshalPKIXPublicKey(chave)
	if err != nil {
		return fmt.Errorf("serializando chave pública: %w", err)
	}
	bloco := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return gravar(caminho, pem.EncodeToMemory(bloco), 0o644)
}

// CarregarPrivada lê uma chave privada RSA de um arquivo PEM.
func CarregarPrivada(caminho string) (*rsa.PrivateKey, error) {
	der, err := lerBlocoPEM(caminho)
	if err != nil {
		return nil, err
	}

	// Aceita PKCS#8 (o formato que gravamos) e PKCS#1, para o caso de a
	// chave ter sido gerada por fora, com openssl antigo.
	if qualquer, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := qualquer.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%s: a chave não é RSA", caminho)
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("%s: chave privada inválida: %w", caminho, err)
	}
	return rsaKey, nil
}

// CarregarPublica lê uma chave pública RSA de um arquivo PEM.
func CarregarPublica(caminho string) (*rsa.PublicKey, error) {
	der, err := lerBlocoPEM(caminho)
	if err != nil {
		return nil, err
	}
	qualquer, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("%s: chave pública inválida: %w", caminho, err)
	}
	rsaKey, ok := qualquer.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s: a chave não é RSA", caminho)
	}
	return rsaKey, nil
}

// Chaveiro é o conjunto de chaves de um microsserviço: a própria chave
// privada, para assinar o que publica, e as chaves públicas de todos os
// outros, para validar o que consome.
type Chaveiro struct {
	Servico  string
	Privada  *rsa.PrivateKey
	Publicas map[string]*rsa.PublicKey
}

// CarregarChaveiro lê a pasta chaves/<servico> inteira.
func CarregarChaveiro(raiz, servico string) (*Chaveiro, error) {
	base := filepath.Join(raiz, servico)

	privada, err := CarregarPrivada(filepath.Join(base, ArquivoPrivada))
	if err != nil {
		return nil, fmt.Errorf("chaveiro de %s: %w (rode: go run ./cmd/gerar-chaves)", servico, err)
	}

	publicas, err := lerChaveiroPublico(raiz, servico)

	if err != nil {
		return nil, fmt.Errorf("falha ao ler chaveiro publico de %s: %w (rode: go run ./cmd/gerar-chaves)", servico, err)
	}

	return &Chaveiro{Servico: servico, Privada: privada, Publicas: publicas}, nil
}

func CarregarChaveiroPublico(raiz, servico string) (*Chaveiro, error) {
	publicas, err := lerChaveiroPublico(raiz, servico)

	if err != nil {
		return nil, fmt.Errorf("falha ao ler chaveiro publico de %s: %w (rode: go run ./cmd/gerar-chaves)", servico, err)
	}

	return &Chaveiro{Servico: servico, Privada: nil, Publicas: publicas}, nil
}

func lerChaveiroPublico(raiz, servico string) (map[string]*rsa.PublicKey, error) {
	base := filepath.Join(raiz, servico)

	pastaPublicas := filepath.Join(base, PastaPublicas)
	entradas, err := os.ReadDir(pastaPublicas)
	if err != nil {
		return nil, fmt.Errorf("chaveiro de %s: lendo %s: %w", servico, pastaPublicas, err)
	}

	publicas := make(map[string]*rsa.PublicKey, len(entradas))
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ExtensaoPublica) {
			continue
		}
		dono := strings.TrimSuffix(e.Name(), ExtensaoPublica)
		chave, err := CarregarPublica(filepath.Join(pastaPublicas, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("chaveiro de %s: %w", servico, err)
		}
		publicas[dono] = chave
	}

	if len(publicas) == 0 {
		return nil, fmt.Errorf("chaveiro de %s: nenhuma chave pública em %s", servico, pastaPublicas)
	}

	return publicas, nil
}

// PublicaDe devolve a chave pública do microsserviço produtor. Um produtor
// desconhecido é erro: sem a chave pública não há como validar nada, e o
// evento tem de ser descartado.
func (c *Chaveiro) PublicaDe(servico string) (*rsa.PublicKey, error) {
	chave, ok := c.Publicas[servico]
	if !ok {
		return nil, fmt.Errorf("não tenho a chave pública de %q", servico)
	}
	return chave, nil
}

func gravar(caminho string, conteudo []byte, modo os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return fmt.Errorf("criando pasta de %s: %w", caminho, err)
	}
	if err := os.WriteFile(caminho, conteudo, modo); err != nil {
		return fmt.Errorf("gravando %s: %w", caminho, err)
	}
	return nil
}

func lerBlocoPEM(caminho string) ([]byte, error) {
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("lendo %s: %w", caminho, err)
	}
	bloco, _ := pem.Decode(conteudo)
	if bloco == nil {
		return nil, fmt.Errorf("%s: não contém um bloco PEM válido", caminho)
	}
	return bloco.Bytes, nil
}
