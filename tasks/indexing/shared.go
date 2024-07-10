package indexing

import (
	"context"
	"encoding/gob"
	"github.com/filecoin-project/curio/lib/pieceprovider"
	"github.com/filecoin-project/curio/lib/storiface"
	"github.com/filecoin-project/curio/market/indexstore"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/solopine/txcar/txcar"
	"golang.org/x/xerrors"
)

func parseRecordsForTxPiece(ctx context.Context, pieceProvider *pieceprovider.PieceProvider, spId abi.ActorID, sectorNumber abi.SectorNumber, proofType abi.RegisteredSealProof, pieceCid cid.Cid) ([]indexstore.Record, error) {
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
