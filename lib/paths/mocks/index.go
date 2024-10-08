// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/filecoin-project/curio/lib/paths (interfaces: SectorIndex)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	abi "github.com/filecoin-project/go-state-types/abi"

	paths "github.com/filecoin-project/curio/lib/paths"
	storiface "github.com/filecoin-project/curio/lib/storiface"

	fsutil "github.com/filecoin-project/lotus/storage/sealer/fsutil"
)

// MockSectorIndex is a mock of SectorIndex interface.
type MockSectorIndex struct {
	ctrl     *gomock.Controller
	recorder *MockSectorIndexMockRecorder
}

// MockSectorIndexMockRecorder is the mock recorder for MockSectorIndex.
type MockSectorIndexMockRecorder struct {
	mock *MockSectorIndex
}

// NewMockSectorIndex creates a new mock instance.
func NewMockSectorIndex(ctrl *gomock.Controller) *MockSectorIndex {
	mock := &MockSectorIndex{ctrl: ctrl}
	mock.recorder = &MockSectorIndexMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSectorIndex) EXPECT() *MockSectorIndexMockRecorder {
	return m.recorder
}

// BatchStorageDeclareSectors mocks base method.
func (m *MockSectorIndex) BatchStorageDeclareSectors(arg0 context.Context, arg1 []paths.SectorDeclaration) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchStorageDeclareSectors", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// BatchStorageDeclareSectors indicates an expected call of BatchStorageDeclareSectors.
func (mr *MockSectorIndexMockRecorder) BatchStorageDeclareSectors(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BatchStorageDeclareSectors", reflect.TypeOf((*MockSectorIndex)(nil).BatchStorageDeclareSectors), arg0, arg1)
}

// StorageAttach mocks base method.
func (m *MockSectorIndex) StorageAttach(arg0 context.Context, arg1 storiface.StorageInfo, arg2 fsutil.FsStat) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageAttach", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageAttach indicates an expected call of StorageAttach.
func (mr *MockSectorIndexMockRecorder) StorageAttach(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageAttach", reflect.TypeOf((*MockSectorIndex)(nil).StorageAttach), arg0, arg1, arg2)
}

// StorageBestAlloc mocks base method.
func (m *MockSectorIndex) StorageBestAlloc(arg0 context.Context, arg1 storiface.SectorFileType, arg2 abi.SectorSize, arg3 storiface.PathType, arg4 abi.ActorID) ([]storiface.StorageInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageBestAlloc", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].([]storiface.StorageInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageBestAlloc indicates an expected call of StorageBestAlloc.
func (mr *MockSectorIndexMockRecorder) StorageBestAlloc(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageBestAlloc", reflect.TypeOf((*MockSectorIndex)(nil).StorageBestAlloc), arg0, arg1, arg2, arg3, arg4)
}

// StorageDeclareSector mocks base method.
func (m *MockSectorIndex) StorageDeclareSector(arg0 context.Context, arg1 storiface.ID, arg2 abi.SectorID, arg3 storiface.SectorFileType, arg4 bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageDeclareSector", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageDeclareSector indicates an expected call of StorageDeclareSector.
func (mr *MockSectorIndexMockRecorder) StorageDeclareSector(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageDeclareSector", reflect.TypeOf((*MockSectorIndex)(nil).StorageDeclareSector), arg0, arg1, arg2, arg3, arg4)
}

// StorageDetach mocks base method.
func (m *MockSectorIndex) StorageDetach(arg0 context.Context, arg1 storiface.ID, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageDetach", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageDetach indicates an expected call of StorageDetach.
func (mr *MockSectorIndexMockRecorder) StorageDetach(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageDetach", reflect.TypeOf((*MockSectorIndex)(nil).StorageDetach), arg0, arg1, arg2)
}

// StorageDropSector mocks base method.
func (m *MockSectorIndex) StorageDropSector(arg0 context.Context, arg1 storiface.ID, arg2 abi.SectorID, arg3 storiface.SectorFileType) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageDropSector", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageDropSector indicates an expected call of StorageDropSector.
func (mr *MockSectorIndexMockRecorder) StorageDropSector(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageDropSector", reflect.TypeOf((*MockSectorIndex)(nil).StorageDropSector), arg0, arg1, arg2, arg3)
}

// StorageFindSector mocks base method.
func (m *MockSectorIndex) StorageFindSector(arg0 context.Context, arg1 abi.SectorID, arg2 storiface.SectorFileType, arg3 abi.SectorSize, arg4 bool) ([]storiface.SectorStorageInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageFindSector", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].([]storiface.SectorStorageInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageFindSector indicates an expected call of StorageFindSector.
func (mr *MockSectorIndexMockRecorder) StorageFindSector(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageFindSector", reflect.TypeOf((*MockSectorIndex)(nil).StorageFindSector), arg0, arg1, arg2, arg3, arg4)
}

// StorageGetLocks mocks base method.
func (m *MockSectorIndex) StorageGetLocks(arg0 context.Context) (storiface.SectorLocks, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageGetLocks", arg0)
	ret0, _ := ret[0].(storiface.SectorLocks)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageGetLocks indicates an expected call of StorageGetLocks.
func (mr *MockSectorIndexMockRecorder) StorageGetLocks(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageGetLocks", reflect.TypeOf((*MockSectorIndex)(nil).StorageGetLocks), arg0)
}

// StorageInfo mocks base method.
func (m *MockSectorIndex) StorageInfo(arg0 context.Context, arg1 storiface.ID) (storiface.StorageInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageInfo", arg0, arg1)
	ret0, _ := ret[0].(storiface.StorageInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageInfo indicates an expected call of StorageInfo.
func (mr *MockSectorIndexMockRecorder) StorageInfo(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageInfo", reflect.TypeOf((*MockSectorIndex)(nil).StorageInfo), arg0, arg1)
}

// StorageList mocks base method.
func (m *MockSectorIndex) StorageList(arg0 context.Context) (map[storiface.ID][]storiface.Decl, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageList", arg0)
	ret0, _ := ret[0].(map[storiface.ID][]storiface.Decl)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageList indicates an expected call of StorageList.
func (mr *MockSectorIndexMockRecorder) StorageList(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageList", reflect.TypeOf((*MockSectorIndex)(nil).StorageList), arg0)
}

// StorageLock mocks base method.
func (m *MockSectorIndex) StorageLock(arg0 context.Context, arg1 abi.SectorID, arg2, arg3 storiface.SectorFileType) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageLock", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageLock indicates an expected call of StorageLock.
func (mr *MockSectorIndexMockRecorder) StorageLock(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageLock", reflect.TypeOf((*MockSectorIndex)(nil).StorageLock), arg0, arg1, arg2, arg3)
}

// StorageReportHealth mocks base method.
func (m *MockSectorIndex) StorageReportHealth(arg0 context.Context, arg1 storiface.ID, arg2 storiface.HealthReport) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageReportHealth", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// StorageReportHealth indicates an expected call of StorageReportHealth.
func (mr *MockSectorIndexMockRecorder) StorageReportHealth(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageReportHealth", reflect.TypeOf((*MockSectorIndex)(nil).StorageReportHealth), arg0, arg1, arg2)
}

// StorageTryLock mocks base method.
func (m *MockSectorIndex) StorageTryLock(arg0 context.Context, arg1 abi.SectorID, arg2, arg3 storiface.SectorFileType) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageTryLock", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageTryLock indicates an expected call of StorageTryLock.
func (mr *MockSectorIndexMockRecorder) StorageTryLock(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageTryLock", reflect.TypeOf((*MockSectorIndex)(nil).StorageTryLock), arg0, arg1, arg2, arg3)
}
