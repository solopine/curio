package txcar

import (
	"context"
	"fmt"
	"github.com/filecoin-project/curio/harmony/harmonydb"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/solopine/txcar/txcar"
	"golang.org/x/xerrors"
)

type txPieceRow struct {
	CarKey     string
	Version    int
	PieceCid   string
	PayloadCid string
	PieceSize  int64
	CarSize    int64
}

func AddTxPieceToDb(ctx context.Context, db *harmonydb.DB, txPiece txcar.TxPiece) error {
	_, err := db.Exec(ctx, `
insert into tx_car(car_key, version, piece_cid, payload_cid, piece_size, car_size, create_time, update_time) VALUES($1, $2, $3, $4, $5, $5, current_timestamp, current_timestamp)
ON CONFLICT(piece_cid) 
DO UPDATE SET
  car_key = EXCLUDED.car_key,
  version = EXCLUDED.version,
  piece_cid = EXCLUDED.piece_cid,
  payload_cid = EXCLUDED.payload_cid,
  piece_size = EXCLUDED.piece_size,
  car_size = EXCLUDED.car_size,
  update_time = EXCLUDED.update_time;
`, txPiece.CarKey.String(), txPiece.Version, txPiece.PieceCid.String(), txPiece.PayloadCid.String(), txPiece.PieceSize, txPiece.CarSize)
	return err
}

func parseFromTxPieceRows(rows []txPieceRow) (*txcar.TxPiece, error) {

	if len(rows) > 1 {
		return nil, xerrors.Errorf("parseFromTxPieceRows: rows >1")
	}
	if len(rows) == 0 {
		// not found, sector is not a txpiece
		return nil, nil
	}

	row := rows[0]
	key, err := uuid.Parse(row.CarKey)
	if err != nil {
		return nil, xerrors.Errorf("parseFromTxPieceRow: uuid.Parse. CarKey:%s", row.CarKey)
	}

	//
	pieceCid, err := cid.Decode(row.PieceCid)
	if err != nil {
		return nil, xerrors.Errorf("parseFromTxPieceRow: pieceCid Parse. PieceCid:%s", row.PieceCid)
	}

	//
	payloadCid, err := cid.Decode(row.PayloadCid)
	if err != nil {
		return nil, xerrors.Errorf("parseFromTxPieceRow: payloadCid Parse. PayloadCid:%s", row.PayloadCid)
	}

	return &txcar.TxPiece{
		Version:    txcar.Version(row.Version),
		CarKey:     key,
		PieceCid:   pieceCid,
		PayloadCid: payloadCid,
		PieceSize:  abi.PaddedPieceSize(row.PieceSize),
		CarSize:    abi.UnpaddedPieceSize(row.CarSize),
	}, nil
}

func GetTxPieceFromDbByPiece(ctx context.Context, db *harmonydb.DB, pieceCid cid.Cid) (*txcar.TxPiece, error) {
	var rows []txPieceRow
	err := db.Select(ctx, &rows, "SELECT car_key, version, piece_cid, payload_cid, piece_size, car_size FROM tx_car WHERE piece_cid = $1", pieceCid.String())
	if err != nil {
		return nil, xerrors.Errorf("GetTxPieceFromDbByPiece: %w", err)
	}
	return parseFromTxPieceRows(rows)
}

func IsTxPiece(ctx context.Context, db *harmonydb.DB, pieceCid cid.Cid) (bool, error) {
	txPiece, err := GetTxPieceFromDbByPiece(ctx, db, pieceCid)
	if err != nil {
		return false, err
	}
	return txPiece != nil, nil
}

func IsTxPieceStr(ctx context.Context, db *harmonydb.DB, pieceCidStr string) (bool, error) {
	pieceCid, err := cid.Decode(pieceCidStr)
	if err != nil {
		return false, fmt.Errorf("tx pieceCidStr is invalid:%s", pieceCidStr)
	}
	return IsTxPiece(ctx, db, pieceCid)
}
