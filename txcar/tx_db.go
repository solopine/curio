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
	CarKey    string
	PieceCid  string
	PieceSize int64
	CarSize   int64
	Version   int
}

func AddTxPieceToDb(ctx context.Context, db *harmonydb.DB, txPiece txcar.TxPiece) error {
	_, err := db.Exec(ctx, `
insert into tx_car_pieces(piece_cid, car_key, piece_size, car_size, version) VALUES($1, $2, $3, $4, $5)
ON CONFLICT(piece_cid) 
DO UPDATE SET
  car_key = EXCLUDED.car_key,
  piece_size = EXCLUDED.piece_size,
  car_size = EXCLUDED.car_size,
  version = EXCLUDED.version;`, txPiece.PieceCid.String(), txPiece.CarKey.String(), txPiece.PieceSize, txPiece.CarSize, txPiece.Version)
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

	return &txcar.TxPiece{
		CarKey:    key,
		PieceCid:  pieceCid,
		PieceSize: abi.PaddedPieceSize(row.PieceSize),
		CarSize:   abi.UnpaddedPieceSize(row.CarSize),
		Version:   txcar.Version(row.Version),
	}, nil
}

func GetTxPieceFromDbByPiece(ctx context.Context, db *harmonydb.DB, pieceCid cid.Cid) (*txcar.TxPiece, error) {
	var rows []txPieceRow
	err := db.Select(ctx, &rows, "SELECT piece_cid,car_key,piece_size,car_size,version FROM tx_car_pieces WHERE piece_cid = $1", pieceCid.String())
	if err != nil {
		return nil, xerrors.Errorf("GetTxPieceFromDbByPiece: %w", err)
	}
	return parseFromTxPieceRows(rows)
}

func GetTxPieceFromDbBySector(ctx context.Context, db *harmonydb.DB, sectorId abi.SectorID) (*txcar.TxPiece, error) {
	var rows []txPieceRow
	err := db.Select(ctx, &rows, `select tcp.piece_cid,tcp.car_key,tcp.piece_size,tcp.car_size, tcp.version  
from tx_car_pieces tcp 
    join sectors_meta_pieces smp on smp.piece_cid=tcp.piece_cid 
where smp.sp_id=$1 and smp.sector_num=$2;`, sectorId.Miner, sectorId.Number)

	if err != nil {
		return nil, xerrors.Errorf("IsAndGetTxCarInfo: %w", err)
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
