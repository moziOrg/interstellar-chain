package server

import (
	"context"
	"errors"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	servertypes "github.com/cosmos/evm/server/types"
	"github.com/stretchr/testify/require"
)

type retryClient struct {
	rpcclient.Client
	events chan coretypes.ResultEvent
	fail   string
	failed chan struct{}
}

func (c *retryClient) Status(context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{}, nil
}
func (c *retryClient) Subscribe(context.Context, string, string, ...int) (<-chan coretypes.ResultEvent, error) {
	return c.events, nil
}
func (c *retryClient) Block(_ context.Context, height *int64) (*coretypes.ResultBlock, error) {
	if c.fail == "block" {
		c.fail = ""
		close(c.failed)
		return nil, errors.New("temporary block error")
	}
	return &coretypes.ResultBlock{Block: &cmttypes.Block{Header: cmttypes.Header{Height: *height}}}, nil
}
func (c *retryClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	if c.fail == "results" {
		c.fail = ""
		close(c.failed)
		return nil, errors.New("temporary results error")
	}
	return &coretypes.ResultBlockResults{Height: *height}, nil
}

type retryIndexer struct {
	servertypes.EVMTxIndexer
	fail    bool
	failed  chan struct{}
	indexed chan int64
}

func (i *retryIndexer) LastIndexedBlock() (int64, error) { return 0, nil }
func (i *retryIndexer) IndexBlock(block *cmttypes.Block, _ []*abci.ExecTxResult) error {
	if i.fail {
		i.fail = false
		close(i.failed)
		return errors.New("temporary write error")
	}
	i.indexed <- block.Height
	return nil
}

func TestIndexerRetriesFailedHeightBeforeAdvancing(t *testing.T) {
	for _, stage := range []string{"block", "results", "write"} {
		t.Run(stage, func(t *testing.T) {
			failed := make(chan struct{})
			client := &retryClient{events: make(chan coretypes.ResultEvent, 4), fail: stage, failed: failed}
			idx := &retryIndexer{fail: stage == "write", failed: failed, indexed: make(chan int64, 4)}
			service := NewEVMIndexerService(idx, client)
			done := make(chan error, 1)
			go func() { done <- service.Start() }()
			t.Cleanup(func() { _ = service.Stop() })
			send := func(h int64) {
				client.events <- coretypes.ResultEvent{Data: cmttypes.EventDataNewBlockHeader{Header: cmttypes.Header{Height: h}}}
			}
			send(1)
			select {
			case <-failed:
			case <-time.After(5 * time.Second):
				t.Fatal("failure path not reached")
			}
			send(2)
			for _, want := range []int64{1, 2} {
				select {
				case got := <-idx.indexed:
					require.Equal(t, want, got)
				case <-time.After(5 * time.Second):
					t.Fatal("indexer did not recover")
				}
			}
			require.NoError(t, service.Stop())
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("indexer did not stop")
			}
		})
	}
}
