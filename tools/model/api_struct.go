package model

import (
	"encoding/json"
	"fmt"
	"time"
)

type ChainInfo struct {
	ServerVersion            string `json:"server_version"`
	HeadBlockNum             int    `json:"head_block_num"`
	LastIrreversibleBlockNum int    `json:"last_irreversible_block_num"`
	HeadBlockID              string `json:"head_block_id"`
	HeadBlockTime            string `json:"head_block_time"`
	HeadBlockProducer        string `json:"head_block_producer"`
}

//BlockResp
type BlockResp struct {
	BlockNum       uint32        `json:"block_num"`
	BlockID        string        `json:"id"`
	BlockTimeStamp JSONTime      `json:"timestamp"`
	Producer       string        `json:"producer"`
	Transactions   []Transaction `json:"transactions"`
}

type Transaction struct {
	Status string          `json:"status"`
	RawTrx json.RawMessage `json:"trx"`
	Trx
}

type Trx struct {
	ID              string `json:"ID"`
	TransactionInfo `json:"transaction"`
}

type TransactionInfo struct {
	Expiration     JSONTime `json:"expiration"`
	RefBlockNum    uint16   `json:"ref_block_num"`
	RefBlockPrefix uint32   `json:"ref_block_prefix"`
	DelaySec       uint32   `json:"delay_sec"`
	Actions        []Action `json:"actions"`
}

type Action struct {
	Account string      `json:"account"`
	Name    string      `json:"name"`
	Data    interface{} `json:"data"`
}

// JSONTime

type JSONTime struct {
	time.Time
}

const JSONTimeFormat = "2006-01-02T15:04:05"

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.Format(JSONTimeFormat))), nil
}

func (t *JSONTime) UnmarshalJSON(data []byte) (err error) {
	if string(data) == "null" {
		return nil
	}

	t.Time, err = time.Parse(`"`+JSONTimeFormat+`"`, string(data))
	return err
}
