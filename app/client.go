package app

import (
	"log"

	cfg "github.com/denniswon/validationcloud/app/config"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Connect to blockchain node, either using HTTP or Websocket connection
// depending upon true/ false, passed to function, respectively
func getClient(isRPC bool) *ethclient.Client {
	var client *ethclient.Client
	var err error

	if isRPC {
		client, err = ethclient.Dial(cfg.Get("RPCUrl"))
	} else {
		client, err = ethclient.Dial(cfg.Get("WebsocketUrl"))
	}

	if err != nil {
		log.Fatalf("[!] Failed to connect to blockchain : %s\n", err.Error())
	}

	return client
}
