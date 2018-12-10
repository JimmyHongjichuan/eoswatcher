package utils

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"

	"github.com/eoscanada/eos-go/btcsuite/btcd/btcec"
	"github.com/eoscanada/eos-go/ecc"
	log "github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

//从签名服务中获取04开头的UnCompressPubkey, 转换成ecc.Pubkey，用于EOS区块链账户系统
func NewPublicKey(uncompresspubkey string) (*ecc.PublicKey, error) {
	if len(uncompresspubkey) != 130 {
		log.Error("uncompresspubkey must be 130 length")
		return nil, errors.New("uncompresspubkey must be 130 length")
	}
	X, _ := hex.DecodeString(uncompresspubkey[2:66])

	Y, _ := hex.DecodeString(uncompresspubkey[66:])
	bigX := new(big.Int).SetBytes(X)
	bigY := new(big.Int).SetBytes(Y)
	ecdsaPubkey := ecdsa.PublicKey{
		Curve: btcec.S256(),
		X:     bigX,
		Y:     bigY,
	}
	pubKey := btcec.PublicKey(ecdsaPubkey)
	newPublicKey := &ecc.PublicKey{
		Curve:   ecc.CurveK1,
		Content: pubKey.SerializeCompressed(),
	}
	return newPublicKey, nil

}
