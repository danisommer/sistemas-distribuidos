package mensageria

import (
	"log"
	"os"
)

// registro é para onde vão as mensagens desta camada: conexão, publicação,
// recebimento e descarte de evento inválido.
//
// Ele é trocável porque o microsserviço Principal tem um menu no terminal, e
// log do broker no meio do menu deixa a tela ilegível. O Principal manda
// isto para um arquivo e imprime na tela só as mudanças de status dos
// pedidos. Os outros microsserviços não mexem e continuam logando na tela.
var registro = log.New(os.Stderr, "", log.Ltime)

// DefinirLog troca o destino dos logs da mensageria.
func DefinirLog(destino *log.Logger) {
	if destino != nil {
		registro = destino
	}
}
