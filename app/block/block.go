package block

import (
	"log"
	"runtime"
	"time"

	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	q "github.com/denniswon/validationcloud/app/queue"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gammazero/workerpool"
	"gorm.io/gorm"
)

// ProcessBlockContent - Processes everything inside this block i.e. block data, tx data, event data
func ProcessBlockContent(client *ethclient.Client, block *types.Block, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, status *d.StatusHolder, startingAt time.Time) bool {

	if block.Transactions().Len() == 0 {

		// Constructing block data to be persisted
		//
		// This is what we just published on pubsub channel
		packedBlock := BuildPackedBlock(block, nil)

		// If block doesn't contain any tx, we'll attempt to persist only block
		if err := db.StoreBlock(_db, packedBlock, status, queue); err != nil {

			log.Printf("Failed to process block %d : %s\n", block.NumberU64(), err.Error())
			return false

		}

		// Successfully processed block
		log.Printf("Block %d with 0 tx(s) [ Took : %s ]\n", block.NumberU64(), time.Now().UTC().Sub(startingAt))
		status.IncrementBlocksProcessed()

		return true

	}

	// Communication channel to be shared between multiple executing go routines
	// which are trying to fetch all tx(s) present in block, concurrently
	returnValChan := make(chan *db.PackedTransaction, runtime.NumCPU() * int(cfg.GetConcurrencyFactor()))

	// -- Tx processing starting
	// Creating job processor queue which will process all tx(s), concurrently
	wp := workerpool.New(runtime.NumCPU() * int(cfg.GetConcurrencyFactor()))

	// Concurrently trying to process all tx(s) for this block, in hope of better performance
	for _, v := range block.Transactions() {

		// Concurrently trying to fetch multiple tx(s) present in block
		// and expecting their return value to be published on shared channel
		//
		// Which is being read
		func(tx *types.Transaction) {
			wp.Submit(func() {

				FetchTransactionByHash(client,
					block,
					tx,
					_db,
					redis,
					status,
					returnValChan)

			})
		}(v)

	}

	// Keeping track of how many of these tx fetchers succeded & how many of them failed
	result := d.ResultStatus{}
	// Data received from tx fetchers, to be stored here
	packedTxs := make([]*db.PackedTransaction, block.Transactions().Len())

	for v := range returnValChan {
		if v != nil {
			result.Success++
		} else {
			result.Failure++
		}

		// #-of tx fetchers completed their job till now
		//
		// Either successfully or failed some how
		total := int(result.Total())
		// Storing tx data received from just completed go routine
		packedTxs[total-1] = v

		// All go routines have completed their job
		if total == block.Transactions().Len() {
			break
		}
	}

	// Stopping job processor forcefully
	// because by this time all jobs have been completed
	//
	// Otherwise control flow will not be able to come here
	// it'll keep looping in 👆 loop, reading from channel
	wp.Stop()
	// -- Tx processing ending

	if !(result.Failure == 0) {
		return false
	}

	// Constructing block data to be persisted
	//
	// This is what we just published on pubsub channel
	packedBlock := BuildPackedBlock(block, packedTxs)

	// If block doesn't contain any tx, we'll attempt to persist only block
	if err := db.StoreBlock(_db, packedBlock, status, queue); err != nil {

		log.Printf("Failed to process block %d : %s\n", block.NumberU64(), err.Error())
		return false

	}

	// Successfully processed block
	log.Printf("Block %d with %d tx(s) [ Took : %s ]\n", block.NumberU64(), block.Transactions().Len(), time.Now().UTC().Sub(startingAt))

	status.IncrementBlocksProcessed()
	return true

}
