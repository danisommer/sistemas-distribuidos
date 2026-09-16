package mensageria

import (
	"log"
	"os"
)

// registro é para onde vão as mensagens desta camada: conexão, publicação,
// recebimento e descarte de evento inválido.
var registro = log.New(os.Stderr, "", log.Ltime)

// DefinirLog troca o destino dos logs da mensageria.
func DefinirLog(destino *log.Logger) {
	if destino != nil {
		registro = destino
	}
}
