package spectrestratum

import (
	"math/big"
	"sync"
	"time"

	"github.com/spectre-project/spectre-stratum-bridge/src/gostratum"
	"github.com/spectre-project/spectred/app/appmessage"
)

const maxjobs = 256

type MiningState struct {
	Jobs        map[uint64]*appmessage.RPCBlock
	JobLock     sync.Mutex
	jobCounter  uint64
	bigDiff     big.Int
	initialized bool
	connectTime time.Time
	stratumDiff *spectreDiff
}

func MiningStateGenerator() any {
	return &MiningState{
		Jobs:        make(map[uint64]*appmessage.RPCBlock, maxjobs),
		JobLock:     sync.Mutex{},
		connectTime: time.Now(),
	}
}

func GetMiningState(ctx *gostratum.StratumContext) *MiningState {
	return ctx.State.(*MiningState)
}

func (ms *MiningState) AddJob(job *appmessage.RPCBlock) uint64 {
	ms.JobLock.Lock()
	ms.jobCounter++
	idx := ms.jobCounter
	ms.Jobs[idx%maxjobs] = job
	ms.JobLock.Unlock()
	return idx
}

func (ms *MiningState) GetJob(id uint64) (*appmessage.RPCBlock, bool) {
	ms.JobLock.Lock()
	job, exists := ms.Jobs[id%maxjobs]
	ms.JobLock.Unlock()
	return job, exists
}
