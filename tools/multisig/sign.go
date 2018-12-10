package multisig

import (
	"crypto/ecdsa"
	"encoding/hex"
	"eosc/tools/utils"
	"errors"
	"fmt"
	"math/big"

	"github.com/spf13/viper"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/btcsuite/btcd/btcec"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/token"
	log "github.com/inconshreveable/log15"
)

func BuildTokenTransferTx(from, to, memo string, quantity int64) (signedTx *eos.SignedTransaction, err error) {
	fromName := eos.AN(from)
	toName := eos.AN(to)
	asset := NewEOSAsset(quantity)

	action := token.NewTransfer(fromName, toName, asset, memo)
	tx := &eos.Transaction{Actions: []*eos.Action{action}}

	api := eos.New(viper.GetString("EOS.base_url"))
	blockInfo, err := api.GetInfo()
	if err != nil {
		log.Error("GetInfo err:", err)
		return nil, err
	}
	tx.Fill(blockInfo.HeadBlockID, 0, 0, 0)

	stx := eos.NewSignedTransaction(tx)

	//stx.estimateResources(*opts, 100, 10000)
	return stx, nil
}

func NewEOSAsset(in int64) eos.Asset {
	EOSSymbol := eos.Symbol{Precision: 4, Symbol: "EOS"}
	return eos.Asset{Amount: in, Symbol: EOSSymbol}
}

func LocalSign(tx *eos.SignedTransaction, privkey string) (sig *ecc.Signature, err error) {
	txdata, err := eos.MarshalBinary(tx.Transaction)
	if err != nil {
		return nil, err
	}

	cfd := []byte{}
	if len(tx.ContextFreeData) > 0 {
		cfd, err = eos.MarshalBinary(tx.ContextFreeData)
		if err != nil {
			return nil, err
		}
	}
	api := eos.New(viper.GetString("EOS.base_url"))
	blockInfo, err := api.GetInfo()
	if err != nil {
		log.Error("get blockInfo err", err)
		return nil, err
	}
	chainID := blockInfo.ChainID

	sigDigest := eos.SigDigest(chainID, txdata, cfd)

	/*通过调用签名服务来进行签名
	sig, err := utils.Sign(sigDigest, mw.pubKeyHash)
	if err != nil {
		log.Error("SIGN", "error:", err)
		return sig, err
	}
	*/

	//直接获取private进行签名,测试用
	privateKey, err := ecc.NewPrivateKey(privkey)
	if err != nil {
		log.Error("NewPrivateKey err", err)
		return nil, err
	}
	sigResult, err := privateKey.Sign(sigDigest)
	if err != nil {
		return nil, err
	}
	fmt.Println("localsig---->", sigResult.Content)

	return &sigResult, nil
}

func PKMSign(tx *eos.SignedTransaction, pubKeyHash string) (sig *ecc.Signature, err error) {
	txdata, err := eos.MarshalBinary(tx.Transaction)
	if err != nil {
		return nil, err
	}

	cfd := []byte{}
	if len(tx.ContextFreeData) > 0 {
		cfd, err = eos.MarshalBinary(tx.ContextFreeData)
		if err != nil {
			return nil, err
		}
	}
	api := eos.New(viper.GetString("EOS.base_url"))
	blockInfo, err := api.GetInfo()
	if err != nil {
		log.Error("get blockInfo err", err)
		return nil, err
	}
	chainID := blockInfo.ChainID

	sigDigest := eos.SigDigest(chainID, txdata, cfd)

	//通过调用签名服务来进行签名
	sigResult, err := utils.Sign(sigDigest, pubKeyHash)
	if err != nil {
		log.Error("SIGN", "error:", err)
		return sig, err
	}
	return &sigResult, nil
}

func MergeSignedTx(tx *eos.SignedTransaction, sigs ...*ecc.Signature) (packedTx *eos.PackedTransaction, err error) {
	//合并签名
	for _, sig := range sigs {
		tx.Signatures = append(tx.Signatures, *sig)
	}
	//交易打包
	//log.Debug("for pack", "data", tx)
	trx, err := tx.Pack(0)
	if err != nil {
		return nil, err
	}
	return trx, nil
}

func SendTx(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error) {
	api := eos.New(viper.GetString("EOS.base_url"))
	//log.Debug("sengTx------>", "tx", tx)
	out, err = api.PushTransaction(tx)
	if err != nil {
		fmt.Println("send tx err:", err)
		return nil, err
	}
	return
}


//根据交易 和签名，推出对应的公钥
func GetPublickeyFromTx(tx *eos.SignedTransaction, sig *ecc.Signature) (out ecc.PublicKey, err error) {
	txdata, err := eos.MarshalBinary(tx.Transaction)
	if err != nil {
		return ecc.PublicKey{}, err
	}

	cfd := []byte{}
	if len(tx.ContextFreeData) > 0 {
		cfd, err = eos.MarshalBinary(tx.ContextFreeData)
		if err != nil {
			return ecc.PublicKey{}, err
		}
	}
	api := eos.New(viper.GetString("EOS.base_url"))
	blockInfo, err := api.GetInfo()
	if err != nil {
		log.Error("get blockInfo err", err)
		return ecc.PublicKey{}, err
	}
	chainID := blockInfo.ChainID

	sigDigest := eos.SigDigest(chainID, txdata, cfd)

	return GetPublickey(sigDigest, sig)
}

//根据签名数据 和签名，推算出公钥
func GetPublickey(hash []byte, sig *ecc.Signature) (out ecc.PublicKey, err error) {
	return sig.PublicKey(hash)
}

func VerifySign(payload []byte, sig ecc.Signature, pubkey ecc.PublicKey) bool {
	result := sig.Verify(payload, pubkey)
	return result
}

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
