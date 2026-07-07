package txcarlib

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"github.com/solopine/txcar/txcar"
	"golang.org/x/xerrors"
)

var log = logging.Logger("txcar")

const TxHttpPort = 24001

func ParseTxPieceFromUrlPath(path string) (*txcar.TxPiece, error) {
	path = strings.TrimPrefix(path, "/")
	p, err := txcar.BoostPathParser.Parse(path)
	return &p, err
}

func getCarFilePathInPiece(spId, sectorNumber int64) string {
	return fmt.Sprintf("/fc/cuseal/piece/%d_%d.car", spId, sectorNumber)
}

// return car file path
func CreateCarFileInPiece(ctx context.Context, txPiece *txcar.TxPiece, spId, sectorNumber int64) (string, error) {
	dest := getCarFilePathInPiece(spId, sectorNumber)
	err := os.Remove(dest)
	if err != nil {
		return "", err
	}
	err = txcar.CreateCarFileWithTxPieceWithDest(ctx, *txPiece, dest)
	if err != nil {
		return "", err
	}
	return dest, nil
}

func RemoveCarFileInPiece(spId, sectorNumber int64) {
	dest := getCarFilePathInPiece(spId, sectorNumber)
	log.Infow("Removing car file in piece", "dest", dest)
	os.Remove(dest)
}

func GetCarFileReaderInPiece(spId, sectorNumber int64) (io.ReadCloser, error) {
	dest := getCarFilePathInPiece(spId, sectorNumber)
	file, err := os.Open(dest)
	if err != nil {
		return nil, xerrors.Errorf("failed to Open CarFileInPiece. dest:%s: %w", dest, err)
	}
	return file, nil
}

func CreateFakeUnsealedFile(filePath string, txPiece txcar.TxPiece) error {
	txPieceStr := txcar.BoostPathParser.DeParse(txPiece)
	return os.WriteFile(filePath, []byte(txPieceStr), 0755)
}
