package server

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cometbft/cometbft/libs/service"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"

	servertypes "github.com/cosmos/evm/server/types"
)

const (
	ServiceName = "EVMIndexerService"

	NewBlockWaitTimeout = 60 * time.Second
)

// EVMIndexerService indexes transactions for json-rpc service.
type EVMIndexerService struct {
	service.BaseService

	txIdxr servertypes.EVMTxIndexer
	client rpcclient.Client
}

// NewEVMIndexerService returns a new service instance.
func NewEVMIndexerService(
	txIdxr servertypes.EVMTxIndexer,
	client rpcclient.Client,
) *EVMIndexerService {
	is := &EVMIndexerService{txIdxr: txIdxr, client: client}
	is.BaseService = *service.NewBaseService(nil, ServiceName, is)
	return is
}

// OnStart implements service.Service by subscribing for new blocks
// and indexing them by events.
func (eis *EVMIndexerService) OnStart() error {
	ctx := context.Background()
	status, err := eis.client.Status(ctx)
	if err != nil {
		return err
	}
	var latestBlock atomic.Int64
	latestBlock.Store(status.SyncInfo.LatestBlockHeight)
	newBlockSignal := make(chan struct{}, 1)

	// Use SubscribeUnbuffered here to ensure both subscriptions does not get
	// canceled due to not pulling messages fast enough. Cause this might
	// sometimes happen when there are no other subscribers.
	blockHeadersChan, err := eis.client.Subscribe(
		ctx,
		ServiceName,
		types.QueryForEvent(types.EventNewBlockHeader).String(),
		0)
	if err != nil {
		return err
	}

	go func() {
		for {
			var msg coretypes.ResultEvent
			select {
			case <-eis.Quit():
				return
			case event, ok := <-blockHeadersChan:
				if !ok {
					return
				}
				msg = event
			}
			eventDataHeader, ok := msg.Data.(types.EventDataNewBlockHeader)
			if !ok {
				continue
			}
			if eventDataHeader.Header.Height > latestBlock.Load() {
				latestBlock.Store(eventDataHeader.Header.Height)
				// notify
				select {
				case newBlockSignal <- struct{}{}:
				default:
				}
			}
		}
	}()

	lastBlock, err := eis.txIdxr.LastIndexedBlock()
	if err != nil {
		return err
	}
	if lastBlock == -1 {
		lastBlock = latestBlock.Load()
	}

	// blockErr indicates a failed fetch or index write. Never skip that height.
	var blockErr error

	for {
		if latestBlock.Load() <= lastBlock || blockErr != nil {
			// two cases to enter this block:
			// 1. nothing to index (indexer is caught up). wait for signal of new block.
			// 2. previous attempt to index errored (failed to fetch the Block or BlockResults).
			//    in this case, wait before retrying the data fetching, rather than infinite looping
			//    a failing fetch. this can occur due to drive latency between the block existing and its
			//    block_results getting saved.
			select {
			case <-eis.Quit():
				return nil
			case <-newBlockSignal:
			case <-time.After(NewBlockWaitTimeout):
			}
			blockErr = nil
		}
		for i := lastBlock + 1; i <= latestBlock.Load(); i++ {
			select {
			case <-eis.Quit():
				return nil
			default:
			}
			var (
				block       *coretypes.ResultBlock
				blockResult *coretypes.ResultBlockResults
			)

			block, blockErr = eis.client.Block(ctx, &i)
			if blockErr != nil {
				eis.Logger.Error("failed to fetch block", "height", i, "err", blockErr)
				break
			}
			blockResult, blockErr = eis.client.BlockResults(ctx, &i)
			if blockErr != nil {
				eis.Logger.Error("failed to fetch block result", "height", i, "err", blockErr)
				break
			}
			if blockErr = eis.txIdxr.IndexBlock(block.Block, blockResult.TxsResults); blockErr != nil {
				eis.Logger.Error("failed to index block", "height", i, "err", blockErr)
				break
			}
			lastBlock = blockResult.Height
		}
	}
}
