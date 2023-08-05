package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	blk "github.com/denniswon/validationcloud/app/block"
	"github.com/gookit/color"

	"github.com/denniswon/validationcloud/app/rest"
)

// Run - Application to be invoked from main runner using this function
func Run(configFile string) {

	ctx, cancel := context.WithCancel(context.Background())
	_connection, _redisClient, _redisInfo, _db, _status, _queue := bootstrap(configFile)

	// Attempting to listen to Ctrl+C signal
	// and when received gracefully shutting down the service
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGTERM, syscall.SIGINT)

	// All resources being used gets cleaned up
	// when we're returning from this function scope
	go func() {

		<-interruptChan

		// This call should be received in all places
		// where root context is passed along
		//
		// But only it's being used in block processor queue
		// go routine, as of now
		//
		// @note This can ( needs to ) be improved
		cancel()

		sql, err := _db.DB()
		if err != nil {
			log.Print(color.Red.Sprintf("[!] Failed to get underlying DB connection : %s", err.Error()))
			return
		}

		if err := sql.Close(); err != nil {
			log.Print(color.Red.Sprintf("[!] Failed to close underlying DB connection : %s", err.Error()))
			return
		}

		if err := _redisInfo.Client.Close(); err != nil {
			log.Print(color.Red.Sprintf("[!] Failed to close connection to Redis : %s", err.Error()))
			return
		}

		// Stopping process
		log.Print(color.Magenta.Sprintf("\n[+] Gracefully shut down the service"))
		os.Exit(0)

	}()

	go _queue.Start(ctx)

	// Pushing block header propagation listener to another thread of execution
	go blk.SubscribeToNewBlocks(_connection, _db, _status, _redisInfo, _queue)

	// Starting http server on main thread
	rest.RunHTTPServer(_db, _status, _redisClient)

}
