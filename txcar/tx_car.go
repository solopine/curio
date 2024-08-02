package txcar

import (
	"context"
	"fmt"
	"github.com/filecoin-project/curio/harmony/harmonydb"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	lpiece "github.com/filecoin-project/lotus/storage/pipeline/piece"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"os"
	"strconv"
	"strings"
)

const (
	TxCarKeyPrefix = "TX_CAR_KEY/"
	Separator      = "/"
)

type TxCarInfo struct {
	CarKey    uuid.UUID
	PieceCid  cid.Cid
	PieceSize int64
	CarSize   int64
}

func IsTxCarPath(path string) bool {
	if !strings.HasPrefix(path, TxCarKeyPrefix) {
		return false
	}

	keyStr := path[len(TxCarKeyPrefix):]
	_, err := uuid.Parse(keyStr)
	return err == nil
}

func ParseTxCarInfo(path string) (TxCarInfo, error) {
	var txCarInfo TxCarInfo
	if !strings.HasPrefix(path, TxCarKeyPrefix) {
		return txCarInfo, fmt.Errorf("path has no TxCarKeyPrefix:%s", path)
	}

	parts := strings.Split(path[len(TxCarKeyPrefix):], Separator)
	if len(parts) != 4 {
		return txCarInfo, fmt.Errorf("path is not valid TxCarKeyPrefix with 4 parts:%s", path)
	}

	//
	key, err := uuid.Parse(parts[0])
	if err != nil {
		return txCarInfo, fmt.Errorf("tx car key is invalid:%s", path)
	}
	txCarInfo.CarKey = key

	//
	pieceCid, err := cid.Decode(parts[1])
	if err != nil {
		return txCarInfo, fmt.Errorf("tx PieceCid is invalid:%s", path)
	}
	txCarInfo.PieceCid = pieceCid

	//
	pieceSize, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return txCarInfo, fmt.Errorf("tx PieceSize is invalid:%s", path)
	}
	txCarInfo.PieceSize = pieceSize

	//
	carSize, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return txCarInfo, fmt.Errorf("tx CarSize is invalid:%s", path)
	}
	txCarInfo.CarSize = carSize

	return txCarInfo, nil
}

func ParseTxCarInfoFromDeal(deal lpiece.PieceDealInfo) (TxCarInfo, error) {
	var txCarInfo TxCarInfo
	if deal.PieceActivationManifest == nil {
		return txCarInfo, fmt.Errorf("deal is not a tx car 0")
	}
	if deal.DealProposal != nil {
		return txCarInfo, fmt.Errorf("deal is not a tx car 1")
	}
	if len(deal.PieceActivationManifest.Notify) != 1 {
		return txCarInfo, fmt.Errorf("deal is not a tx car 2")
	}
	notify := deal.PieceActivationManifest.Notify[0]
	if notify.Address != addr.Undef {
		return txCarInfo, fmt.Errorf("deal is not a tx car 3")
	}
	if notify.Payload == nil {
		return txCarInfo, fmt.Errorf("deal is not a tx car 4")
	}
	txCarInfoStr := string(notify.Payload)
	return ParseTxCarInfo(txCarInfoStr)
}

func EncodeTxCarInfo(txCarInfo TxCarInfo) string {
	return TxCarKeyPrefix +
		txCarInfo.CarKey.String() + Separator +
		txCarInfo.PieceCid.String() + Separator +
		strconv.FormatInt(txCarInfo.PieceSize, 10) + Separator +
		strconv.FormatInt(txCarInfo.CarSize, 10)
}

func AddDbEntry(ctx context.Context, db *harmonydb.DB, txCarInfo TxCarInfo) error {
	_, err := db.Exec(ctx, `
insert into tx_car_pieces(piece_cid, car_key, piece_size, car_size) VALUES($1, $2, $3, $4)
ON CONFLICT(piece_cid) 
DO UPDATE SET
  car_key = EXCLUDED.car_key,
  piece_size = EXCLUDED.piece_size,
  car_size = EXCLUDED.car_size;`, txCarInfo.PieceCid.String(), txCarInfo.CarKey.String(), txCarInfo.PieceSize, txCarInfo.CarSize)
	return err
}

func IsAndGetTxCarInfo(ctx context.Context, db *harmonydb.DB, sectorId abi.SectorID) (TxCarInfo, error) {
	var nilTxCarInfo TxCarInfo
	var rows []struct {
		CarKey    string
		PieceCid  string
		PieceSize int64
		CarSize   int64
	}
	err := db.Select(ctx, &rows, `select tcp.piece_cid,tcp.car_key,tcp.piece_size,tcp.car_size  
from sectors_meta_pieces smp 
    join tx_car_pieces tcp on smp.piece_cid=tcp.piece_cid 
where smp.sp_id=$1 and smp.sector_num=$2;`, sectorId.Miner, sectorId.Number)

	if err != nil {
		return nilTxCarInfo, xerrors.Errorf("IsAndGetTxCarInfo: %w", err)
	}
	if len(rows) != 1 {
		return nilTxCarInfo, xerrors.Errorf("IsAndGetTxCarInfo: rows !=1")
	}

	//
	key, err := uuid.Parse(rows[0].CarKey)
	if err != nil {
		return nilTxCarInfo, xerrors.Errorf("IsAndGetTxCarInfo: uuid.Parse. CarKey:%s", rows[0].CarKey)
	}

	//
	pieceCid, err := cid.Decode(rows[0].PieceCid)
	if err != nil {
		return nilTxCarInfo, xerrors.Errorf("IsAndGetTxCarInfo: uuid.Parse. PieceCid:%s", rows[0].PieceCid)
	}

	return TxCarInfo{
		CarKey:    key,
		PieceCid:  pieceCid,
		PieceSize: rows[0].PieceSize,
		CarSize:   rows[0].CarSize,
	}, nil
}

func IsTxCarPiece(ctx context.Context, db *harmonydb.DB, pieceCid cid.Cid) (bool, error) {
	var rowCount int
	err := db.QueryRow(ctx, "SELECT COUNT(*) FROM tx_car_pieces WHERE piece_cid = $1", pieceCid.String()).Scan(&rowCount)
	if err != nil {
		return false, xerrors.Errorf("IsTxCarPiece: %w", err)
	}
	return rowCount > 0, nil
}

func IsTxCarPieceStr(ctx context.Context, db *harmonydb.DB, pieceCidStr string) (bool, error) {
	pieceCid, err := cid.Decode(pieceCidStr)
	if err != nil {
		return false, fmt.Errorf("tx pieceCidStr is invalid:%s", pieceCidStr)
	}
	return IsTxCarPiece(ctx, db, pieceCid)
}

func CreateFakeUnsealedFile(filePath string, txCarInfo TxCarInfo) error {
	txCarInfoStr := EncodeTxCarInfo(txCarInfo)
	return os.WriteFile(filePath, []byte(txCarInfoStr), 0755)
}
