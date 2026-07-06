package txcarlib

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/solopine/txcar/txcar"
)

var log = logging.Logger("txcar")

const TxHttpPort = 24001

func ParseTxPieceFromUrlPath(path string) (*txcar.TxPiece, error) {
	p, err := txcar.BoostPathParser.Parse(path)
	return &p, err
}

func CreateFakeUnsealedFile(filePath string, txPiece txcar.TxPiece) error {
	txPieceStr := txcar.BoostPathParser.DeParse(txPiece)
	return os.WriteFile(filePath, []byte(txPieceStr), 0755)
}
