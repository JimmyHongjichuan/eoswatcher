package eosstore

import (
	"eosc/tools/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newESClient() *ESClient {
	newClient := NewESClient()
	return newClient
}

func TestUpdateTxInfo(t *testing.T) {
	data := &model.TransferData{
		BlockNum:       999,
		BlockID:        "0000006492871283c47f6ef57b00cf534628eb818c34deb87ea68a3557254c6b",
		BlockTimeStamp: time.Now().Unix(),
		Producer:       "zxj",
		TxID:           "a fake tx",
		Status:         "excuted",
		//CpuUsageUs uint32
		//NetUsageWords eos.Varuint32
		Expiration:     1500000000,
		RefBlockNum:    8888,
		RefBlockPrefix: 9999,
		DelaySec:       10,
		From:           "gates",
		To:             "wade",
		Amount:         77777777,
		Symbol:         "EOS",
		Precision:      4,
		Memo:           "rich test",
	}
	result := newESClient().UpdateTxInfo(data, 0)
	assert.True(t, result)
}

func TestDeleteUnconfirmTx(t *testing.T) {
	result := newESClient().DeleteUnconfirmTx("test1231")
	assert.True(t, result)
}
