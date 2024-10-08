package paths

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/curio/lib/partialfile"
	"github.com/filecoin-project/curio/lib/storiface"

	"github.com/filecoin-project/lotus/storage/sealer/fsutil"
)

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/pf.go -package=mocks . PartialFileHandler

// PartialFileHandler helps mock out the partial file functionality during testing.
type PartialFileHandler interface {
	// OpenPartialFile opens and returns a partial file at the given path and also verifies it has the given
	// size
	OpenPartialFile(maxPieceSize abi.PaddedPieceSize, path string) (*partialfile.PartialFile, error)

	// HasAllocated returns true if the given partial file has an unsealed piece starting at the given offset with the given size.
	// returns false otherwise.
	HasAllocated(pf *partialfile.PartialFile, offset storiface.UnpaddedByteIndex, size abi.UnpaddedPieceSize) (bool, error)

	// Reader returns a file from which we can read the unsealed piece in the partial file.
	Reader(pf *partialfile.PartialFile, offset storiface.PaddedByteIndex, size abi.PaddedPieceSize) (io.Reader, error)

	// Close closes the partial file
	Close(pf *partialfile.PartialFile) error
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/store.go -package=mocks . Store

type Store interface {
	AcquireSector(ctx context.Context, s storiface.SectorRef, existing storiface.SectorFileType, allocate storiface.SectorFileType, sealing storiface.PathType, op storiface.AcquireMode, opts ...storiface.AcquireOption) (paths storiface.SectorPaths, stores storiface.SectorPaths, err error)
	Remove(ctx context.Context, s abi.SectorID, types storiface.SectorFileType, force bool, keepIn []storiface.ID) error

	// like remove, but doesn't remove the primary sector copy, nor the last
	// non-primary copy if there no primary copies
	RemoveCopies(ctx context.Context, s abi.SectorID, types storiface.SectorFileType) error

	// move sectors into storage
	MoveStorage(ctx context.Context, s storiface.SectorRef, types storiface.SectorFileType, opts ...storiface.AcquireOption) error

	FsStat(ctx context.Context, id storiface.ID) (fsutil.FsStat, error)

	Reserve(ctx context.Context, sid storiface.SectorRef, ft storiface.SectorFileType, storageIDs storiface.SectorPaths, overheadTab map[storiface.SectorFileType]int, minFreePercentage float64) (func(), error)

	GenerateSingleVanillaProof(ctx context.Context, minerID abi.ActorID, si storiface.PostSectorChallenge, ppt abi.RegisteredPoStProof) ([]byte, error)
	GeneratePoRepVanillaProof(ctx context.Context, sr storiface.SectorRef, sealed, unsealed cid.Cid, ticket abi.SealRandomness, seed abi.InteractiveSealRandomness) ([]byte, error)
	ReadSnapVanillaProof(ctx context.Context, sr storiface.SectorRef) ([]byte, error)
}
