package seal

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/filecoin-project/curio/lib/paths"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/curio/build"
	"github.com/filecoin-project/curio/harmony/harmonydb"
	"github.com/filecoin-project/curio/harmony/harmonytask"
	"github.com/filecoin-project/curio/harmony/resources"
	"github.com/filecoin-project/curio/harmony/taskhelp"
	"github.com/filecoin-project/curio/lib/dealdata"
	ffi2 "github.com/filecoin-project/curio/lib/ffi"
	storiface "github.com/filecoin-project/curio/lib/storiface"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/types"
)

var IsDevnet = build.BlockDelaySecs < 30

func SetDevnet(value bool) {
	IsDevnet = value
}

func GetDevnet() bool {
	return IsDevnet
}

type SDRAPI interface {
	ChainHead(context.Context) (*types.TipSet, error)
	StateGetRandomnessFromTickets(context.Context, crypto.DomainSeparationTag, abi.ChainEpoch, []byte, types.TipSetKey) (abi.Randomness, error)
}

type SDRTask struct {
	api SDRAPI
	db  *harmonydb.DB
	sp  *SealPoller

	sc *ffi2.SealCalls

	max taskhelp.Limiter
	min int
}

func NewSDRTask(api SDRAPI, db *harmonydb.DB, sp *SealPoller, sc *ffi2.SealCalls, maxSDR taskhelp.Limiter, minSDR int) *SDRTask {
	return &SDRTask{
		api: api,
		db:  db,
		sp:  sp,
		sc:  sc,
		max: maxSDR,
		min: minSDR,
	}
}

func (s *SDRTask) Do(taskID harmonytask.TaskID, stillOwned func() bool) (done bool, err error) {
	log.Infow("----sdr.do", "taskID", taskID)
	ctx := context.Background()

	var sectorParamsArr []struct {
		SpID         int64                   `db:"sp_id"`
		SectorNumber int64                   `db:"sector_number"`
		RegSealProof abi.RegisteredSealProof `db:"reg_seal_proof"`
		TicketEpoch  *abi.ChainEpoch         `db:"ticket_epoch"`
	}

	err = s.db.Select(ctx, &sectorParamsArr, `
		SELECT sp_id, sector_number, reg_seal_proof, ticket_epoch
		FROM sectors_sdr_pipeline
		WHERE task_id_sdr = $1`, taskID)
	if err != nil {
		return false, xerrors.Errorf("getting sector params: %w", err)
	}

	if len(sectorParamsArr) != 1 {
		return false, xerrors.Errorf("expected 1 sector params, got %d", len(sectorParamsArr))
	}
	sectorParams := sectorParamsArr[0]

	log.Infow("----sdr.do.1", "taskID", taskID, "sectorParams", sectorParams)
	dealData, err := dealdata.DealDataSDRPoRep(ctx, s.db, s.sc, sectorParams.SpID, sectorParams.SectorNumber, sectorParams.RegSealProof, true)
	if err != nil {
		return false, xerrors.Errorf("getting deal data: %w", err)
	}

	sref := storiface.SectorRef{
		ID: abi.SectorID{
			Miner:  abi.ActorID(sectorParams.SpID),
			Number: abi.SectorNumber(sectorParams.SectorNumber),
		},
		ProofType: sectorParams.RegSealProof,
	}

	log.Infow("----sdr.do.2", "taskID", taskID, "sref", sref, "dealData.CommD", dealData.CommD.String(), "dealData.IsUnpadded", dealData.IsUnpadded, "dealData.KeepUnsealed", dealData.KeepUnsealed)
	// get ticket
	maddr, err := address.NewIDAddress(uint64(sectorParams.SpID))
	if err != nil {
		return false, xerrors.Errorf("getting miner address: %w", err)
	}

	var ticket abi.SealRandomness
	var ticketEpoch abi.ChainEpoch
	if sectorParams.TicketEpoch == nil {
		// FAIL: api may be down
		// FAIL-RESP: rely on harmony retry
		ticket, ticketEpoch, err = GetTicket(ctx, s.api, maddr)
		if err != nil {
			return false, xerrors.Errorf("getting ticket: %w", err)
		}
	} else {
		ticketEpoch = *sectorParams.TicketEpoch
		log.Infow("----sdr.do.2.5 already have ticket", "ticketEpoch", ticketEpoch)

		ticket, err = getTicketFromEpoch(ctx, s.api, maddr, ticketEpoch)
		if err != nil {
			return false, xerrors.Errorf("getTicketFromEpoch ticket: %w", err)
		}
	}

	log.Infow("----sdr.do.3", "taskID", taskID, "sref", sref, "ticketEpoch", ticketEpoch, "ticket", hex.EncodeToString(ticket))

	// do the SDR!!

	// FAIL: storage may not have enough space
	// FAIL-RESP: rely on harmony retry

	// LATEFAIL: compute error in sdr
	// LATEFAIL-RESP: Check in Trees task should catch this; Will retry computing
	//                Trees; After one retry, it should return the sector to the
	// 			      SDR stage; max number of retries should be configurable

	err = s.sc.GenerateSDR(ctx, taskID, storiface.FTCache, sref, ticket, dealData.CommD)
	if err != nil {
		return false, xerrors.Errorf("generating sdr: %w", err)
	}

	log.Infow("----sdr.do.4.done", "taskID", taskID, "sref", sref)

	// store success!
	n, err := s.db.Exec(ctx, `UPDATE sectors_sdr_pipeline
		SET after_sdr = true, ticket_epoch = $3, ticket_value = $4, task_id_sdr = NULL
		WHERE sp_id = $1 AND sector_number = $2`,
		sectorParams.SpID, sectorParams.SectorNumber, ticketEpoch, []byte(ticket))
	if err != nil {
		return false, xerrors.Errorf("store sdr success: updating pipeline: %w", err)
	}
	if n != 1 {
		return false, xerrors.Errorf("store sdr success: updated %d rows", n)
	}

	return true, nil
}

type TicketNodeAPI interface {
	ChainHead(context.Context) (*types.TipSet, error)
	StateGetRandomnessFromTickets(context.Context, crypto.DomainSeparationTag, abi.ChainEpoch, []byte, types.TipSetKey) (abi.Randomness, error)
}

func GetTicket(ctx context.Context, api TicketNodeAPI, maddr address.Address) (abi.SealRandomness, abi.ChainEpoch, error) {
	ts, err := api.ChainHead(ctx)
	if err != nil {
		return nil, 0, xerrors.Errorf("getting chain head: %w", err)
	}

	ticketEpoch := ts.Height() - policy.SealRandomnessLookback
	buf := new(bytes.Buffer)
	if err := maddr.MarshalCBOR(buf); err != nil {
		return nil, 0, xerrors.Errorf("marshaling miner address: %w", err)
	}

	rand, err := api.StateGetRandomnessFromTickets(ctx, crypto.DomainSeparationTag_SealRandomness, ticketEpoch, buf.Bytes(), ts.Key())
	if err != nil {
		return nil, 0, xerrors.Errorf("getting randomness from tickets: %w", err)
	}

	return abi.SealRandomness(rand), ticketEpoch, nil
}

func (t *SDRTask) CanAccept(ids []harmonytask.TaskID, engine *harmonytask.TaskEngine) (*harmonytask.TaskID, error) {
	if t.min > len(ids) {
		log.Debugw("did not accept task", "name", "SDR", "reason", "below min", "min", t.min, "count", len(ids))
		return nil, nil
	}

	ctx := context.Background()
	ls, err := t.sc.LocalStorage(ctx)
	if err != nil {
		return nil, xerrors.Errorf("getting local storage: %w", err)
	}
	log.Infow("----sdr.CanAccept.LocalStorage", "ls", ls)
	for _, id := range ids {
		s, err := t.taskToSector(id)
		log.Infow("----sdr.CanAccept", "s", s, "TaskId", id)
		if err != nil {
			continue
		}
		var storageIds []string
		err = t.db.Select(ctx, &storageIds, `SELECT storage_id FROM sector_location WHERE miner_id=$1 AND sector_num=$2 AND sector_filetype=$3`,
			s.SpID, s.SectorNumber, storiface.FTCache)
		if err != nil {
			return nil, xerrors.Errorf("getting sector id from db: %w", err)
		}
		log.Infow("----sdr.CanAccept.Select", "storageIds", storageIds)
		if len(storageIds) == 0 {
			log.Infow("----sdr.CanAccept.newly", "s", s, "TaskId", id)
			for _, localStoragePath := range ls {
				if localStoragePath.CanSeal {
					ok, err := allocPathOk(localStoragePath.AllowTypes, localStoragePath.DenyTypes, localStoragePath.AllowMiners, localStoragePath.DenyMiners, storiface.FTCache, s.ID().Miner)
					if err != nil {
						return nil, xerrors.Errorf("allocPathOk: %w", err)
					}
					if ok {
						return &id, nil
					}
				}
			}
			log.Infow("----sdr.CanAccept.newly but no local storage for miner", "s", s, "TaskId", id)
			continue
		}

		log.Infow("----sdr.CanAccept.existed", "s", s, "TaskId", id)
		for _, localStoragePath := range ls {
			if localStoragePath.CanSeal {
				for _, storageId := range storageIds {
					if storageId == string(localStoragePath.ID) {
						return &id, nil
					}
				}
			}
		}
	}

	return nil, xerrors.Errorf("Can NOT accept SDR")
}

func allocPathOk(allowTypes, denyTypes, allowMiners, denyMiners []string, fileType storiface.SectorFileType, miner abi.ActorID) (bool, error) {
	if !fileType.Allowed(allowTypes, denyTypes) {
		return false, nil
	}
	proceed, _, err := paths.MinerFilter(allowMiners, denyMiners, miner)
	if err != nil {
		return false, err
	}
	if !proceed {
		return false, nil
	}

	return true, nil
}

func (s *SDRTask) TypeDetails() harmonytask.TaskTypeDetails {
	ssize := abi.SectorSize(32 << 30) // todo task details needs taskID to get correct sector size
	if IsDevnet {
		ssize = abi.SectorSize(2 << 20)
	}

	res := harmonytask.TaskTypeDetails{
		Max:  s.max,
		Name: "SDR",
		Cost: resources.Resources{
			Cpu:     4, // todo multicore sdr
			Gpu:     0,
			Ram:     (64 << 30) + (256 << 20),
			Storage: s.sc.Storage(s.taskToSector, storiface.FTCache, storiface.FTNone, ssize, storiface.PathSealing, 50),
		},
		MaxFailures: 2,
		Follows:     nil,
	}

	if IsDevnet {
		res.Cost.Ram = 1 << 30
		res.Cost.Cpu = 1
	}

	return res
}

func (s *SDRTask) Adder(taskFunc harmonytask.AddTaskFunc) {
	s.sp.pollers[pollerSDR].Set(taskFunc)
}

func (s *SDRTask) GetSpid(db *harmonydb.DB, taskID int64) string {
	sid, err := s.GetSectorID(db, taskID)
	if err != nil {
		log.Errorf("getting sector id: %s", err)
		return ""
	}
	return sid.Miner.String()
}

func (s *SDRTask) GetSectorID(db *harmonydb.DB, taskID int64) (*abi.SectorID, error) {
	var spId, sectorNumber uint64
	err := db.QueryRow(context.Background(), `SELECT sp_id,sector_number FROM sectors_sdr_pipeline WHERE task_id_sdr = $1`, taskID).Scan(&spId, &sectorNumber)
	if err != nil {
		return nil, err
	}
	return &abi.SectorID{
		Miner:  abi.ActorID(spId),
		Number: abi.SectorNumber(sectorNumber),
	}, nil
}

var _ = harmonytask.Reg(&SDRTask{})

func (s *SDRTask) taskToSector(id harmonytask.TaskID) (ffi2.SectorRef, error) {
	var refs []ffi2.SectorRef

	err := s.db.Select(context.Background(), &refs, `SELECT sp_id, sector_number, reg_seal_proof FROM sectors_sdr_pipeline WHERE task_id_sdr = $1`, id)
	if err != nil {
		return ffi2.SectorRef{}, xerrors.Errorf("getting sector ref: %w", err)
	}

	if len(refs) != 1 {
		return ffi2.SectorRef{}, xerrors.Errorf("expected 1 sector ref, got %d", len(refs))
	}

	return refs[0], nil
}

func getTicketFromEpoch(ctx context.Context, api TicketNodeAPI, maddr addr.Address, ticketEpoch abi.ChainEpoch) (abi.SealRandomness, error) {
	ts, err := api.ChainHead(ctx)
	if err != nil {
		return nil, xerrors.Errorf("getting chain head: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := maddr.MarshalCBOR(buf); err != nil {
		return nil, xerrors.Errorf("marshaling miner address: %w", err)
	}

	rand, err := api.StateGetRandomnessFromTickets(ctx, crypto.DomainSeparationTag_SealRandomness, ticketEpoch, buf.Bytes(), ts.Key())
	if err != nil {
		return nil, xerrors.Errorf("getting randomness from tickets: %w", err)
	}

	return abi.SealRandomness(rand), nil
}

var _ harmonytask.TaskInterface = &SDRTask{}
