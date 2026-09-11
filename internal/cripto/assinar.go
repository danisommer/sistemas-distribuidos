package cripto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"ecommerce/internal/evento"
)

// Assinar executa os dois primeiros passos exigidos pelo enunciado: gera o
// hash SHA-256 do conteúdo do evento e assina esse hash com a chave privada
// do microsserviço produtor.
//
// O esquema é o RSASSA-PKCS#1 v1.5, o mesmo do pkcs1_v1_5 do PyCryptodome e
// do SHA256withRSA do Java. O retorno vem em base64 porque o envelope é JSON
// e não comporta bytes crus.
func Assinar(privada *rsa.PrivateKey, conteudo []byte) (string, error) {
	if privada == nil {
		return "", errors.New("chave privada não carregada")
	}

	// 1. hash do conteúdo do evento
	resumo := sha256.Sum256(conteudo)

	// 2. assinatura do hash com a chave privada
	assinatura, err := rsa.SignPKCS1v15(rand.Reader, privada, crypto.SHA256, resumo[:])
	if err != nil {
		return "", fmt.Errorf("assinando evento: %w", err)
	}

	return base64.StdEncoding.EncodeToString(assinatura), nil
}

// AssinarEnvelope é o terceiro passo: monta os bytes canônicos do evento,
// assina e guarda o resultado no campo Signature do próprio envelope.
//
// O envelope entra por ponteiro justamente porque é ele que sai alterado.
func AssinarEnvelope(privada *rsa.PrivateKey, env *evento.Envelope) error {
	conteudo, err := env.BytesCanonicos()
	if err != nil {
		return err
	}

	assinatura, err := Assinar(privada, conteudo)
	if err != nil {
		return err
	}

	// 3. assinatura digital no campo Signature do envelope
	env.Signature = assinatura
	return nil
}
