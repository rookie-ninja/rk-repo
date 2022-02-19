package exchange

import (
	"github.com/rookie-ninja/rk-common/common"
	"time"
)

// Syncer interface
type Syncer interface {
	GetType() string

	Sync(*SyncReq) *SyncResp
}

// SyncReq defines request of Sync()
type SyncReq struct {
	BaseUnit string
}

// NewSyncResp create new SyncResp based on SyncReq
func NewSyncResp(req *SyncReq) *SyncResp {
	res := &SyncResp{
		BaseUnit: req.BaseUnit,
		Currency: make(map[string]float64),
	}

	res.Meta.RequestId = rkcommon.GenerateRequestId()
	res.Meta.SyncTime = time.Now()

	return res
}

// SyncResp defines response of Sync()
type SyncResp struct {
	Meta struct {
		RequestId  string
		Success    bool
		SourceType string
		SyncTime   time.Time
		Error      error
	}
	BaseUnit string
	Currency map[string]float64
}
