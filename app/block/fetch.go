package block

import (
	"context"
	"log"
	"math/big"
	"time"

	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	q "github.com/denniswon/validationcloud/app/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

// FetchBlockByHash - Fetching block content using blockHash
func FetchBlockByHash(client *ethclient.Client, hash common.Hash, number string, _db *gorm.DB, redis *d.RedisInfo, queue *q.BlockProcessorQueue, _status *d.StatusHolder) bool {

	// Starting block processing at
	startingAt := time.Now().UTC()

	block, err := client.BlockByHash(context.Background(), hash)
	if err != nil {

		log.Printf("Failed to fetch block %s : %s\n", number, err.Error())
		return false

	}

	return ProcessBlockContent(client, block, _db, redis, true, queue, _status, startingAt)

}

// FetchBlockByNumber - Fetching block content using block number
func FetchBlockByNumber(client *ethclient.Client, number uint64, _db *gorm.DB, redis *d.RedisInfo, publishable bool, queue *q.BlockProcessorQueue, _status *d.StatusHolder) bool {

	// Starting block processing at
	startingAt := time.Now().UTC()

	_num := big.NewInt(0)
	_num.SetUint64(number)

	block, err := client.BlockByNumber(context.Background(), _num)
	if err != nil {

		log.Printf("Failed to fetch block %d : %s\n", number, err)
		return false

	}

	return ProcessBlockContent(client, block, _db, redis, publishable, queue, _status, startingAt)

}

// FetchTransactionByHash - Fetching specific transaction related data, tries to publish data if required
// & lets listener go routine know about all tx, event data it collected while processing this tx,
// which will be attempted to be stored in database
func FetchTransactionByHash(client *ethclient.Client, block *types.Block, tx *types.Transaction, _db *gorm.DB, redis *d.RedisInfo, _status *d.StatusHolder, returnValChan chan *db.PackedTransaction) {

	receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		// log.Printf("Failed to fetch tx receipt for %s [ block : %d ] : %s\n", tx.Hash().Hex(), block.NumberU64(), err.Error())

		// Passing nil, to denote, failed to fetch all tx data
		// from blockchain node
		returnValChan <- nil
		return
	}

	// log.Printf("Transaction receipt for %s %v\n", tx.Hash().Hex(), receipt)

	// Supports EIP-2930 and EIP-2718 and EIP-1559 and EIP-155 and legacy transactions
    message, err := tx.AsMessage(types.LatestSignerForChainID(tx.ChainId()), block.BaseFee())
	if err != nil {
		log.Printf("Failed to get tx as msg[ block : %d ] : %s\n", block.NumberU64(), err.Error())
		// Passing nil, to denote, failed to fetch all tx data from blockchain node
		returnValChan <- nil
	}
	sender := message.From()

	// Weird: issue with getting transaction sender using ethclient

	// sender, err := client.TransactionSender(context.Background(), tx, block.Hash(), receipt.TransactionIndex)
	// if err != nil {
	// 	log.Printf("Failed to fetch tx sender [ block : %d ] : %s\n", block.NumberU64(), err.Error())

	// 	// Passing nil, to denote, failed to fetch all tx data
	// 	// from blockchain node
	// 	returnValChan <- nil
	// 	return
	// }

	// Passing all tx related data to listener go routine
	// so that it can attempt to store whole block data
	// into database
	returnValChan <- BuildPackedTx(tx, sender, receipt)
}

