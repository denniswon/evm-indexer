package queue

import (
	"context"
	"math"
	"time"
)

// You don’t have unlimited resource on your machine, the minimal size of a goroutine object is 2 KB,
// when you spawn too many goroutine, your machine will quickly run out of memory and the CPU will keep processing
// the task until it reach the limit. By using limited pool of workers and keep the task on the queue,
// we can reduce the burst of CPU and memory since the task will wait on the queue until the the worker pull the task.

// Block - Keeps track of single block i.e. how many
// times attempted till date, last attempted to process
// is block processing currently
type Block struct {
	Done       			bool
	LastAttempted       time.Time
	Delay               time.Duration
}

// SetDelay - Set delay at next fibonacci number in series, interpreted as seconds
// NOTE: The ratio of two adjacent numbers in the Fibonacci series rapidly approaches ((1 + sqrt(5)) / 2).
// So if N is multiplied by ((1 + sqrt(5)) / 2) and round it, the resultant number will be the next fibonacci number.
func (b *Block) SetDelay() {
	b.Delay = time.Duration(int64(math.Round(b.Delay.Seconds()*(1.0+math.Sqrt(5.0))/2))%3600) * time.Second
}

// ResetDelay - Reset delay back to 1 second
func (b *Block) ResetDelay() {
	b.Delay = time.Duration(1) * time.Second
}

// SetLastAttempted - Updates last attempted to process block
// to current UTC time
func (b *Block) SetLastAttempted() {
	b.LastAttempted = time.Now().UTC()
}

// CanAttempt - Can we attempt to process this block ?
//
// Yes, if waiting phase has elapsed
func (b *Block) CanAttempt() bool {
	return time.Now().UTC().After(b.LastAttempted.Add(b.Delay))
}

// Request - Any request to be placed into queue's channels in this form
type Request struct {
	BlockNumber  uint64
	ResponseChan chan bool
}

// Next - Block to be processed next, asked by sending this request
// when receptor detects so, will attempt to find out what should be next processed and
// send that block number is response over channel specified by client
type Next struct {
	ResponseChan chan struct {
		Status bool
		Number uint64
	}
}

// Stat - Clients can query how many blocks present in queue currently
type Stat struct {
	ResponseChan chan StatResponse
}

// StatResponse - Statistics of queue to be responded back to client in this form
type StatResponse struct {
	Waiting    			uint64
	Total               uint64
}

// BlockProcessorQueue - concurrent safe queue to be interacted with before attempting to process any block
type BlockProcessorQueue struct {
	Blocks                map[uint64]*Block
	StartedWith           uint64
	TotalInserted         uint64
	Total                 uint64
	PutChan               chan Request
	InsertedChan          chan Request
	FailedChan   		  chan Request
	DoneChan     		  chan Request
	StatChan              chan Stat
	NextChan     		  chan Next
}

// New - Getting new instance of queue, to be invoked during setting up application
func New(startingWith uint64) *BlockProcessorQueue {

	return &BlockProcessorQueue{
		Blocks:                make(map[uint64]*Block),
		StartedWith:           startingWith,
		TotalInserted:         0,
		Total:                 0,
		PutChan:               make(chan Request, 128),
		InsertedChan:          make(chan Request, 128),
		FailedChan:   		   make(chan Request, 128),
		DoneChan:     		   make(chan Request, 128),
		StatChan:              make(chan Stat, 1),
		NextChan:    		   make(chan Next, 1),
	}

}

// Put - Client is supposed to be invoking this method
// when it's interested in putting new block to processing queue
//
// If responded with `true`, they're good to go with execution of
// processing of this block
//
// If this block is already put into queue, it'll ask client
// to not proceed with this number
func (b *BlockProcessorQueue) Put(block uint64) bool {

	resp := make(chan bool)
	req := Request{
		BlockNumber:  block,
		ResponseChan: resp,
	}

	b.PutChan <- req
	return <-resp

}

// Inserted - Marking this block has been inserted into DB (not updation, it's insertion)
func (b *BlockProcessorQueue) Inserted(block uint64) bool {

	resp := make(chan bool)
	req := Request{
		BlockNumber:  block,
		ResponseChan: resp,
	}

	b.InsertedChan <- req
	return <-resp

}

// Failed - block processing failed
func (b *BlockProcessorQueue) Failed(block uint64) bool {

	resp := make(chan bool)
	req := Request{
		BlockNumber:  block,
		ResponseChan: resp,
	}

	b.FailedChan <- req
	return <-resp

}

// Done - block processed successfully
func (b *BlockProcessorQueue) Done(block uint64) bool {

	resp := make(chan bool)
	req := Request{
		BlockNumber:  block,
		ResponseChan: resp,
	}

	b.DoneChan <- req
	return <-resp

}

// Stat - Client's are supposed to be invoking this abstracted method
// for checking queue status
func (b *BlockProcessorQueue) Stat() StatResponse {

	resp := make(chan StatResponse)
	req := Stat{ResponseChan: resp}

	b.StatChan <- req
	return <-resp

}

// Next - Next block that can be processed
func (b *BlockProcessorQueue) Next() (uint64, bool) {

	resp := make(chan struct {
		Status bool
		Number uint64
	})
	req := Next{ResponseChan: resp}

	b.NextChan <- req

	v := <-resp
	return v.Number, v.Status

}

// Start - You're supposed to be starting this method as an
// independent go routine, with will listen on multiple channels
// & respond back over provided channel ( by client )
func (b *BlockProcessorQueue) Start(ctx context.Context) {

	for {
		select {

		case <-ctx.Done():
			return

		case req := <-b.PutChan:

			// Once a block is inserted into processing queue, don't overwrite its history with some new request
			if _, ok := b.Blocks[req.BlockNumber]; ok {

				req.ResponseChan <- false
				break

			}

			b.Blocks[req.BlockNumber] = &Block{
				LastAttempted:       time.Now().UTC(),
				Delay:               time.Duration(1) * time.Second,
			}
			req.ResponseChan <- true

		case req := <-b.InsertedChan:
			// Increments how many blocks were inserted into DB

			if _, ok := b.Blocks[req.BlockNumber]; !ok {
				req.ResponseChan <- false
				break
			}

			b.TotalInserted++
			req.ResponseChan <- true

		case req := <-b.FailedChan:

			block, ok := b.Blocks[req.BlockNumber]
			if !ok {
				req.ResponseChan <- false
				break
			}

			block.SetDelay()

			req.ResponseChan <- true

		case req := <-b.DoneChan:

			block, ok := b.Blocks[req.BlockNumber]
			if !ok {
				req.ResponseChan <- false
				break
			}

			block.Done = true

			req.ResponseChan <- true

		case nxt := <-b.NextChan:

			var selected uint64
			var found bool

			for k := range b.Blocks {

				if b.Blocks[k].Done {
					continue
				}

				if b.Blocks[k].CanAttempt() {
					selected = k
					found = true

					break
				}

			}

			if !found {

				nxt.ResponseChan <- struct {
					Status bool
					Number uint64
				}{
					Status: false,
				}
				break

			}

			b.Blocks[selected].SetLastAttempted()

			nxt.ResponseChan <- struct {
				Status bool
				Number uint64
			}{
				Status: true,
				Number: selected,
			}

		case req := <-b.StatChan:

			// Returning back how many blocks currently living
			// in block processor queue & in what state
			var stat StatResponse

			for k := range b.Blocks {

				if !b.Blocks[k].Done {
					stat.Waiting++
					continue
				}

			}

			stat.Total = b.Total
			req.ResponseChan <- stat

		case <-time.After(time.Duration(100) * time.Millisecond):

			// Finding out which blocks are good to clean those up
			for k := range b.Blocks {

				if b.Blocks[k].Done {
					delete(b.Blocks, k)
					b.Total++ // Successfully processed #-of blocks
				}

			}

		}
	}

}
