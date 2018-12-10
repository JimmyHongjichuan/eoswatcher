package eoswatcher

import (
	"encoding/hex"
	"encoding/json"
	"github.com/eoscanada/eos-go"
)

type PushEvent interface {
	GetBusiness()		string
	GetEventType()		uint32
	GetProposal()		string
	GetTxID()			string
	GetHeight()			int64
	GetTranxIx()		int
	GetAmount()			uint64
	GetFee()			uint64
	GetFrom()			byte
	GetTo()				byte
	GetData()			[]byte
}

type EOSPushEvent struct {
	TxID				eos.SHA256Bytes

	Account				eos.AccountName
	Name				eos.ActionName
	Memo				string
	Amount				uint64
	Symbol				string
	Precision			uint8

	BlockNum			uint32
	Index				int
}

type JsonMemo struct {
	Address	string
	Chain	string
}

func (event *EOSPushEvent) GetBusiness() string {
	return ""
}

func (event *EOSPushEvent) GetEventType() uint32 {
	return 0
}

func (event *EOSPushEvent) GetProposal() string {
	return ""
}

func (event *EOSPushEvent) GetTxID() string {
	return hex.EncodeToString(event.TxID)
}

func (event *EOSPushEvent) GetHeight() int64 {
	return int64(event.BlockNum)
}

func (event *EOSPushEvent) GetTranxIx() int {
	return event.Index
}

func (event *EOSPushEvent) GetAmount() uint64 {
	//return uint64(event.Transfer.Quantity.Amount)
	return event.Amount
}

func (event *EOSPushEvent) GetFee() uint64 {
	return 0
}

func (event *EOSPushEvent) GetFrom() byte {
	return 0
}

func (event *EOSPushEvent) GetTo() byte {
	return 0
}

func (event *EOSPushEvent) GetData() ([]byte, error) {
	if (event.Symbol == "WBCH" || event.Symbol == "WBTC") {
		var tmpMemo JsonMemo
		err := json.Unmarshal([]byte(event.Memo), &tmpMemo)
		if err != nil {
			return []byte{}, err
		}

		if event.Symbol == "WBCH" {
			tmpMemo.Chain = "bch"
			byteMemo, err := json.Marshal(tmpMemo)
			if err != nil {
				return []byte{}, err
			}
			return byteMemo, nil
		}
		if event.Symbol == "WBTC" {
			tmpMemo.Chain = "btc"
			byteMemo, err := json.Marshal(tmpMemo)
			if err != nil {
				return []byte{}, err
			}
			return byteMemo, nil
		}
	}
	return []byte(event.Memo), nil
	//return []byte{}
}

func (event *EOSPushEvent) GetSymbol() string {
	return event.Symbol
}

func (event *EOSPushEvent) GetPrecision() uint8 {
	return event.Precision
}
