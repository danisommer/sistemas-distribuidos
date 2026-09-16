package cripto

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"ecommerce/internal/evento"
)

// ============================================================================
//  PARTE DA DUPLA — validação da assinatura digital
//
//  Só falta o corpo de Verificar, logo abaixo. O resto do caminho já está
//  pronto: o consumidor em internal/mensageria/consumidor.go chama
//  VerificarEnvelope a cada mensagem e descarta o evento se vier erro.
//
//  Para implementar, espelhe cripto.Assinar (assinar.go):
//
//    1. assinatura, err := base64.StdEncoding.DecodeString(assinaturaBase64)
//    2. resumo := sha256.Sum256(conteudo)
//    3. rsa.VerifyPKCS1v15(publica, crypto.SHA256, resumo[:], assinatura)
//    4. devolva nil se bater, ou o erro recebido caso contrário
//
//  Depois de implementar, apague ErrNaoImplementado e a linha que o devolve.
// ============================================================================

// ErrNaoImplementado marca que a validação ainda não foi escrita. Enquanto
// ele existir, rodar com VERIFICAR_ASSINATURA=on faz todo evento ser
// descartado, o que é o comportamento correto: sem validação não dá para
// confiar em nada que chega.

// Verificar confere se assinaturaBase64 foi mesmo produzida pela chave
// privada correspondente a publica, sobre o conteúdo informado.
//
// Devolve nil quando a assinatura é válida, o que confirma de uma vez a
// autenticidade (veio de quem diz ter vindo) e a integridade (não foi
// alterada no caminho) da mensagem.
func Verificar(publica *rsa.PublicKey, conteudo []byte, assinaturaBase64 string) error {
	if publica == nil {
		return errors.New("chave pública não carregada")
	}
	if assinaturaBase64 == "" {
		return errors.New("evento sem assinatura")
	}

	assinaturaBytes, err := base64.StdEncoding.DecodeString(assinaturaBase64)

	if err != nil {
		return fmt.Errorf("falha ao decodificar assinatura: %w", err)
	}

	resumo := sha256.Sum256(conteudo)

	if err := rsa.VerifyPKCS1v15(publica, crypto.SHA256, resumo[:], assinaturaBytes); err != nil {
		return fmt.Errorf("falha ao verificar assinatura: %w", err)
	}

	return nil
}

// VerificarEnvelope é o caminho completo da validação de um evento recebido:
// pega a chave pública do produtor declarado, remonta os mesmos bytes que
// foram assinados e chama Verificar.
func VerificarEnvelope(chaveiro *Chaveiro, env evento.Envelope) error {
	// 1. obter a chave pública do microsserviço produtor
	publica, err := chaveiro.PublicaDe(env.Produtor)
	if err != nil {
		return err
	}

	// 2. remontar exatamente o conteúdo que o produtor assinou
	conteudo, err := env.BytesCanonicos()
	if err != nil {
		return err
	}

	// 3. verificar a assinatura digital
	return Verificar(publica, conteudo, env.Signature)
}
