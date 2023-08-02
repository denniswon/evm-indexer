package data

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// SyncState - Whether `validationcloud` is synced with blockchain or not
type SyncState struct {
	Done                    uint64
	StartedAt               time.Time
	BlockCountAtStartUp     uint64
	MaxBlockNumberAtStartUp uint64
	NewBlocksInserted       uint64
	LatestBlockNumber       uint64
}

// BlockCountInDB - Blocks currently present in database
func (s *SyncState) BlockCountInDB() uint64 {
	return s.BlockCountAtStartUp + s.NewBlocksInserted
}

// StatusHolder - Keeps track of progress. To be delivered when `/v1/synced` is queried
type StatusHolder struct {
	State *SyncState
	Mutex *sync.RWMutex
}

// MaxBlockNumberAtStartUp - thread safe read latest block number at the time of service start
// To determine whether a missing block related notification needs to be sent on a pubsub channel or not
func (s *StatusHolder) MaxBlockNumberAtStartUp() uint64 {

	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	return s.State.MaxBlockNumberAtStartUp

}

// SetStartedAt - Sets started at time
func (s *StatusHolder) SetStartedAt() {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.State.StartedAt = time.Now().UTC()

}

// IncrementBlocksInserted - thread safe increments number of blocks inserted into DB since start
func (s *StatusHolder) IncrementBlocksInserted() {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.State.NewBlocksInserted++

}

// IncrementBlocksProcessed - thread safe increments number of blocks processed by after it started
func (s *StatusHolder) IncrementBlocksProcessed() {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.State.Done++

}

// BlockCountInDB - thread safe reads currently present blocks in db
func (s *StatusHolder) BlockCountInDB() uint64 {

	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	return s.State.BlockCountInDB()

}

// ElapsedTime - thread safe uptime of the service
func (s *StatusHolder) ElapsedTime() time.Duration {

	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	return time.Now().UTC().Sub(s.State.StartedAt)

}

// Done - thread safe  #-of Blocks processed during uptime i.e. after last time it started
func (s *StatusHolder) Done() uint64 {

	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	return s.State.Done

}

// GetLatestBlockNumber - thread safe read latest block number
func (s *StatusHolder) GetLatestBlockNumber() uint64 {

	s.Mutex.RLock()
	defer s.Mutex.RUnlock()

	return s.State.LatestBlockNumber

}

// SetLatestBlockNumber - thread safe write latest block number
func (s *StatusHolder) SetLatestBlockNumber(num uint64) {

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.State.LatestBlockNumber = num

}

// RedisInfo
type RedisInfo struct {
	Client *redis.Client
	BlockPublishTopic, TxPublishTopic, EventPublishTopic string
}

// ResultStatus
type ResultStatus struct {
	Success uint64
	Failure uint64
}

// Total - Returns total count of operations which were supposed to be performed
//
// To check whether all go routines have sent their status i.e. completed their tasks or not
func (r ResultStatus) Total() uint64 {
	return r.Success + r.Failure
}

// Job - For running a block fetching job
type Job struct {
	Client *ethclient.Client
	DB     *gorm.DB
	Redis  *RedisInfo
	Block  uint64
	Status *StatusHolder
}

// BlockChainNodeConnection - Holds network connection object for blockchain nodes
//
// Use `RPC` i.e. HTTP based connection, for querying blockchain for data
// Use `Websocket` for real-time listening of events in blockchain
type BlockChainNodeConnection struct {
	RPC       *ethclient.Client
	Websocket *ethclient.Client
}
