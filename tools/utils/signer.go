package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"eosc/tools/model"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/eoscanada/eos-go/btcsuite/btcd/btcec"
	"github.com/spf13/viper"

	//"github.com/btcsuite/btcd/btcec"
	"github.com/eoscanada/eos-go/ecc"
	log "github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

// Calculate the hash of hasher over buf.
func calcHash(buf []byte, hasher hash.Hash) []byte {
	hasher.Write(buf)
	return hasher.Sum(nil)
}

//此处要返回一个ecc.Signature
func Sign(sigDigest []byte, pubkeyHash string) (ecc.Signature, error) {

	sigString := hex.EncodeToString(sigDigest)
	inputData := &model.SignInput{
		InputHex:   sigString,
		PubHashHex: pubkeyHash,
		TimeStamp:  time.Now().Unix(),
		ServiceID:  viper.GetString("KEYSTORE.service_id"),
		Type:       2, //EOS签名固定传2
	}
	postData, err := json.Marshal(inputData)
	if err != nil {
		log.Error("SIGN", "error", err)
		return ecc.Signature{}, err
	}

	body, err := callAPI("/key/sign", postData)
	if err != nil {
		log.Error("signData failed", "err", err.Error())
		return ecc.Signature{}, err
	}

	var sigRes model.SignResponse
	err1 := json.Unmarshal(body, &sigRes)
	if err1 != nil {
		log.Error("signData failed", "err", err.Error())
		return ecc.Signature{}, err
	}
	if sigRes.Code == 0 {
		compactSig, err := hex.DecodeString(sigRes.Data.SignatureCompact)
		if err != nil {
			log.Error("SIG DECODE FAILED", "err", err)
			return ecc.Signature{}, errors.New("decode sig fail")
		}
		return ecc.Signature{Curve: ecc.CurveK1, Content: compactSig}, nil
	}
	return ecc.Signature{}, errors.Errorf("no sign result")
}

func callAPI(api string, postData []byte) ([]byte, error) {
	keyStorePK := viper.GetString("KEYSTORE.keystore_private_key")

	keypkByte, err := hex.DecodeString(keyStorePK)
	if err != nil {
		log.Error("postData failed", "err", err.Error())
		return nil, err
	}

	keyPk, _ := btcec.PrivKeyFromBytes(btcec.S256(), keypkByte)

	postDataHash := calcHash(postData, sha256.New())
	postSign, err := keyPk.Sign(postDataHash)
	if err != nil {
		log.Error("postData failed", "err", err.Error())
		return nil, err
	}

	client := &http.Client{}
	url := strings.Join([]string{viper.GetString("KEYSTORE.url"), api}, "/")
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
	if err != nil {
		log.Error("postData failed", "err:", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("signature", hex.EncodeToString(postSign.Serialize()))
	req.Header.Set("serviceID", viper.GetString("KEYSTORE.service_id"))
	resp, err := client.Do(req)
	if err != nil {
		log.Error("postData failed", "err:", err)
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)

}

//Generate 生成公钥
//count: 需要生成几对公私钥
func Generate(count int) (*model.GenerateData, error) {
	inputData := model.GenerateInput{
		Count:     count,
		ServiceID: viper.GetString("KEYSTORE.service_id"),
		TimeStamp: time.Now().Unix(),
	}

	postData, err := json.Marshal(&inputData)
	if err != nil {
		log.Error("signData failed", "err", err.Error())
		return nil, err
	}

	body, err := callAPI("key/generate", postData)
	if err != nil {
		log.Error("generate failed", "err", err.Error())
		return nil, err
	}

	var res model.GenerateResponse
	err1 := json.Unmarshal(body, &res)
	if err1 != nil {
		log.Error("generate failed", "err", err.Error())
		return nil, err
	}
	if res.Code == 0 {
		return res.Data, nil
	}

	return nil, errors.Errorf("generate result code is %d", res.Code)

}

func VeryfySig(payload []byte, sig ecc.Signature, uncomperssPubkey string) bool {
	bytepubkey, _ := hex.DecodeString(uncomperssPubkey) //这里是04开头的uncompersspubkey
	pubkey := ecc.PublicKey{
		Curve:   ecc.CurveK1,
		Content: bytepubkey,
	}
	result := sig.Verify(payload, pubkey)
	return result
}

/*
func makeCompact(curve *btcec.KoblitzCurve, R, S string, isCompressedKey bool) []byte {
	//byteR, _ := hex.DecodeString(sigData.R)
	byteR, _ := hex.DecodeString(R)
	bigR := new(big.Int)
	bigR.SetBytes(byteR)
	//byteS, _ := hex.DecodeString(sigData.S)
	byteS, _ := hex.DecodeString(S)
	bigS := new(big.Int)
	bigS.SetBytes(byteS)

	//curve.byteSize cant export
	//secp256k1.BitSize = 256
	//secp256k1.byteSize = secp256k1.BitSize / 8
	//result := make([]byte, 1, 2*curve.byteSize+1)
	result := make([]byte, 1, 2*(256/8)+1)
	result[0] = 27
	if isCompressedKey {
		result[0] += 4
	}
	// Not sure this needs rounding but safer to do so.
	curvelen := (curve.BitSize + 7) / 8
	// Pad R and S to curvelen if needed.
	bytelen := (bigR.BitLen() + 7) / 8
	if bytelen < curvelen {
		result = append(result,
			make([]byte, curvelen-bytelen)...)
	}
	result = append(result, bigR.Bytes()...)

	bytelen = (bigS.BitLen() + 7) / 8
	if bytelen < curvelen {
		result = append(result,
			make([]byte, curvelen-bytelen)...)
	}
	result = append(result, bigS.Bytes()...)
	fmt.Println("result--->", result)

	return result
}

func isCanonical(compactSig []byte) bool {
	// From EOS's codebase, our way of doing Canonical sigs.
	// https://steemit.com/steem/@dantheman/steem-and-bitshares-cryptographic-security-update
	//
	// !(c.data[1] & 0x80)
	// && !(c.data[1] == 0 && !(c.data[2] & 0x80))
	// && !(c.data[33] & 0x80)
	// && !(c.data[33] == 0 && !(c.data[34] & 0x80));

	d := compactSig
	t1 := (d[1] & 0x80) == 0
	t2 := !(d[1] == 0 && ((d[2] & 0x80) == 0))
	t3 := (d[33] & 0x80) == 0
	t4 := !(d[33] == 0 && ((d[34] & 0x80) == 0))
	return t1 && t2 && t3 && t4
}
*/
//将EOSxxxxx类型的数据转换成16进制的pubkey字符串
func pubkeyToString(pubKey string) (string, error) {
	newPubKey, err := ecc.NewPublicKey(pubKey)
	if err != nil {
		return "", err
	}
	pubkeystring := hex.EncodeToString(newPubKey.Content)
	return pubkeystring, nil
}

//cant use yet
func wifToPrivKeyHex(wif string) (string, error) {
	newPrivateKey, err := ecc.NewPrivateKey(wif)
	if err != nil {
		return "", nil
	}
	fmt.Println(newPrivateKey)
	//intD := newPrivateKey.
	return "", nil
}

