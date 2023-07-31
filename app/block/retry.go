package block

import (
	"log"
	"runtime"
	"time"

	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	q "github.com/denniswon/validationcloud/app/queue"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gammazero/workerpool"
	"gorm.io/gorm"
)

// RetryQueueManager - Pop oldest block number from Redis backed retry queue
// and try to fetch it in different go routine
//
// Sleeps for 500 milliseconds then repeat
func RetryQueueManager(client *ethclient.Client, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, status *d.StatusHolder) {
	sleep := func() {
		time.Sleep(time.Duration(512) * time.Millisecond)
	}

	// Creating worker pool and submitting jobs when there's `to be processed` blocks in retry queue
	wp := workerpool.New(runtime.NumCPU() * int(cfg.GetConcurrencyFactor()))
	defer wp.Stop()

	for {
		sleep()

		block, ok := queue.UnconfirmedNext()
		if !ok {
			continue
		}

		stat := queue.Stat()
		log.Printf("ℹ️ Retrying block : %d [ Unconfirmed : ( Progress : %d, Waiting : %d ) | Confirmed : ( Progress : %d, Waiting : %d ) | Total : %d ]\n", block, stat.UnconfirmedProgress, stat.UnconfirmedWaiting, stat.ConfirmedProgress, stat.ConfirmedWaiting, stat.Total)

		// Submitting block processor job into pool
		// which will be picked up & processed
		//
		// This will stop us from blindly creating too many go routines
		func(_blockNumber uint64, queue *q.BlockProcessorQueue) {

			wp.Submit(func() {

				if !FetchBlockByNumber(client, _blockNumber, _db, redis, true, queue, status) {

					queue.UnconfirmedFailed(_blockNumber)
					return

				}

				queue.UnconfirmedDone(_blockNumber)

			})

		}(block, queue)
	}
}
