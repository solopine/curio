package indexing

import (
	"bytes"
	"context"
	"errors"
	"github.com/filecoin-project/curio/lib/paths"
	"github.com/filecoin-project/curio/txcar"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/yugabyte/pgx/v5"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/curio/deps/config"
	"github.com/filecoin-project/curio/harmony/harmonydb"
	"github.com/filecoin-project/curio/harmony/harmonytask"
	"github.com/filecoin-project/curio/harmony/resources"
	"github.com/filecoin-project/curio/harmony/taskhelp"
	"github.com/filecoin-project/curio/lib/ffi"
	"github.com/filecoin-project/curio/lib/passcall"
	"github.com/filecoin-project/curio/lib/pieceprovider"
	"github.com/filecoin-project/curio/lib/storiface"
	"github.com/filecoin-project/curio/market/indexstore"
	txcarlib "github.com/solopine/txcar/txcar"
)

var log = logging.Logger("indexing")

type IndexingTask struct {
	db                *harmonydb.DB
	indexStore        *indexstore.IndexStore
	pieceProvider     *pieceprovider.SectorReader
	sc                *ffi.SealCalls
	cfg               *config.CurioConfig
	insertConcurrency int
	insertBatchSize   int
	max               taskhelp.Limiter

	tmpSectorIndex paths.SectorIndex
	tmpLocal       *paths.Local
	inTest         atomic.Bool
}

func NewIndexingTask(db *harmonydb.DB, sc *ffi.SealCalls, indexStore *indexstore.IndexStore, pieceProvider *pieceprovider.SectorReader, cfg *config.CurioConfig, max taskhelp.Limiter, tmpSectorIndex paths.SectorIndex, tmpLocal *paths.Local) *IndexingTask {

	return &IndexingTask{
		db:                db,
		indexStore:        indexStore,
		pieceProvider:     pieceProvider,
		sc:                sc,
		cfg:               cfg,
		insertConcurrency: cfg.Market.StorageMarketConfig.Indexing.InsertConcurrency,
		insertBatchSize:   cfg.Market.StorageMarketConfig.Indexing.InsertBatchSize,
		max:               max,

		tmpSectorIndex: tmpSectorIndex,
		tmpLocal:       tmpLocal,
	}
}

type itask struct {
	UUID        string                  `db:"uuid"`
	SpID        int64                   `db:"sp_id"`
	Sector      abi.SectorNumber        `db:"sector"`
	Proof       abi.RegisteredSealProof `db:"reg_seal_proof"`
	PieceCid    string                  `db:"piece_cid"`
	Size        abi.PaddedPieceSize     `db:"piece_size"`
	Offset      int64                   `db:"sector_offset"`
	RawSize     int64                   `db:"raw_size"`
	ShouldIndex bool                    `db:"should_index"`
	Announce    bool                    `db:"announce"`
	ChainDealId abi.DealID              `db:"chain_deal_id"`
	IsDDO       bool                    `db:"is_ddo"`
}

func (i *IndexingTask) Do(taskID harmonytask.TaskID, stillOwned func() bool) (done bool, err error) {

	var tasks []itask

	ctx := context.Background()

	err = i.db.Select(ctx, &tasks, `SELECT 
											p.uuid, 
											p.sp_id, 
											p.sector,
											p.piece_cid, 
											p.piece_size, 
											p.sector_offset,
											p.reg_seal_proof,
											p.raw_size,
											p.should_index,
											p.announce,
											p.is_ddo,
											COALESCE(d.chain_deal_id, 0) AS chain_deal_id  -- If NULL, return 0
										FROM 
											market_mk12_deal_pipeline p
										LEFT JOIN 
											market_mk12_deals d 
											ON p.uuid = d.uuid AND p.sp_id = d.sp_id
										LEFT JOIN 
											market_direct_deals md 
											ON p.uuid = md.uuid AND p.sp_id = md.sp_id
										WHERE 
											p.indexing_task_id = $1;
										;`, taskID)
	if err != nil {
		return false, xerrors.Errorf("getting indexing params: %w", err)
	}

	if len(tasks) != 1 {
		return false, xerrors.Errorf("expected 1 sector params, got %d", len(tasks))
	}

	task := tasks[0]

	// Check if piece is already indexed
	var indexed bool
	err = i.db.QueryRow(ctx, `SELECT indexed FROM market_piece_metadata WHERE piece_cid = $1`, task.PieceCid).Scan(&indexed)
	if err != nil && err != pgx.ErrNoRows {
		return false, xerrors.Errorf("checking if piece %s is already indexed: %w", task.PieceCid, err)
	}

	// Return early if already indexed or should not be indexed
	if indexed || !task.ShouldIndex {
		err = i.recordCompletion(ctx, task, taskID, false)
		if err != nil {
			return false, err
		}
		log.Infow("Piece already indexed or should not be indexed", "piece_cid", task.PieceCid, "indexed", indexed, "should_index", task.ShouldIndex, "uuid", task.UUID, "sp_id", task.SpID, "sector", task.Sector)

		return true, nil
	}

	pieceCid, err := cid.Parse(task.PieceCid)
	if err != nil {
		return false, xerrors.Errorf("parsing piece CID: %w", err)
	}

	txPiece, err := txcar.GetTxPieceFromDbByPiece(ctx, i.db, pieceCid)
	if err != nil {
		return false, err
	}
	if txPiece != nil {
		log.Infow("----txPiece indexForTxPiece", "pieceCid", txPiece.PieceCid.String())
		return i.indexForTxPiece(ctx, taskID, task, txPiece)
	}

	log.Infow("----txPiece indexForNormal - will not index", "pieceCid", txPiece.PieceCid.String())
	_, err = i.db.Exec(ctx, `UPDATE market_mk12_deal_pipeline SET indexed = FALSE, should_index = FALSE, indexing_task_id = NULL, 
                                     complete = TRUE WHERE uuid = $1 AND indexing_task_id = $2`, task.UUID, taskID)
	if err != nil {
		return false, xerrors.Errorf("----UPDATE market_mk12_deal_pipeline: updating pipeline: %w", err)
	}

	return true, nil

	//
	//	reader, err := i.pieceProvider.ReadPiece(ctx, storiface.SectorRef{
	//		ID: abi.SectorID{
	//			Miner:  abi.ActorID(task.SpID),
	//			Number: task.Sector,
	//		},
	//		ProofType: task.Proof,
	//	}, storiface.PaddedByteIndex(task.Offset).Unpadded(), task.Size.Unpadded(), pieceCid)
	//
	//	if err != nil {
	//		return false, xerrors.Errorf("getting piece reader: %w", err)
	//	}
	//
	//	defer reader.Close()
	//
	//	startTime := time.Now()
	//
	//	dealCfg := i.cfg.Market.StorageMarketConfig
	//	chanSize := dealCfg.Indexing.InsertConcurrency * dealCfg.Indexing.InsertBatchSize
	//
	//	recs := make(chan indexstore.Record, chanSize)
	//
	//	//recs := make([]indexstore.Record, 0, chanSize)
	//	opts := []carv2.Option{carv2.ZeroLengthSectionAsEOF(true)}
	//	blockReader, err := carv2.NewBlockReader(bufio.NewReaderSize(reader, 4<<20), opts...)
	//	if err != nil {
	//		return false, fmt.Errorf("getting block reader over piece: %w", err)
	//	}
	//
	//	var eg errgroup.Group
	//	addFail := make(chan struct{})
	//	var interrupted bool
	//	var blocks int64
	//	start := time.Now()
	//
	//	eg.Go(func() error {
	//		defer close(addFail)
	//
	//		serr := i.indexStore.AddIndex(ctx, pieceCid, recs)
	//		if serr != nil {
	//			return xerrors.Errorf("adding index to DB: %w", serr)
	//		}
	//		return nil
	//	})
	//
	//	blockMetadata, err := blockReader.SkipNext()
	//loop:
	//	for err == nil {
	//		blocks++
	//
	//		select {
	//		case recs <- indexstore.Record{
	//			Cid:    blockMetadata.Cid,
	//			Offset: blockMetadata.Offset,
	//			Size:   blockMetadata.Size,
	//		}:
	//		case <-addFail:
	//			interrupted = true
	//			break loop
	//		}
	//		blockMetadata, err = blockReader.SkipNext()
	//	}
	//	if err != nil && !errors.Is(err, io.EOF) {
	//		return false, fmt.Errorf("generating index for piece: %w", err)
	//	}
	//
	//	// Close the channel
	//	close(recs)
	//
	//	// Wait till AddIndex is finished
	//	err = eg.Wait()
	//	if err != nil {
	//		return false, xerrors.Errorf("adding index to DB (interrupted %t): %w", interrupted, err)
	//	}
	//
	//	log.Infof("Indexing deal %s took %0.3f seconds", task.UUID, time.Since(startTime).Seconds())
	//
	//	err = i.recordCompletion(ctx, task, taskID, true)
	//	if err != nil {
	//		return false, err
	//	}
	//
	//	blocksPerSecond := float64(blocks) / time.Since(start).Seconds()
	//	log.Infow("Piece indexed", "piece_cid", task.PieceCid, "uuid", task.UUID, "sp_id", task.SpID, "sector", task.Sector, "blocks", blocks, "blocks_per_second", blocksPerSecond)
	//
	//	return true, nil
}

// recordCompletion add the piece metadata and piece deal to the DB and
// records the completion of an indexing task in the database
func (i *IndexingTask) recordCompletion(ctx context.Context, task itask, taskID harmonytask.TaskID, indexed bool) error {
	_, err := i.db.Exec(ctx, `SELECT process_piece_deal($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		task.UUID, task.PieceCid, !task.IsDDO, task.SpID, task.Sector, task.Offset, task.Size, task.RawSize, indexed, false, task.ChainDealId)
	if err != nil {
		return xerrors.Errorf("failed to update piece metadata and piece deal for deal %s: %w", task.UUID, err)
	}

	// If IPNI is disabled then mark deal as complete otherwise just mark as indexed
	if i.cfg.Market.StorageMarketConfig.IPNI.Disable {
		n, err := i.db.Exec(ctx, `UPDATE market_mk12_deal_pipeline SET indexed = TRUE, indexing_task_id = NULL, 
                                     complete = TRUE WHERE uuid = $1 AND indexing_task_id = $2`, task.UUID, taskID)
		if err != nil {
			return xerrors.Errorf("store indexing success: updating pipeline: %w", err)
		}
		if n != 1 {
			return xerrors.Errorf("store indexing success: updated %d rows", n)
		}
	} else {
		n, err := i.db.Exec(ctx, `UPDATE market_mk12_deal_pipeline SET indexed = TRUE, indexing_task_id = NULL 
                                 WHERE uuid = $1 AND indexing_task_id = $2`, task.UUID, taskID)
		if err != nil {
			return xerrors.Errorf("store indexing success: updating pipeline: %w", err)
		}
		if n != 1 {
			return xerrors.Errorf("store indexing success: updated %d rows", n)
		}
	}

	return nil
}

func (i *IndexingTask) CanAccept(ids []harmonytask.TaskID, engine *harmonytask.TaskEngine) (*harmonytask.TaskID, error) {
	ctx := context.Background()

	indIDs := make([]int64, len(ids))
	for x, id := range ids {
		indIDs[x] = int64(id)
	}

	// Accept any task which should not be indexed as
	// it does not require storage access
	var id int64
	err := i.db.QueryRow(ctx, `SELECT indexing_task_id 
										FROM market_mk12_deal_pipeline 
										WHERE should_index = FALSE AND 
										      indexing_task_id = ANY ($1) ORDER BY indexing_task_id LIMIT 1`, indIDs).Scan(&id)
	if err == nil {
		ret := harmonytask.TaskID(id)
		return &ret, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, xerrors.Errorf("getting pending indexing task: %w", err)
	}

	var tasks []struct {
		TaskID       harmonytask.TaskID `db:"indexing_task_id"`
		SpID         int64              `db:"sp_id"`
		SectorNumber int64              `db:"sector"`
		StorageID    string             `db:"storage_id"`
	}

	if storiface.FTUnsealed != 1 {
		panic("storiface.FTUnsealed != 1")
	}

	err = i.db.Select(ctx, &tasks, `
		SELECT dp.indexing_task_id, dp.sp_id, dp.sector, l.storage_id FROM market_mk12_deal_pipeline dp
			INNER JOIN sector_location l ON dp.sp_id = l.miner_id AND dp.sector = l.sector_num
			WHERE dp.indexing_task_id = ANY ($1) AND l.sector_filetype = 1
`, indIDs)
	if err != nil {
		return nil, xerrors.Errorf("getting tasks: %w", err)
	}

	ls, err := i.sc.LocalStorage(ctx)
	if err != nil {
		return nil, xerrors.Errorf("getting local storage: %w", err)
	}

	localStorageMap := make(map[string]bool, len(ls))
	for _, l := range ls {
		localStorageMap[string(l.ID)] = true
	}

	for _, t := range tasks {
		if found, ok := localStorageMap[t.StorageID]; ok && found {
			return &t.TaskID, nil
		}
	}

	return nil, nil
}

func (i *IndexingTask) FixIndex() error {
	type TxPieceRow struct {
		CarKey     string
		Version    int
		PieceCid   string
		PayloadCid string
		PieceSize  int64
		CarSize    int64
	}

	type TxSectorRow struct {
		SpId      uint64
		SectorNum uint64
	}

	innerParseFromTxPieceRows := func(row TxPieceRow) (*txcarlib.TxPiece, error) {
		key, err := uuid.Parse(row.CarKey)
		if err != nil {
			return nil, xerrors.Errorf("parseFromTxPieceRow: uuid.Parse. CarKey:%s", row.CarKey)
		}

		//
		pieceCid, err := cid.Decode(row.PieceCid)
		if err != nil {
			return nil, xerrors.Errorf("parseFromTxPieceRow: pieceCid Parse. PieceCid:%s", row.PieceCid)
		}

		return &txcarlib.TxPiece{
			Version:   txcarlib.Version(row.Version),
			CarKey:    key,
			PieceCid:  pieceCid,
			PieceSize: abi.PaddedPieceSize(row.PieceSize),
			CarSize:   abi.UnpaddedPieceSize(row.CarSize),
		}, nil
	}

	ctx := context.Background()

	var rows []TxPieceRow

	err := i.db.Select(ctx, &rows, `
select tx_car_pieces.car_key, tx_car_pieces.version, tx_car_pieces.piece_cid, tx_car_pieces.piece_size,tx_car_pieces.car_size
from tx_car_pieces 
left join tx_car on tx_car_pieces.piece_cid = tx_car.piece_cid 
where tx_car_pieces.version>1 and tx_car.piece_cid is null;
`)
	if err != nil {
		return err
	}

	proof := abi.RegisteredSealProof_StackedDrg32GiBV1_1

	rowSize := len(rows)
	for ii, row := range rows {
		log.Infow("process", "i", ii, "size", rowSize, "PieceCid", row.PieceCid)
		txPiece, err := innerParseFromTxPieceRows(row)
		if err != nil {
			return err
		}
		if txPiece == nil {
			return xerrors.Errorf("txPiece == nil")
		}

		var secRows []TxSectorRow
		err = i.db.Select(ctx, &secRows, `
select sp_id, sector_num 
from market_piece_deal 
where piece_cid=$1;
`, row.PieceCid)
		if err != nil {
			return err
		}

		if len(secRows) == 0 {
			continue
		}

		//
		var allRecs []txcarlib.TxBlockRecord
		var lastRec txcarlib.TxBlockRecord
		for _, secRow := range secRows {
			spId := abi.ActorID(secRow.SpId)
			sectorNum := abi.SectorNumber(secRow.SectorNum)

			allRecs, err = tmpParseRecordsForTxPiece(ctx, i.pieceProvider, spId, sectorNum, proof, txPiece.PieceCid)
			if err != nil {
				log.Errorw("tmpParseRecordsForTxPiece", "err", err)
				continue
			}
			lastRec = allRecs[len(allRecs)-1]
			log.Infow("lastRec.Cid", "txPiece", txPiece, "lastRec.Cid", lastRec.Cid)
			break
		}
		if allRecs == nil {
			log.Errorw("something wrong in tmpParseRecordsForTxPiece, skip")
			continue
		}
		txPiece.PayloadCid = lastRec.Cid

		tmpIndexPath := filepath.Join(os.TempDir(), row.PieceCid)

		err = tmpCreateTxCarIndexFile(txPiece, allRecs, tmpIndexPath)
		if err != nil {
			return err
		}

		for _, secRow := range secRows {
			spId := abi.ActorID(secRow.SpId)
			sectorNum := abi.SectorNumber(secRow.SectorNum)

			//
			sector := abi.SectorID{
				Miner:  spId,
				Number: sectorNum,
			}
			log.Infow("sector", "sector", sector)

			sr := storiface.SectorRef{
				ID:        sector,
				ProofType: proof,
			}

			existing := storiface.FTUnsealed
			allocate := storiface.FTNone
			pathType := storiface.PathStorage
			op := storiface.AcquireCopy
			out, storageIDs, err := i.tmpLocal.AcquireSector(ctx, sr, existing, allocate, pathType, op)
			if err != nil {
				return err
			}
			log.Infow("AcquireSector", "out", out, "storageIDs", storageIDs)

			if out.Unsealed == "" {
				log.Errorw("out.Unsealed empty")
				continue
			}

			err = CopyFile(tmpIndexPath, out.Unsealed)
			if err != nil {
				return err
			}
		}
		err = os.Remove(tmpIndexPath)
		if err != nil {
			return err
		}

		_, err = i.db.Exec(ctx, `
insert into tx_car(car_key, version, piece_cid, payload_cid, piece_size, car_size, create_time, update_time)
values ($1, $2, $3, $4, $5, $6, current_timestamp, current_timestamp);
`, txPiece.CarKey, txPiece.Version, txPiece.PieceCid, txPiece.PayloadCid, txPiece.PieceSize, txPiece.CarSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func CopyFile(from, to string) error {
	// `cp` has decades of experience in moving files quickly; don't pretend we
	//  can do better
	log.Debugw("CopyFile", "from", from, "to", to)

	var errOut bytes.Buffer

	var cmd *exec.Cmd
	cmd = exec.Command("/usr/bin/env", "cp", from, to) // nolint

	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf("exec mv (stderr: %s): %w", strings.TrimSpace(errOut.String()), err)
	}

	return nil
}

func (i *IndexingTask) TypeDetails() harmonytask.TaskTypeDetails {
	//dealCfg := i.cfg.Market.StorageMarketConfig
	//chanSize := dealCfg.Indexing.InsertConcurrency * dealCfg.Indexing.InsertBatchSize * 56 // (56 = size of each index.Record)

	return harmonytask.TaskTypeDetails{
		Name: "Indexing",
		Cost: resources.Resources{
			Cpu: 1,
			Ram: uint64(i.insertBatchSize * i.insertConcurrency * 56 * 2),
		},
		Max:         i.max,
		MaxFailures: 3,
		IAmBored: passcall.Every(30*time.Second, func(taskFunc harmonytask.AddTaskFunc) error {
			return i.schedule(context.Background(), taskFunc)
		}),
	}
}

func (i *IndexingTask) schedule(ctx context.Context, taskFunc harmonytask.AddTaskFunc) error {
	//{
	//	if i.inTest.CompareAndSwap(false, true) {
	//		log.Infow("FixIndex...")
	//		err := i.FixIndex()
	//		if err != nil {
	//			return err
	//		}
	//	} else {
	//		log.Infow("skip FixIndex...")
	//		return nil
	//	}
	//}

	// schedule submits
	var stop bool
	for !stop {
		taskFunc(func(id harmonytask.TaskID, tx *harmonydb.Tx) (shouldCommit bool, seriousError error) {
			stop = true // assume we're done until we find a task to schedule

			var pendings []struct {
				UUID string `db:"uuid"`
			}

			// Indexing job must be created for every deal to make sure piece details are inserted in DB
			// even if we don't want to index it. If piece is not supposed to be indexed then it will handled
			// by the Do()
			err := i.db.Select(ctx, &pendings, `SELECT uuid FROM market_mk12_deal_pipeline 
            										WHERE sealed = TRUE
            										AND indexing_task_id IS NULL
            										AND indexed = FALSE
													ORDER BY indexing_created_at ASC LIMIT 1;`)
			if err != nil {
				return false, xerrors.Errorf("getting pending indexing tasks: %w", err)
			}

			if len(pendings) == 0 {
				return false, nil
			}

			pending := pendings[0]

			_, err = tx.Exec(`UPDATE market_mk12_deal_pipeline SET indexing_task_id = $1 
                             WHERE indexing_task_id IS NULL AND uuid = $2`, id, pending.UUID)
			if err != nil {
				return false, xerrors.Errorf("updating indexing task id: %w", err)
			}

			stop = false // we found a task to schedule, keep going
			return true, nil
		})
	}

	return nil
}

func (i *IndexingTask) Adder(taskFunc harmonytask.AddTaskFunc) {
}

func (i *IndexingTask) GetSpid(db *harmonydb.DB, taskID int64) string {
	var spid string
	err := db.QueryRow(context.Background(), `SELECT sp_id FROM market_mk12_deal_pipeline WHERE indexing_task_id = $1`, taskID).Scan(&spid)
	if err != nil {
		log.Errorf("getting spid: %s", err)
		return ""
	}
	return spid
}

func (i *IndexingTask) indexForTxPiece(ctx context.Context, taskID harmonytask.TaskID, task itask, txPiece *txcarlib.TxPiece) (done bool, err error) {
	if txPiece == nil {
		return false, xerrors.Errorf("cannot call indexForTxPiece with txPiece==nil")
	}

	if txPiece.Version == txcarlib.V1 {
		log.Infow("----txPiece V1 indexForTxPiece", "pieceCid", txPiece.PieceCid.String())

		_, err = i.db.Exec(ctx, `UPDATE market_mk12_deal_pipeline SET indexed = FALSE, should_index = FALSE, indexing_task_id = NULL, 
                                     complete = TRUE WHERE uuid = $1 AND indexing_task_id = $2`, task.UUID, taskID)
		if err != nil {
			return false, xerrors.Errorf("----UPDATE market_mk12_deal_pipeline: updating pipeline1: %w", err)
		}
		return true, nil
	}

	log.Infow("----txPiece indexForTxPiece", "pieceCid", txPiece.PieceCid.String())

	startTime := time.Now()

	allRecs, err := parseRecordsForTxPiece(ctx, i.pieceProvider, abi.ActorID(task.SpID), abi.SectorNumber(task.Sector), task.Proof, txPiece.PieceCid)
	if err != nil {
		log.Errorw("----indexForTxPiece.parseRecordsForTxPiece error", "sp", task.SpID, "sector", task.Sector, "version", txPiece.Version, "pieceCid", txPiece.PieceCid.String(), "error", err)

		//_, err = i.db.Exec(ctx, `UPDATE market_mk12_deal_pipeline SET indexed = FALSE, should_index = FALSE, indexing_task_id = NULL,
		//                             complete = TRUE WHERE uuid = $1 AND indexing_task_id = $2`, task.UUID, taskID)
		//if err != nil {
		//	return false, xerrors.Errorf("----UPDATE market_mk12_deal_pipeline: updating pipeline2: %w", err)
		//}
		return false, err
	}

	dealCfg := i.cfg.Market.StorageMarketConfig
	chanSize := dealCfg.Indexing.InsertConcurrency * dealCfg.Indexing.InsertBatchSize
	recs := make(chan indexstore.Record, chanSize)

	var eg errgroup.Group
	addFail := make(chan struct{})
	var interrupted bool
	var blocks int64
	start := time.Now()

	eg.Go(func() error {
		defer close(addFail)

		serr := i.indexStore.AddIndex(ctx, txPiece.PieceCid, recs)
		if serr != nil {
			return xerrors.Errorf("adding index to DB: %w", serr)
		}
		return nil
	})

	for _, rec := range allRecs {
		blocks++

		select {
		case recs <- indexstore.Record{
			Cid:    rec.Cid,
			Offset: rec.Offset,
			Size:   rec.Size,
		}:
		case <-addFail:
			interrupted = true
			break
		}
	}

	// Close the channel
	close(recs)

	// Wait till AddIndex is finished
	err = eg.Wait()
	if err != nil {
		return false, xerrors.Errorf("adding index to DB (interrupted %t): %w", interrupted, err)
	}

	log.Infof("Indexing deal %s took %0.3f seconds", task.UUID, time.Since(startTime).Seconds())

	err = i.recordCompletion(ctx, task, taskID, true)
	if err != nil {
		return false, err
	}

	blocksPerSecond := float64(blocks) / time.Since(start).Seconds()
	log.Infow("Piece indexed", "piece_cid", task.PieceCid, "uuid", task.UUID, "sp_id", task.SpID, "sector", task.Sector, "blocks", blocks, "blocks_per_second", blocksPerSecond)

	return true, nil
}

var _ = harmonytask.Reg(&IndexingTask{})
var _ harmonytask.TaskInterface = &IndexingTask{}
