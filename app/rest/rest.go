package rest

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"

	cmn "github.com/denniswon/validationcloud/app/common"
	cfg "github.com/denniswon/validationcloud/app/config"
	d "github.com/denniswon/validationcloud/app/data"
	"github.com/denniswon/validationcloud/app/db"
	ps "github.com/denniswon/validationcloud/app/pubsub"
	"github.com/denniswon/validationcloud/app/rest/graph/generated"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/denniswon/validationcloud/app/rest/graph"
)

// RunHTTPServer - Holds definition for all REST API(s) to be exposed
func RunHTTPServer(_db *gorm.DB, _status *d.StatusHolder, _redisClient *redis.Client) {

	respondWithJSON := func(data []byte, c *gin.Context) {
		if data != nil {
			c.Data(http.StatusOK, "application/json", data)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"msg": "JSON encoding failed",
		})
	}

	// Checking if webserver in production mode or not
	checkIfInProduction := func() bool {
		return strings.ToLower(cfg.Get("Production")) == "yes"
	}

	// Running in production/ debug mode depending upon
	// config specified in .env file
	if checkIfInProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.Default()

	// enabled cors
	router.Use(cors.Default())

	grp := router.Group("/v1")

	{

		// For checking the service's syncing status
		grp.GET("/synced", func(c *gin.Context) {

			currentBlockNumber := _status.GetLatestBlockNumber()
			blockCountInDB := _status.BlockCountInDB()
			remaining := (currentBlockNumber + 1) - blockCountInDB
			elapsed := _status.ElapsedTime()

			status := fmt.Sprintf("%.2f %%", (float64(blockCountInDB)/float64(currentBlockNumber+1))*100)
			eta := "0s"
			if remaining > 0 {
				eta = (time.Duration((elapsed.Seconds()/float64(_status.Done()))*float64(remaining)) * time.Second).String()
			}

			c.JSON(http.StatusOK, gin.H{
				"synced":    status,
				"processed": _status.Done(),
				"elapsed":   elapsed.String(),
				"eta":       eta,
				"status":	_status.State,
			})

		})

		// Query block data using block hash/ number/ block number range ( 10 at max )
		grp.GET("/block", func(c *gin.Context) {

			hash := c.Query("hash")
			number := c.Query("number")
			tx := c.Query("tx")

			// Block hash based all tx retrieval request handler
			if strings.HasPrefix(hash, "0x") && len(hash) == 66 && tx == "yes" {
				if tx := db.GetTransactionsByBlockHash(_db, common.HexToHash(hash)); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// Given block number, finds out all tx(s) present in that block
			if number != "" && tx == "yes" {

				_num, err := cmn.ParseNumber(number)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number",
					})
					return
				}

				if tx := db.GetTransactionsByBlockNumber(_db, _num); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// Block hash based single block retrieval request handler
			if strings.HasPrefix(hash, "0x") && len(hash) == 66 {
				if block := db.GetBlockByHash(_db, common.HexToHash(hash)); block != nil {
					respondWithJSON(block.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// Block number based single block retrieval request handler
			if number != "" {

				_num, err := cmn.ParseNumber(number)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number",
					})
					return
				}

				if block := db.GetBlockByNumber(_db, _num); block != nil {
					respondWithJSON(block.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// Block number range based query
			// At max 10 blocks at a time to be returned
			fromBlock := c.Query("fromBlock")
			toBlock := c.Query("toBlock")

			if fromBlock != "" && toBlock != "" {

				_from, _to, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if blocks := db.GetBlocksByNumberRange(_db, _from, _to); blocks != nil {
					respondWithJSON(blocks.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// Query blocks by timestamp range, at max 60 seconds of timestamp
			// can be mentioned, otherwise request to be rejected
			fromTime := c.Query("fromTime")
			toTime := c.Query("toTime")

			if fromTime != "" && toTime != "" {

				_from, _to, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if blocks := db.GetBlocksByTimeRange(_db, _from, _to); blocks != nil {
					respondWithJSON(blocks.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"msg": "Bad query param(s)",
			})

		})

		// Transaction fetch ( by query params ) request handler
		grp.GET("/transaction", func(c *gin.Context) {

			hash := c.Query("hash")

			// Simply returns single tx object, when queried using tx hash
			if strings.HasPrefix(hash, "0x") && len(hash) == 66 {
				if tx := db.GetTransactionByHash(_db, common.HexToHash(hash)); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return
			}

			// block number range
			fromBlock := c.Query("fromBlock")
			toBlock := c.Query("toBlock")

			// block time span range
			fromTime := c.Query("fromTime")
			toTime := c.Query("toTime")

			// Contract deployer address
			deployer := c.Query("deployer")

			// account pair, in between this pair tx(s) to be extracted out in time span range/ block number range
			//
			// only single one can be supplied to enforce tx search
			// for incoming/ outgoing tx to & from an account
			fromAccount := c.Query("fromAccount")
			toAccount := c.Query("toAccount")

			// Account nonce, to be used for finding
			// tx, in combination with `fromAccount`
			nonce := c.Query("nonce")

			// Responds with tx sent from account with specified nonce
			if nonce != "" && strings.HasPrefix(fromAccount, "0x") && len(fromAccount) == 42 {

				_nonce, err := cmn.ParseNumber(nonce)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad account nonce",
					})
					return
				}

				if tx := db.GetTransactionFromAccountWithNonce(_db, common.HexToAddress(fromAccount), _nonce); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Responds with all contract creation tx(s) sent from specific account
			// ( i.e. deployer ) with in given block number range
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(deployer, "0x") && len(deployer) == 42 {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if tx := db.GetContractCreationTransactionsFromAccountByBlockNumberRange(_db, common.HexToAddress(deployer), _fromBlock, _toBlock); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Responds with all contract creation tx(s) sent from specific account
			// ( i.e. deployer ) with in given time frame
			if fromTime != "" && toTime != "" && strings.HasPrefix(deployer, "0x") && len(deployer) == 42 {

				_fromTime, _toTime, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if tx := db.GetContractCreationTransactionsFromAccountByBlockTimeRange(_db, common.HexToAddress(deployer), _fromTime, _toTime); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number range & a pair of accounts, can find out all tx performed
			// between that pair, where `from` & `to` fields are fixed
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(fromAccount, "0x") && len(fromAccount) == 42 && strings.HasPrefix(toAccount, "0x") && len(toAccount) == 42 {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if tx := db.GetTransactionsBetweenAccountsByBlockNumberRange(_db, common.HexToAddress(fromAccount), common.HexToAddress(toAccount), _fromBlock, _toBlock); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block time range & a pair of accounts, can find out all tx performed
			// between that pair, where `from` & `to` fields are fixed
			if fromTime != "" && toTime != "" && strings.HasPrefix(fromAccount, "0x") && len(fromAccount) == 42 && strings.HasPrefix(toAccount, "0x") && len(toAccount) == 42 {

				_fromTime, _toTime, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if tx := db.GetTransactionsBetweenAccountsByBlockTimeRange(_db, common.HexToAddress(fromAccount), common.HexToAddress(toAccount), _fromTime, _toTime); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number range & account, can find out all tx performed
			// from account
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(fromAccount, "0x") && len(fromAccount) == 42 {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if tx := db.GetTransactionsFromAccountByBlockNumberRange(_db, common.HexToAddress(fromAccount), _fromBlock, _toBlock); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block mining time stamp range & account address, returns all outgoing tx
			// from this account in that given time span
			if fromTime != "" && toTime != "" && strings.HasPrefix(fromAccount, "0x") && len(fromAccount) == 42 {

				_fromTime, _toTime, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if tx := db.GetTransactionsFromAccountByBlockTimeRange(_db, common.HexToAddress(fromAccount), _fromTime, _toTime); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number range & account address, returns all incoming tx
			// to this account in that block range
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(toAccount, "0x") && len(toAccount) == 42 {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if tx := db.GetTransactionsToAccountByBlockNumberRange(_db, common.HexToAddress(toAccount), _fromBlock, _toBlock); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block mining time stamp range & account address, returns all incoming tx
			// to this account in that given time span
			if fromTime != "" && toTime != "" && strings.HasPrefix(toAccount, "0x") && len(toAccount) == 42 {

				_fromTime, _toTime, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if tx := db.GetTransactionsToAccountByBlockTimeRange(_db, common.HexToAddress(toAccount), _fromTime, _toTime); tx != nil {
					respondWithJSON(tx.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			c.JSON(http.StatusBadRequest, gin.H{
				"msg": "Bad query param(s)",
			})

		})

		// Event(s) fetched by query params handler end point
		grp.GET("/event", func(c *gin.Context) {

			fromBlock := c.Query("fromBlock")
			toBlock := c.Query("toBlock")

			fromTime := c.Query("fromTime")
			toTime := c.Query("toTime")

			contract := c.Query("contract")

			count := c.Query("count")

			topic0 := c.Query("topic0")
			topic1 := c.Query("topic1")
			topic2 := c.Query("topic2")
			topic3 := c.Query("topic3")

			blockHash := c.Query("blockHash")
			txHash := c.Query("txHash")

			// specific event log which under is requesting
			logIndex := c.Query("logIndex")
			// specific block number which is being asked to look inside
			// for finding 👆 event log inside block
			blockNumber := c.Query("blockNumber")

			// Given block hash and log index in block attempts to find out event
			// which satisfies that criteria
			if logIndex != "" && strings.HasPrefix(blockHash, "0x") && len(blockHash) == 66 {

				_logIndex, err := strconv.ParseUint(logIndex, 10, 64)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad log index",
					})
					return
				}

				if event := db.GetEventByBlockHashAndLogIndex(_db, common.HexToHash(blockHash), uint(_logIndex)); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number and log index in block attempts to find out event
			// which satisfies that criteria
			if logIndex != "" && blockNumber != "" {

				_blockNumber, err := strconv.ParseUint(blockNumber, 10, 64)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number",
					})
					return
				}

				_logIndex, err := strconv.ParseUint(logIndex, 10, 64)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad log index",
					})
					return
				}

				if event := db.GetEventByBlockNumberAndLogIndex(_db, _blockNumber, uint(_logIndex)); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given blockhash, retrieves all events emitted by tx present in block
			if strings.HasPrefix(blockHash, "0x") && len(blockHash) == 66 {

				if event := db.GetEventsByBlockHash(_db, common.HexToHash(blockHash)); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given txhash, retrieves all events emitted by that tx ( i.e. during tx execution )
			if strings.HasPrefix(txHash, "0x") && len(txHash) == 66 {

				if event := db.GetEventsByTransactionHash(_db, common.HexToHash(txHash)); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Finds out last `x` events emitted by contract
			if count != "" && strings.HasPrefix(contract, "0x") && len(contract) == 42 {

				_count, err := strconv.Atoi(count)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad event count",
					})
					return
				}

				if _count > 50 {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Too many events requested",
					})
					return
				}

				if event := db.GetLastXEventsFromContract(_db, common.HexToAddress(contract), _count); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number range, contract address & topics of log event, returns
			// events satisfying criteria
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(contract, "0x") && len(contract) == 42 && ((strings.HasPrefix(topic0, "0x") && len(topic0) == 66) || (strings.HasPrefix(topic1, "0x") && len(topic1) == 66) || (strings.HasPrefix(topic2, "0x") && len(topic2) == 66) || (strings.HasPrefix(topic3, "0x") && len(topic3) == 66)) {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {

					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return

				}

				topics := cmn.CreateEventTopicMap([]string{topic0, topic1, topic2, topic3})
				if len(topics) == 0 {

					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad event topic signature(s)",
					})
					return

				}

				if event := db.GetEventsFromContractWithTopicsByBlockNumberRange(_db, common.HexToAddress(contract), _fromBlock, _toBlock, topics); event != nil {

					respondWithJSON(event.ToJSON(), c)
					return

				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given time span, contract address & topic 0 of log event, returns
			// events satisfying criteria
			if fromTime != "" && toTime != "" && strings.HasPrefix(contract, "0x") && len(contract) == 42 && ((strings.HasPrefix(topic0, "0x") && len(topic0) == 66) || (strings.HasPrefix(topic1, "0x") && len(topic1) == 66) || (strings.HasPrefix(topic2, "0x") && len(topic2) == 66) || (strings.HasPrefix(topic3, "0x") && len(topic3) == 66)) {

				_fromTime, _toTime, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {

					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return

				}

				topics := cmn.CreateEventTopicMap([]string{topic0, topic1, topic2, topic3})
				if len(topics) == 0 {

					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad event topic signature(s)",
					})
					return

				}

				if event := db.GetEventsFromContractWithTopicsByBlockTimeRange(_db, common.HexToAddress(contract), _fromTime, _toTime, topics); event != nil {

					respondWithJSON(event.ToJSON(), c)
					return

				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block number range & contract address, finds out all events emitted by this contract
			if fromBlock != "" && toBlock != "" && strings.HasPrefix(contract, "0x") && len(contract) == 42 {

				_fromBlock, _toBlock, err := cmn.RangeChecker(fromBlock, toBlock, cfg.GetBlockNumberRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block number range",
					})
					return
				}

				if event := db.GetEventsFromContractByBlockNumberRange(_db, common.HexToAddress(contract), _fromBlock, _toBlock); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			// Given block time span & contract address, returns a list of
			// events emitted by this contract during time span
			if fromTime != "" && toTime != "" && strings.HasPrefix(contract, "0x") && len(contract) == 42 {

				_fromTime, _toTime, err := cmn.RangeChecker(fromTime, toTime, cfg.GetTimeRange())
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"msg": "Bad block time range",
					})
					return
				}

				if event := db.GetEventsFromContractByBlockTimeRange(_db, common.HexToAddress(contract), _fromTime, _toTime); event != nil {
					respondWithJSON(event.ToJSON(), c)
					return
				}

				c.JSON(http.StatusNotFound, gin.H{
					"msg": "Not found",
				})
				return

			}

			c.JSON(http.StatusBadRequest, gin.H{
				"msg": "Bad query param(s)",
			})

		})

	}

	router.GET("/v1/ws", func(c *gin.Context) {

		// Setting read & write buffer size
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {

			log.Printf("[!] Failed to upgrade to websocket : %s\n", err.Error())
			return

		}

		// Registering websocket connection closing, to be executed when leaving
		// this function block
		defer conn.Close()

		// To be used for concurrent safe access of
		// underlying network socket
		connLock := sync.Mutex{}
		// To be used for concurrent safe access of subscribed
		// topic's associative array
		topicLock := sync.RWMutex{}

		// Log it when closing connection
		defer func() {

			log.Printf("[] Closing websocket connection\n",)

		}()

		// All topic subscription/ unsubscription requests
		// to handled by this higher layer abstraction
		pubsubManager := ps.SubscriptionManager{
			Topics:     make(map[string]map[string]*ps.SubscriptionRequest),
			Consumers:  make(map[string]ps.Consumer),
			Client:     _redisClient,
			Connection: conn,
			DB:         _db,
			ConnLock:   &connLock,
			TopicLock:  &topicLock,
		}

		// Unsubscribe from all pubsub topics ( 3 at max ) when returning from
		// this execution scope
		defer func() {

			topicLock.Lock()
			defer topicLock.Unlock()

			for _, v := range pubsubManager.Consumers {
				v.Unsubscribe()
			}

		}()

		// Client communication handling logic
		for {

			var req ps.SubscriptionRequest

			if err := conn.ReadJSON(&req); err != nil {

				log.Printf("[!] Failed to read message : %s\n", err.Error())
				break

			}

			// Validating incoming request on websocket subscription channel
			if !req.Validate(&pubsubManager) {
				// -- Critical section of code begins
				//
				// Attempting to write to shared network connection
				connLock.Lock()

				if err := conn.WriteJSON(&ps.SubscriptionResponse{Code: 0, Message: "Bad Payload"}); err != nil {
					log.Printf("[!] Failed to write message : %s\n", err.Error())
				}

				connLock.Unlock()
				// -- ends here
				break
			}

			// Attempting to subscribe to/ unsubscribe from this topic
			switch req.Type {
			case "subscribe":
				pubsubManager.Subscribe(&req)
			case "unsubscribe":
				pubsubManager.Unsubscribe(&req)
			}

		}

	})

	router.POST("/v1/graphql",
		// Attempting to pass router context, which so that some job can
		// be done if needed to (i.e. logging, stats, etc.) before delivering requested piece of data to client
		func(c *gin.Context) {
			ctx := context.WithValue(c.Request.Context(), "RouterContextInGraphQL", c)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		},

		func(c *gin.Context) {

			gql := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
				Resolvers: &graph.Resolver{},
			}))

			if gql == nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"msg": "Failed to handle graphQL query",
				})
				return
			}

			gql.ServeHTTP(c.Writer, c.Request)

		})

	router.GET("/v1/graphql-playground", func(c *gin.Context) {

		gpg := playground.Handler("validationcloud", "/v1/graphql")

		if gpg == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"msg": "Failed to create graphQL playground",
			})
			return
		}

		gpg.ServeHTTP(c.Writer, c.Request)

	})

	router.Run(fmt.Sprintf(":%s", cfg.Get("PORT")))
}
