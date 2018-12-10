package eosmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newClient() *EosClient {
	//jungleTestnet
	client := NewEosClient("http://47.98.185.203:8888")
	client.SetChainID()
	return client
}

func TestGetBlockByID(t *testing.T) {
	client := newClient()
	out, err := client.GetBlockByID(155)
	assert.NoError(t, err)
	assert.NotNil(t, out.BlockNum)
}

func TestGetTransaction(t *testing.T) {
	client := newClient()
	//jungletestnet上面对应于zxj的一笔交易
	out, err := client.GetTransaction("8969540a95ca8637a2931c945608befdebe1c0b68f5e852f2f4931c5cc1ea726")
	assert.NoError(t, err)
	assert.NotNil(t, out)
}

func TestGetInfo(t *testing.T) {
	client := newClient()
	out, err := client.GetInfo()
	assert.NoError(t, err)
	assert.NotNil(t, out.LastIrreversibleBlockNum)
}

func TestGetCurrencyBalance(t *testing.T) {
	client := newClient()
	out, err := client.GetCurrencyBalance("zxj", "")
	assert.NoError(t, err)
	assert.NotNil(t, out)
}
