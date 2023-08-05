package block

import (
	"log"
	"runtime"
	"sort"
	"time"

	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	q "github.com/denniswon/validationcloud/app/queue"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gammazero/workerpool"
	"github.com/gookit/color"
	"gorm.io/gorm"
)

// FindMissingBlocksInRange - Given ascending ordered block numbers read from DB
// attempts to find out which numbers are missing in [from, to] range inclusive
func FindMissingBlocksInRange(found []uint64, from uint64, to uint64) []uint64 {

	absent := make([]uint64, 0, to - from + 1)

	for b := from; b <= to; b++ {

		idx := sort.Search(len(found), func(j int) bool { return found[j] >= b })

		if !(idx < len(found) && found[idx] == b) {
			absent = append(absent, b)
		}

	}

	return absent

}

// Syncer - Given ascending block number range i.e. fromBlock <= toBlock
// attempts to fetch missing blocks in that range
// while running n workers concurrently, where n = number of cores this machine has
//
// Waits for all of them to complete
func Syncer(client *ethclient.Client, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, fromBlock uint64, toBlock uint64, status *d.StatusHolder, jd func(*workerpool.WorkerPool, *d.Job, *q.BlockProcessorQueue)) {
	if !(fromBlock <= toBlock) {
		log.Print(color.Red.Sprintf("[!] Bad block range for syncer"))
		return
	}

	wp := workerpool.New(runtime.NumCPU() * int(cfg.GetConcurrencyFactor()))

	// Jobs need to be submitted using this interface, while
	// just mentioning which block needs to be fetched
	job := func(num uint64) {
		jd(wp, &d.Job{
			Client: client,
			DB:     _db,
			Redis:  redis,
			Block:  num,
			Status: status,
		}, queue)
	}

	// attempting to fetch X blocks at a time
	var step uint64 = 10000

	for i := fromBlock; i <= toBlock; i += step {

		to := i + step - 1
		if to > toBlock {
			to = toBlock
		}

		blocks := db.GetAllBlockNumbersInRange(_db, i, to)

		// No blocks present in DB, in queried range
		if len(blocks) == 0 {

			// submit all of them to job processor queue
			for j := i; j <= to; j++ {
				job(j)
			}
			continue

		}

		// All blocks in range present in DB
		if to - i + 1 == uint64(len(blocks)) {
			continue
		}

		// Some blocks are missing in range, attempting to find them
		// and pushing their processing request to job queue
		for _, v := range FindMissingBlocksInRange(blocks, i, to) {
			job(v)
		}

	}

	wp.StopWait()
}

// SyncBlocksByRange - Fetch & persist all blocks in range(fromBlock, toBlock), both inclusive
//
// Range can be either ascending or descending, depending upon that proper arguments to be
// passed to `Syncer` function during invokation
func SyncBlocksByRange(client *ethclient.Client, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, fromBlock uint64, toBlock uint64, status *d.StatusHolder) {

	// Job to be submitted and executed by each worker
	//
	// Job specification is provided in `Job` struct
	job := func(wp *workerpool.WorkerPool, j *d.Job, queue *q.BlockProcessorQueue) {

		wp.Submit(func() {

			if !queue.Put(j.Block) {
				return
			}

			if !FetchBlockByNumber(j.Client, j.Block, j.DB, j.Redis, false, queue, j.Status) {
				queue.UnconfirmedFailed(j.Block)
				return
			}

			queue.UnconfirmedDone(j.Block)

		})
	}

	log.Printf("Starting block syncer\n")

	if fromBlock < toBlock {
		Syncer(client, _db, redis, queue, fromBlock, toBlock, status, job)
	} else {
		Syncer(client, _db, redis, queue, toBlock, fromBlock, status, job)
	}

	log.Printf("Stopping block syncer\n")

	// Once completed first iteration of processing blocks upto last time where it left
	// off, we're going to start worker to look at DB & decide which blocks are missing
	// i.e. need to be fetched again
	//
	// And this will itself run as a infinite job, completes one iteration &
	// takes break for 1 min, then repeats
	go SyncMissingBlocksInDB(client, _db, redis, queue, status)

}

// SyncMissingBlocksInDB - Checks with database for what blocks are present & what are not, fetches missing
// blocks & related data iteratively
func SyncMissingBlocksInDB(client *ethclient.Client, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, status *d.StatusHolder) {

	for {

		log.Printf("Starting missing block finder\n")

		currentBlockNumber := db.GetCurrentBlockNumber(_db)

		// Safely reading shared variable
		blockCount := status.BlockCountInDB()

		// If all blocks present in between 0 to latest block in network
		// the service sleeps for 1 minute & again get to work
		if currentBlockNumber+1 == blockCount {
			log.Printf("No missing blocks found\n")

			<-time.After(time.Duration(1) * time.Minute)
			continue
		}

		// Job to be submitted and executed by each worker
		//
		// Job specification is provided in `Job` struct
		job := func(wp *workerpool.WorkerPool, j *d.Job, queue *q.BlockProcessorQueue) {

			wp.Submit(func() {

				// Worker fetches block by number from local storage
				block := db.GetBlock(j.DB, j.Block)
				if !(block == nil) {
					return
				}

				if !queue.Put(j.Block) {
					return
				}

				if !FetchBlockByNumber(j.Client, j.Block, j.DB, j.Redis, false, queue, j.Status) {
					queue.UnconfirmedFailed(j.Block)
					return
				}

				queue.UnconfirmedDone(j.Block)

			})

		}

		Syncer(client, _db, redis, queue, 0, currentBlockNumber, status, job)

		log.Printf("Stopping missing block finder\n")
		<-time.After(time.Duration(1) * time.Minute)

	}

}
