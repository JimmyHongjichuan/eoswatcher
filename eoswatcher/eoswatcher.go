package eoswatcher

import (
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"eosc/tools/utils"
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/btcsuite/btcd/btcec"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/system"
	"github.com/eoscanada/eos-go/token"
	log "github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	leveldberr "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"math/big"
	"net"
	"net/http"
	"time"
)

type EOSWatcher struct {
	EosAPI						*eos.API
	ScanBlockHeight				uint32

	HeadBlockNum				uint32
	LastIrreversibleBlockNum	uint32
	EOSPushEvents				[]*EOSPushEvent

	// 签名所用私钥 对应的公钥哈希值
	PubKeyHash 					string

	// 扫块时，具体提取哪些操作
	ActionAccount				eos.AccountName
	ActionNameDestroy			eos.ActionName
	ActionNameCreate			eos.ActionName

	DB							*leveldb.DB
}

func NewEosWatcher(url, pubKeyHash, actionAccount, actionNameDestroy, actionNameCreate, dirName string) (*EOSWatcher) {
	//api := eos.New(url)
	api := &eos.API{
		HttpClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 5 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableKeepAlives:     true, // default behavior, because of `nodeos`'s lack of support for Keep alives.
			},
		},
		BaseURL:  url,
		Compress: eos.CompressionZlib,
	}

	eos.RegisterAction(eos.AN(actionAccount), eos.ActN(actionNameDestroy), DestroyToken{})
	eos.RegisterAction(eos.AN(actionAccount), eos.ActN(actionNameCreate), CreateToken{})

	//dirname := viper.GetString("LEVELDB.eos_db_path")
	db, err := leveldb.OpenFile(dirName, &opt.Options{
		OpenFilesCacheCapacity: 16,
		BlockCacheCapacity:     16 / 2 * opt.MiB,
		WriteBuffer:            16 / 4 * opt.MiB,
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*leveldberr.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(dirName, nil)
	}
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}
	//defer db.Close()

	var temp_sacn uint32 = 0
	data, err := db.Get([]byte("ScanBlockHeight"), nil)
	if err != nil {
		log.Error("read eos leveldb ScanBlockHeight err", "info", err)
	} else {
		log.Debug("read eos leveldb ScanBlockHeight", "ScanBlockHeight", binary.LittleEndian.Uint32(data))
		temp_sacn = binary.LittleEndian.Uint32(data)
	}

	ew := &EOSWatcher{
		EosAPI:						api,
		ScanBlockHeight:			temp_sacn,
		//ScanBlockIndex:				0,
		HeadBlockNum:				0,
		LastIrreversibleBlockNum:	0,
		//ScanBlockResp:				nil,
		EOSPushEvents:				nil,
		PubKeyHash:					pubKeyHash,
		ActionAccount:				eos.AN(actionAccount),
		ActionNameDestroy:			eos.ActN(actionNameDestroy),
		ActionNameCreate:			eos.ActN(actionNameCreate),
		DB:							db,
	}
	return ew
}

func (ew *EOSWatcher) UpdatePubKeyHash(pubKeyHash string) {
	ew.PubKeyHash = pubKeyHash
}

func (ew *EOSWatcher) UpdateActionAccount(actionAccount string) {
	ew.ActionAccount = eos.AN(actionAccount)
}

func (ew *EOSWatcher) UpdateActionNameDestroy(actionName string) {
	ew.ActionNameDestroy = eos.ActN(actionName)
}

func (ew *EOSWatcher) UpdateActionNameCreate(actionName string) {
	ew.ActionNameCreate = eos.ActN(actionName)
}

//扫块开始
func (ew *EOSWatcher) StartWatch(scanBlockHeight, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent, channelCount int)  {
	if scanBlockHeight <= 0 {
		scanBlockHeight = 1
	}
	// 扫块代码
	if scanBlockHeight > ew.ScanBlockHeight {
		ew.ScanBlockHeight = scanBlockHeight
	}

	go func() {
		defer ew.DB.Close()
		for {
			// 更新 最新不可逆转块的高度。 返回则表明一定执行成功，不成功则会在该函数中阻塞
			ew.UpdateInfo()
			//log.Debug("-------- Scan Block Height", "info", ew.ScanBlockHeight)

			// 创建50个goroutine，去请求块
			var tmpChannel = make(chan struct{}, channelCount)
			if ew.ScanBlockHeight < ew.LastIrreversibleBlockNum {
				for ; ew.ScanBlockHeight < ew.LastIrreversibleBlockNum;  {
					if ew.ScanBlockHeight % 100 == 0 {
						log.Debug("EOS Scan Block Height", "info", ew.ScanBlockHeight)
					}

					// 协程池中，没用空闲的协程，等待
					for len(tmpChannel) >= channelCount {
						time.Sleep(time.Duration(10) * time.Millisecond)
					}
					//log.Debug("len(tmpChannel)", "length", len(tmpChannel))

					scanHeightx := ew.ScanBlockHeight

					// 将扫块高度，写入leveldb
					bytes := make([]byte, 4)
					binary.LittleEndian.PutUint32(bytes, scanHeightx)
					err := ew.DB.Put([]byte("ScanBlockHeight"), bytes, nil)
					if err != nil {
						log.Error("write eos leveldb ScanBlockHeight err", "info", err)
					}
					go func(scanHeightx uint32) {
						tmpChannel <- struct{}{}

						blockResp := ew.UpdateBlock(scanHeightx)

						ew.UpdateEOSPushEvent(blockResp, scanBlockIndex, eventChan)

						<-tmpChannel
					}(scanHeightx)

					ew.ScanBlockHeight ++
					scanBlockIndex = 0
				}
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

// 更新 最后一个不可逆转块块高、最高块块高。 如果rpc请求报错，那么继续请求，阻塞在这里
func (ew *EOSWatcher) UpdateInfo () {
	for {
		infoResp, err := ew.EosAPI.GetInfo()
		if err != nil {
			log.Error("Get info error!", "EosAPI.BaseURL", ew.EosAPI.BaseURL)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		ew.HeadBlockNum = infoResp.HeadBlockNum
		ew.LastIrreversibleBlockNum = infoResp.LastIrreversibleBlockNum
		return
	}
}

// 更新 要扫描块的信息。 如果rpc请求报错，那么继续请求，阻塞在这里
func (ew *EOSWatcher) UpdateBlock (scanBlockHeight uint32)  *eos.BlockResp {
	for {
		blockResp, err := ew.EosAPI.GetBlockByID(fmt.Sprintf("%d", scanBlockHeight))
		if err != nil {
			log.Debug("Get block error! Wait 100ms to request.",
				"EosAPI.BaseURL", ew.EosAPI.BaseURL,
				"ScanBlockHeight", scanBlockHeight,
				/*err.Error()*/)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return blockResp
	}

}

// 更新 根据获得到的 块收据 信息，生成EOSPush事件，发给网关
func (ew *EOSWatcher) UpdateEOSPushEvent (scanBlockResp *eos.BlockResp, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent) {
	// 首先清空EOSPush事件数组
	ew.EOSPushEvents = []*EOSPushEvent{}

	for index, transactionReceipt := range scanBlockResp.SignedBlock.Transactions{
		if uint32(index) < scanBlockIndex {
			continue
		}
		if transactionReceipt.Transaction.Packed != nil {
			signedTx, err := transactionReceipt.Transaction.Packed.Unpack()
			if err != nil {
				// 如果解压失败，忽略该交易
				log.Error("Unpack error", "TxID", hex.EncodeToString(transactionReceipt.Transaction.ID))
				continue
			}
			// 暂时忽略一笔交易中有多个action 的情况。（待优化）
			if len(signedTx.Transaction.Actions) != 1 {
				continue
			}
			action := signedTx.Transaction.Actions[0]

			// 判断是否是multisigpkm2 合约 的destroytoken操作
			if action.Account != ew.ActionAccount || action.Name != ew.ActionNameDestroy {
				continue
			}

			//dataType := reflect.TypeOf(action.ActionData.Data)
			//fmt.Println(dataType)
			// 解析action 中具体传给合约的参数 data
			destroyToken, ok := action.ActionData.Data.(*DestroyToken)
			if ok == false {
				continue
			}
			//if destroyToken.User != eos.AN("multisigpkm2") && destroyToken.Amount != eos.AN("multisigpkm2") {
			//	continue
			//}

			eosPushEvent := &EOSPushEvent{
				TxID:				transactionReceipt.Transaction.ID,

				Account:			action.Account,
				Name:				action.Name,
				Memo:				destroyToken.Memo,
				Amount:				uint64(destroyToken.Amount),
				Symbol:				"XIN",
				Precision:			0,

				BlockNum:			ew.ScanBlockHeight,
				Index:				index,
			}

			//ew.EOSPushEvents = append(ew.EOSPushEvents, eosPushEvent)
			eventChan <- eosPushEvent

		} //else {
		//	// 暂时忽略特殊交易，比如misg 的exec 产生的交易。（待优化）
		//	log.Error("Ignore transaction without transaction body.", "Block Height", ew.ScanBlockHeight)
		//}
	}

	//更新下一块，从第几笔交易开始扫
	//ew.ScanBlockIndex = 0
}

func (ew *EOSWatcher) GetEventByTxid(txid string) (*EOSPushEvent, error) {
	transactionResp, err := ew.EosAPI.GetTransaction(txid)
	if err != nil {
		return nil, err
	}

	// 暂时忽略一笔交易中有多个action 的情况。（待优化）
	if len(transactionResp.Transaction.Transaction.Actions) != 1 {
		return nil, errors.New("Transaction actions' length is not one.")
	}
	action := transactionResp.Transaction.Transaction.Actions[0]

	if action.Account != ew.ActionAccount {
		return nil, errors.New("Account doesn't match.")
	}

	//dataType := reflect.TypeOf(action.ActionData.Data)
	//fmt.Println(dataType)

	// 处理溶币
	if action.Name == ew.ActionNameDestroy {
		destroy, ok := action.ActionData.Data.(map[string]interface{})
		if ok == false {
			return nil, errors.New("Action Data reflect error.")
		}

		if len(destroy) != 3 {
			return nil, errors.New("Action Data length error.")
		}

		// 解析actionData的具体字段
		user, ok := destroy["user"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field error")
		}
		//fmt.Println(reflect.TypeOf(destroy["amount"]))
		amount, ok := destroy["amount"].(float64)
		if ok == false {
			return nil, errors.New("Action Data 'amount' field error")
		}
		memo, ok := destroy["memo"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'memo' field error")
		}

		destroyToken := &DestroyToken{
			User:		eos.AN(user),
			Amount:		uint32(amount),
			Memo:		memo,
		}

		eosPushEvent := &EOSPushEvent{
			TxID:				transactionResp.ID,

			Account:			action.Account,
			Name:				action.Name,
			Memo:				destroyToken.Memo,
			Amount:				uint64(destroyToken.Amount),
			Symbol:				"XIN",
			Precision:			0,

			BlockNum:			transactionResp.BlockNum,
			Index:				0, // 暂时没有好方法获取index
		}
		return eosPushEvent, nil
	}

	// 处理铸币
	if action.Name == ew.ActionNameCreate {
		create, ok := action.ActionData.Data.(map[string]interface{})
		if ok == false {
			return nil, errors.New("Action Data reflect error.")
		}

		if len(create) != 2 {
			return nil, errors.New("Action Data length error.")
		}

		// 解析actionData的具体字段
		user, ok := create["user"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field error")
		}
		//fmt.Println(reflect.TypeOf(destroy["amount"]))
		amount, ok := create["amount"].(float64)
		if ok == false {
			return nil, errors.New("Action Data 'amount' field error")
		}

		createToken := &CreateToken{
			User:		eos.AN(user),
			Amount:		uint32(amount),
		}

		eosPushEvent := &EOSPushEvent{
			TxID:				transactionResp.ID,

			Account:			action.Account,
			Name:				action.Name,
			Memo:				"",
			Amount:				uint64(createToken.Amount),
			Symbol:				"XIN",
			Precision:			0,

			BlockNum:			transactionResp.BlockNum,
			Index:				0, // 暂时没有好方法获取index
		}
		return eosPushEvent, nil
	}

	return nil, errors.New("Name doesn't match.")
}

// 根据multisig下的PKMSign代码，移植过来
func (ew *EOSWatcher) PKMSign(tx *eos.SignedTransaction) (sig *ecc.Signature, err error) {
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
	blockInfo, err := ew.EosAPI.GetInfo()
	if err != nil {
		log.Error("get blockInfo err", err)
		return nil, err
	}
	chainID := blockInfo.ChainID

	sigDigest := eos.SigDigest(chainID, txdata, cfd)

	//通过调用签名服务来进行签名
	sigResult, err := utils.Sign(sigDigest, ew.PubKeyHash)
	if err != nil {
		log.Error("SIGN", "error:", err)
		return sig, err
	}
	return &sigResult, nil
}

//根据multisig下的GetPublickeyFromTx代码，移植过来
func (ew *EOSWatcher) GetPublickeyFromTx(tx *eos.SignedTransaction, sig *ecc.Signature) (out ecc.PublicKey, err error) {
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
	blockInfo, err := ew.EosAPI.GetInfo()
	if err != nil {
		log.Error("get Info err", err)
		return ecc.PublicKey{}, err
	}
	chainID := blockInfo.ChainID

	sigDigest := eos.SigDigest(chainID, txdata, cfd)

	return sig.PublicKey(sigDigest)
}

//根据multisig下的MergeSignedTx代码，移植过来
func (ew *EOSWatcher) MergeSignedTx(tx *eos.SignedTransaction, sigs ...*ecc.Signature) (packedTx *eos.PackedTransaction, err error) {
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

//根据multisig下的SendTx代码，移植过来
func (ew *EOSWatcher) SendTx(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error) {
	out, err = ew.EosAPI.PushTransaction(tx)
	if err != nil {
		log.Error("send tx err:", err.Error())
		return nil, err
	}
	return
}

// 创建转账action
func (ew *EOSWatcher) CreateTransferAction(account, action, from, to string, amount int64, memo string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: 4, Symbol: "EOS"}}
	//quantity, err := eos.NewAsset(amount)
	//转账代币
	//tokeTransfer := token.NewTransfer(eos.AN(from), eos.AN(to), quantity, memo)
	//这里因为是自定义合约，所以不能利用已有的"token.NewTransfer"方法
	//if err != nil {
	//	return nil, err
	//}
	tokenTransfer := &eos.Action{
		Account: eos.AN(account),
		Name:    eos.ActN(action),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(from), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(token.Transfer{
			From:     eos.AN(from),
			To:       eos.AN(to),
			Quantity: quantity,
			Memo:     memo,
		}),
	}
	return tokenTransfer, nil
}

// 创建部署合约action
func (ew *EOSWatcher) CreateSetCodeAction(accountName, wasmFile, abiFile string) ([]*eos.Action, error) {
	setContract, err := system.NewSetContract(eos.AN(accountName), wasmFile, abiFile)
	if err != nil {
		return nil, err
	}
	return setContract, nil
}

// 创建转账action issue
func (ew *EOSWatcher) XinPlayerIssueAction(account, permissionAccount string, amount uint64) (*eos.Action, error) {
	xinPlayerCreateToken := &eos.Action{
		Account: eos.AN(account),
		Name:    eos.ActN("issue"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(permissionAccount), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(Issue{
			Amount:		amount,
		}),
	}
	return xinPlayerCreateToken, nil
}

// 创建转账action createtoken
func (ew *EOSWatcher) XinPlayerCreateTokenAction(account, permissionAccount, user string, amount uint32) (*eos.Action, error) {
	xinPlayerCreateToken := &eos.Action{
		Account: eos.AN(account),
		Name:    eos.ActN("createtoken"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(permissionAccount), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(CreateToken{
			User:		eos.AN(user),
			Amount:		amount,
		}),
	}
	return xinPlayerCreateToken, nil
}

// 创建转账action destroytoken
func (ew *EOSWatcher) XinPlayerDestroyTokenAction(account, permissionAccount, user, memo string, amount uint32) (*eos.Action, error) {
	xinPlayerDestroyToken := &eos.Action{
		Account: eos.AN(account),
		Name:    eos.ActN("destroytoken"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(permissionAccount), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(DestroyToken{
			User:		eos.AN(user),
			Amount:		amount,
			Memo:		memo,
		}),
	}
	return xinPlayerDestroyToken, nil
}

// 创建转账action newaccount
func (ew *EOSWatcher) XinPlayerNewaccountAction(account, permissionAccount, user string) (*eos.Action, error) {
	xinPlayerNewaccount := &eos.Action{
		Account: eos.AN(account),
		Name:    eos.ActN("newaccount"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(permissionAccount), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(NewAccount{
			User:		eos.AN(user),
		}),
	}
	return xinPlayerNewaccount, nil
}

// 根据action等，创建交易
func (ew *EOSWatcher) CreateTx(action *eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction([]*eos.Action{action}, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

// 根据多个actions等，创建交易
func (ew *EOSWatcher) CreateActionsTx(action []*eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction(action, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

func (ew *EOSWatcher) NewPublicKey(uncompresspubkey string) (*ecc.PublicKey, error) {
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

type Issue struct {
	Amount	uint64 			`json:"amount"`
}

type CreateToken struct {
	User	eos.AccountName	`json:"user"`
	Amount	uint32 			`json:"amount"`
}

type DestroyToken struct {
	User	eos.AccountName	`json:"user"`
	Amount	uint32 			`json:"amount"`
	Memo	string 			`json:"amount"`
}

type NewAccount struct {
	User	eos.AccountName	`json:"user"`
}
