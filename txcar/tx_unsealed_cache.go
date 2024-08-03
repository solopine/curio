package txcar

import (
	"context"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"os"
	"sync"
	"time"
)

const (
	maxCacheSize               = 2
	serveTimeOutSeconds        = 600
	safeTimeoutToDeleteSeconds = 60
)

var pieceCidToServeMapLock = sync.Mutex{}
var pieceCidToServeMap = map[cid.Cid]*serveOperation{}

type serveOperation struct {
	ctx           context.Context
	cancel        context.CancelFunc
	rwLock        sync.RWMutex
	txCarInfo     *TxCarInfo
	filePath      string
	isFileCreated bool
	// req wait creating
	createDoneMap map[uuid.UUID]chan string
	// reqId -> done chan
	serveDoneMap        map[uuid.UUID]chan struct{}
	timeout             <-chan time.Time
	safeTimeoutToDelete <-chan time.Time
	canBeDeleted        bool
	wakeFromIdle        chan struct{}
}

func newServeOperation(txCarInfo *TxCarInfo, reqId uuid.UUID, filePathCh chan string, serveDone chan struct{}) *serveOperation {
	return &serveOperation{
		rwLock:    sync.RWMutex{},
		txCarInfo: txCarInfo,
		createDoneMap: map[uuid.UUID]chan string{
			reqId: filePathCh,
		},
		serveDoneMap: map[uuid.UUID]chan struct{}{
			reqId: serveDone,
		},
		wakeFromIdle: make(chan struct{}, 1),
	}
}

// return filePath chan
func (op *serveOperation) addRequest(reqId uuid.UUID, serveDone chan struct{}) (chan string, error) {
	op.rwLock.Lock()
	defer func() {
		op.rwLock.Unlock()
		op.wakeFromIdle <- struct{}{}
	}()
	_, exist := op.serveDoneMap[reqId]
	if exist {
		return nil, xerrors.Errorf("serveDoneMap.reqId exist. reqId:%x, txCarInfo:%x", reqId, op.txCarInfo)
	}
	op.serveDoneMap[reqId] = serveDone
	op.canBeDeleted = false

	ch := make(chan string, 1)

	if op.isFileCreated {
		ch <- op.filePath
	} else {
		_, exist = op.createDoneMap[reqId]
		if exist {
			return nil, xerrors.Errorf("createDoneMap.reqId exist. reqId:%x, txCarInfo:%x", reqId, op.txCarInfo)
		}
		op.createDoneMap[reqId] = ch
	}
	return ch, nil
}

func (op *serveOperation) isIdle() bool {
	return op.canBeDeleted
}

// return filePath chan
func (op *serveOperation) startServe(parentCtx context.Context) error {
	file, err := NewTxCarUnsealedFile(*op.txCarInfo)
	if err != nil {
		return nil
	}
	log.Infow("----GetTxCarUnsealedCache.startServe file created", "pieceCid", op.txCarInfo.PieceCid)

	op.rwLock.Lock()
	op.ctx, op.cancel = context.WithCancel(parentCtx)
	op.filePath = file
	op.isFileCreated = true
	for req, createDone := range op.createDoneMap {
		createDone <- file
		delete(op.createDoneMap, req)
	}
	op.rwLock.Unlock()
	ctx := op.ctx

	defer func() {
		log.Infow("----txcar.startServe finished, now remove file", "car", op.txCarInfo)
		err := os.Remove(op.filePath)
		if err != nil {
			log.Errorw("----txcar.startServe remove file fail", "car", op.txCarInfo)
		}
	}()

	// serve
	for {
		op.rwLock.Lock()
		log.Infow("----txcar.startServe.serveDoneMap", "car", op.txCarInfo)
		for req, serveDone := range op.serveDoneMap {
			select {
			case <-ctx.Done():
				op.rwLock.Unlock()
				return ctx.Err()
			case <-serveDone:
				log.Infow("----txcar.startServe done for req", "req", req, "car", op.txCarInfo)
				op.timeout = time.After(serveTimeOutSeconds * time.Second)
				op.safeTimeoutToDelete = time.After(safeTimeoutToDeleteSeconds * time.Second)
				delete(op.serveDoneMap, req)
				select {
				case op.wakeFromIdle <- struct{}{}:
				default:
				}
			default:
				//No value ready, moving on.
			}
		}

		inServeCount := len(op.serveDoneMap)
		op.rwLock.Unlock()

		if inServeCount == 0 {
			// none in serve, so idle
			log.Infow("----txcar.startServe in idle", "car", op.txCarInfo)
			select {
			case <-ctx.Done():
				log.Infow("----txcar.startServe canceled, now clean", "car", op.txCarInfo)
				return ctx.Err()
			case _, ok := <-op.timeout:
				if ok {
					// timeout need clean
					log.Infow("----txcar.startServe timeout, now clean", "car", op.txCarInfo)
					return nil
				}
				// op.timeout just closed and replace by another one
			case <-op.safeTimeoutToDelete:
				op.canBeDeleted = true
			case <-op.wakeFromIdle:
				// something changed, need check again
			}
		} else {
			// still in serve
			log.Infow("----txcar.startServe still in serve", "inServeCount", inServeCount, "car", op.txCarInfo)
			select {
			case <-ctx.Done():
				log.Infow("----txcar.startServe.ctx.Done", "car", op.txCarInfo)
				return ctx.Err()
			case <-time.After(10 * time.Second):
				log.Infow("----txcar.startServe.waited 10s", "car", op.txCarInfo)
			}
		}
	}
}

func (op *serveOperation) stopServe() {
	op.cancel()
}

func GetTxCarUnsealedCache(txCarInfo TxCarInfo, serveDone chan struct{}) (string, error) {
	ctx := context.Background()
	reqId := uuid.New()
	log.Infow("----GetTxCarUnsealedCache.start", "reqId", reqId, "txCarInfo", txCarInfo)

	pieceCidToServeMapLock.Lock()
	if op, ok := pieceCidToServeMap[txCarInfo.PieceCid]; ok {
		// already exist
		log.Infow("----GetTxCarUnsealedCache. in cache", "pieceCid", op.txCarInfo.PieceCid)

		// add request to op serve list
		filePathCh, err := op.addRequest(reqId, serveDone)
		pieceCidToServeMapLock.Unlock()

		if err != nil {
			return "", err
		}

		// wait file created
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case s := <-filePathCh:
			return s, nil
		}
	}

	log.Infow("----GetTxCarUnsealedCache. NOT in cache", "reqId", reqId, "pieceCid", txCarInfo.PieceCid)

	// wait and start new serve
	// wait or swap other serve thread
	for {
		canServe := false
		if len(pieceCidToServeMap) < maxCacheSize {
			canServe = true
		} else {
			for pieceCid, op := range pieceCidToServeMap {
				if op.isIdle() {
					log.Warnw("----delete idle tx car unsealed cache", "pieceCid", pieceCid)
					// delete
					op.stopServe()
					delete(pieceCidToServeMap, pieceCid)
					canServe = true
					break
				}
			}
		}
		if canServe {
			//new
			filePathCh := make(chan string, 1)
			op := newServeOperation(&txCarInfo, reqId, filePathCh, serveDone)
			pieceCidToServeMap[txCarInfo.PieceCid] = op
			pieceCidToServeMapLock.Unlock()

			log.Infow("----GetTxCarUnsealedCache.startServe", "reqId", reqId, "pieceCid", op.txCarInfo.PieceCid)

			errCh := make(chan error)
			go func() {
				err := op.startServe(ctx)
				if err != nil {
					log.Errorf("----GetTxCarUnsealedCache.startServe.exit with err:%v", err)
					errCh <- err
				}
			}()

			// wait file created
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case s := <-filePathCh:
				return s, nil
			case err := <-errCh:
				return "", err
			}
		}

		// cannot serve, just wait
		pieceCidToServeMapLock.Unlock()
		//wait
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
		pieceCidToServeMapLock.Lock()
	}
}
