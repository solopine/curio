package txcar

import (
	addr "github.com/filecoin-project/go-address"
	lpiece "github.com/filecoin-project/lotus/storage/pipeline/piece"
	"github.com/solopine/txcar/txcar"
	"github.com/solopine/txcar/txcar/parser"
	"os"
)

func ParseTxPiece(path string) (*txcar.TxPiece, error) {
	txPieceParser := parser.NewBoostPathParser(path)
	return txPieceParser.Parse()
}

func ParseTxPieceFromDeal(deal lpiece.PieceDealInfo) (*txcar.TxPiece, error) {

	if deal.PieceActivationManifest == nil {
		return nil, nil
	}

	if len(deal.PieceActivationManifest.Notify) != 1 {
		return nil, nil
	}
	notify := deal.PieceActivationManifest.Notify[0]
	if notify.Address != addr.Undef {
		return nil, nil
	}
	if notify.Payload == nil {
		return nil, nil
	}
	txCarInfoStr := string(notify.Payload)
	return ParseTxPiece(txCarInfoStr)
}

func CreateFakeUnsealedFile(filePath string, txPiece txcar.TxPiece) error {
	txPieceStr := parser.DeParse(txPiece)
	return os.WriteFile(filePath, []byte(txPieceStr), 0755)
}