package txcar

import (
	"github.com/filecoin-project/go-padreader"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/storage/sealer/fr32"
	"github.com/filecoin-project/lotus/storage/sealer/partialfile"
	"github.com/filecoin-project/lotus/storage/sealer/storiface"
	"golang.org/x/xerrors"
	"io"
	"os"
	"path"
)

// (unsealedFilePath, error)
func NewTxCarUnsealedFile(txCarInfo TxCarInfo) (string, error) {
	log.Infow("----NewTxCarUnsealedFile", "txCarInfo", txCarInfo)
	destDir := "/cartmp"
	_, err := os.Stat(destDir)
	if err != nil {
		destDir = os.TempDir()
	}

	unsealedFileName := txCarInfo.PieceCid.String()
	unsealedFilePath := path.Join(destDir, unsealedFileName)

	carReader, err := NewTxCarReader(txCarInfo)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := carReader.Close(); err != nil {
			log.Warnw("closing carReader", "error", err)
		}
	}()

	pieceSize := abi.PaddedPieceSize(txCarInfo.PieceSize)
	padPieceData, err := padreader.NewInflator(carReader, uint64(txCarInfo.CarSize), pieceSize.Unpadded())
	if err != nil {
		return "", err
	}

	var done func()
	var stagedFile *partialfile.PartialFile

	defer func() {
		if done != nil {
			done()
		}

		if stagedFile != nil {
			if err := stagedFile.Close(); err != nil {
				log.Errorf("closing staged file: %+v", err)
			}
		}
	}()

	stagedFile, err = partialfile.CreatePartialFile(pieceSize, unsealedFilePath)
	if err != nil {
		return "", xerrors.Errorf("creating unsealed sector file: %w", err)
	}

	w, err := stagedFile.Writer(storiface.UnpaddedByteIndex(0).Padded(), pieceSize)
	if err != nil {
		return "", xerrors.Errorf("getting partial file writer: %w", err)
	}

	pw := fr32.NewPadWriter(w)

	pr := io.TeeReader(io.LimitReader(padPieceData, int64(pieceSize.Unpadded())), pw)

	chunk := abi.PaddedPieceSize(4 << 20)
	buf := make([]byte, chunk.Unpadded())
	for {
		_, err := pr.Read(buf)
		if err != nil && err != io.EOF {
			return "", xerrors.Errorf("pr read error: %w", err)
		}
		if err == io.EOF {
			break
		}
	}

	if err := pw.Close(); err != nil {
		return "", xerrors.Errorf("closing padded writer: %w", err)
	}

	if err := stagedFile.MarkAllocated(storiface.UnpaddedByteIndex(0).Padded(), pieceSize); err != nil {
		return "", xerrors.Errorf("marking data range as allocated: %w", err)
	}

	if err := stagedFile.Close(); err != nil {
		return "", err
	}
	stagedFile = nil

	return unsealedFilePath, nil
}
