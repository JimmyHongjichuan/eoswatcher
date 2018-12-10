package eoswatcher

import (
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"time"
)

type EOSWatcherInterface interface {
	UpdatePubKeyHash(pubKeyHash string)

	StartWatch(scanBlockHeight, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent, channelCount int)

	UpdateInfo()
	UpdateBlock(scanBlockHeight uint32)		*eos.BlockResp
	UpdateEOSPushEvent(scanBlockResp *eos.BlockResp, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent)

	GetEventByTxid(txid string) (*EOSPushEvent, error)

	PKMSign(tx *eos.SignedTransaction) (sig *ecc.Signature, err error)
	GetPublickeyFromTx(tx *eos.SignedTransaction, sig *ecc.Signature) (out ecc.PublicKey, err error)
	MergeSignedTx(tx *eos.SignedTransaction, sigs ...*ecc.Signature) (packedTx *eos.PackedTransaction, err error)
	SendTx(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error)

	CreateTx(action *eos.Action, duration time.Duration) (*eos.SignedTransaction, error)
	CreateActionsTx(action []*eos.Action, duration time.Duration) (*eos.SignedTransaction, error)

	NewPublicKey(uncompresspubkey string) (*ecc.PublicKey, error)
}

