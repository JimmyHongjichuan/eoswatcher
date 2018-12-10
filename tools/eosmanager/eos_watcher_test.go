package eosmanager

import (
	"eosc/tools/utils"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newWatcher() *EosWatcher {
	redispool := utils.NewPool()
	client := NewEosWatcher("http://47.97.167.221:8888", 0, redispool)
	return client
}

func TestUpdateScanHeightToRedis(t *testing.T) {
	client := newWatcher()
	result := client.UpdateScanHeightToRedis(998)
	assert.True(t, result)

}

func TestGetScanHeightFromRedis(t *testing.T) {
	client := newWatcher()
	result := client.GetScanHeightFromRedis()
	assert.Equal(t, uint32(998), result)
}
