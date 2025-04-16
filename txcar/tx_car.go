package txcar

import (
	"context"
	"encoding/gob"
	"github.com/filecoin-project/curio/lib/pieceprovider"
	"github.com/filecoin-project/curio/lib/storiface"
	"github.com/filecoin-project/curio/market/indexstore"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	lpiece "github.com/filecoin-project/lotus/storage/pipeline/piece"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/solopine/txcar/txcar"
	"golang.org/x/xerrors"
	"os"
)

var log = logging.Logger("txcar")

const TxHttpPort = 24001

func ParseTxPiece(path string) (*txcar.TxPiece, error) {
	p, err := txcar.BoostPathParser.Parse(path)
	return &p, err
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
	txPieceStr := txcar.BoostPathParser.DeParse(txPiece)
	return os.WriteFile(filePath, []byte(txPieceStr), 0755)
}

func ParseRecordsForTxPiece(ctx context.Context, pieceProvider *pieceprovider.PieceProvider, spId abi.ActorID, sectorNumber abi.SectorNumber, proofType abi.RegisteredSealProof, pieceCid cid.Cid) ([]indexstore.Record, error) {
	log.Infow("IPNITask.parseRecordsForTxPiece", "minerAddr", spId.String(), "task.Sector", sectorNumber, "pieceCid", pieceCid)

	reader, err := pieceProvider.TxReadUnsealed(ctx, storiface.SectorRef{
		ID: abi.SectorID{
			Miner:  spId,
			Number: sectorNumber,
		},
		ProofType: proofType,
	})
	if err != nil {
		return nil, err
	}

	dec := gob.NewDecoder(reader)

	// 1. read txpiece
	var txPiece txcar.TxPiece
	if err := dec.Decode(&txPiece); err != nil {
		return nil, err
	}
	if txPiece.PieceCid != pieceCid {
		return nil, xerrors.Errorf("pieceCid error. pieceCid in file: %s, pieceCid expected: %s", txPiece.PieceCid, pieceCid)
	}

	// 2. read txRecs
	var txRecs []txcar.TxBlockRecord
	if err := dec.Decode(&txRecs); err != nil {
		return nil, err
	}

	recs := make([]indexstore.Record, 0, len(txRecs))
	for _, txRec := range txRecs {
		recs = append(recs, indexstore.Record{
			Cid:    txRec.Cid,
			Offset: txRec.Offset,
			Size:   txRec.Size,
		})
	}

	return recs, nil
}
