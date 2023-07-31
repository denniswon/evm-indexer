package app

import (
	"log"
	"sync"

	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	q "github.com/denniswon/validationcloud/app/queue"
	"gorm.io/gorm"
)

// Setting ground up i.e. acquiring resources required & determining with
// some basic checks whether we can proceed to next step or not
func bootstrap(configFile string) (*d.BlockChainNodeConnection, *gorm.DB, *d.StatusHolder, *q.BlockProcessorQueue) {

	err := cfg.Read(configFile)
	if err != nil {
		log.Fatalf("[!] Failed to read `.env` : %s\n", err.Error())
	}

	// Maintaining both HTTP & Websocket based connection to blockchain
	_connection := &d.BlockChainNodeConnection{
		RPC:       getClient(true),
		Websocket: getClient(false),
	}

	_db := db.Connect()

	_status := &d.StatusHolder{
		State: &d.SyncState{
			BlockCountAtStartUp:     db.GetBlockCount(_db),
			MaxBlockNumberAtStartUp: db.GetCurrentBlockNumber(_db),
		},
		Mutex: &sync.RWMutex{},
	}

	// block processor queue
	_queue := q.New(db.GetCurrentBlockNumber(_db))

	return _connection, _db, _status, _queue
}
