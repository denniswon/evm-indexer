package app

import (
	"context"
	"log"
	"sync"

	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	q "github.com/denniswon/validationcloud/app/queue"
	"github.com/denniswon/validationcloud/app/rest/graph"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Setting ground up i.e. acquiring resources required & determining with
// some basic checks whether we can proceed to next step or not
func bootstrap(configFile string) (*d.BlockChainNodeConnection, *redis.Client, *d.RedisInfo, *gorm.DB, *d.StatusHolder, *q.BlockProcessorQueue) {

	err := cfg.Read(configFile)
	if err != nil {
		log.Fatalf("[!] Failed to read `.env` : %s\n", err.Error())
	}

	// Maintaining both HTTP & Websocket based connection to blockchain
	_connection := &d.BlockChainNodeConnection{
		RPC:       getClient(true),
		Websocket: getClient(false),
	}

	_redisClient := getRedisClient()

	if _redisClient == nil {
		log.Fatalf("[!] Failed to connect to Redis Server\n")
	}

	if err := _redisClient.FlushAll(context.Background()).Err(); err != nil {
		log.Printf("[!] Failed to flush all keys from redis : %s\n", err.Error())
	}

	_db := db.Connect()

	// Passing db handle to graph for resolving graphQL queries
	graph.GetDatabaseConnection(_db)

	_status := &d.StatusHolder{
		State: &d.SyncState{
			BlockCountAtStartUp:     db.GetBlockCount(_db),
			MaxBlockNumberAtStartUp: db.GetCurrentBlockNumber(_db),
		},
		Mutex: &sync.RWMutex{},
	}

	_redisInfo := &d.RedisInfo{
		Client:            _redisClient,
		BlockPublishTopic: "block",
		TxPublishTopic:    "transaction",
		EventPublishTopic: "event",
	}

	// block processor queue
	_queue := q.New(db.GetCurrentBlockNumber(_db))

	return _connection, _redisClient, _redisInfo, _db, _status, _queue
}
