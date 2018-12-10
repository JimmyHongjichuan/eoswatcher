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
	//"reflect"
	"time"
)

type TokenContract struct {
	// 合约名
	ActionAccount				eos.AccountName
	// 溶币方法名
	ActionNameDestroy			eos.ActionName
	// 铸币方法名
	ActionNameCreate			eos.ActionName

	// 货币名称、精度
	Symbol						string
	Precision					uint8
}

type EOSWatcherMain struct {
	// 链API url
	EosAPI						*eos.API
	// 网关名
	Gateway						eos.AccountName

	ScanBlockHeight				uint32

	HeadBlockNum				uint32
	LastIrreversibleBlockNum	uint32

	// 签名所用私钥 对应的公钥哈希值
	PubKeyHash 					string

	// 合约、方法列表
	TokenContracts				[]*TokenContract

	DB							*leveldb.DB
}

func NewEosWatcherMain(url, pubKeyHash, gateway, dirName string, tokenContracts []*TokenContract) (*EOSWatcherMain) {
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

	// 方法注册
	for _, tokenContract := range tokenContracts{
		if tokenContract.ActionAccount != eos.AN("eosio.token") {
			eos.RegisterAction(tokenContract.ActionAccount, tokenContract.ActionNameDestroy, Solvent{})
			eos.RegisterAction(tokenContract.ActionAccount, tokenContract.ActionNameCreate, token.Issue{})
		}
	}

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

	ew := &EOSWatcherMain{
		EosAPI:						api,
		ScanBlockHeight:			temp_sacn,
		HeadBlockNum:				0,
		LastIrreversibleBlockNum:	0,
		PubKeyHash:					pubKeyHash,
		Gateway:					eos.AN(gateway),
		TokenContracts:				tokenContracts,
		DB:							db,
	}
	return ew
}

func (ew *EOSWatcherMain) ShowRegist() {
	for contract := range eos.RegisteredActions {
		fmt.Printf("ContractRegister:%s->%s\n", contract, eos.RegisteredActions[contract])
	}
}

func (ew *EOSWatcherMain) UpdatePubKeyHash(pubKeyHash string) {
	ew.PubKeyHash = pubKeyHash
}

func (ew *EOSWatcherMain) UpdateGateway(gateway string) {
	ew.Gateway = eos.AN(gateway)
}

//扫块开始
func (ew *EOSWatcherMain) StartWatch(scanBlockHeight, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent, channelCount int)  {
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
func (ew *EOSWatcherMain) UpdateInfo () {
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
func (ew *EOSWatcherMain) UpdateBlock (scanBlockHeight uint32)  *eos.BlockResp {
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
func (ew *EOSWatcherMain) UpdateEOSPushEvent (scanBlockResp *eos.BlockResp, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent) {

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

			for _, tokenContract := range ew.TokenContracts {
				// 检查是否是需要的合约中交易
				if action.Account == tokenContract.ActionAccount {
					if action.Name == tokenContract.ActionNameDestroy {
						eosPushEvent, err := ew.ActionDataParse(action.Name, action.ActionData.Data, tokenContract.Precision, tokenContract.Symbol)
						if err == nil {
							eosPushEvent.TxID = transactionReceipt.Transaction.ID
							eosPushEvent.Account = action.Account
							eosPushEvent.Name = action.Name
							eosPushEvent.BlockNum = scanBlockResp.BlockNum
							eosPushEvent.Index = index
							eventChan <- eosPushEvent
							break
						}
					}
					// 扫块只需要扫溶币交易，不需要扫铸币交易
					//
					//} else if action.Name == tokenContract.ActionNameCreate {
					//	eosPushEvent, err := ew.ActionDataParse(action.Name, action.ActionData.Data, tokenContract.Precision, tokenContract.Symbol)
					//	if err == nil {
					//		eosPushEvent.TxID = transactionReceipt.Transaction.ID
					//		eosPushEvent.Account = action.Account
					//		eosPushEvent.Name = action.Name
					//		eosPushEvent.BlockNum = scanBlockResp.BlockNum
					//		eosPushEvent.Index = 0
					//		eventChan <- eosPushEvent
					//	}
					//	continue
					//}
				}
			}

		} //else {
		//	// 暂时忽略特殊交易，比如misg 的exec 产生的交易。（待优化）
		//	log.Error("Ignore transaction without transaction body.", "Block Height", ew.ScanBlockHeight)
		//}
	}
}

func (ew *EOSWatcherMain) ActionDataTransferParse(data interface{}, precision uint8, symbol string) (*token.Transfer, error){
	var transferToken *token.Transfer
	var ok bool

	transferToken, ok = data.(*token.Transfer)
	if ok == true {
		// 扫块时，才会执行这里
		// 扫块只扫溶到网关账户的交易
		// 必须是转给网关的交易，才被认为是溶币操作
		if transferToken.To != ew.Gateway {
			return nil, errors.New("Action Data 'to' field is not gateway.")
		}
		if transferToken.Quantity.Symbol.Precision != precision {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if transferToken.Quantity.Symbol.Symbol != symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}
	} else {
		// GetEventByTxid的时候才会执行这里
		//
		var transfer map[string]interface{}

		transfer, ok = data.(map[string]interface{})
		if ok == false {
			return nil, errors.New("Action Data reflect error.")
		}

		if len(transfer) != 4 {
			return nil, errors.New("Action Data length error.")
		}

		// 解析actionData的具体字段
		from, ok := transfer["from"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'from' field error")
		}

		to, ok := transfer["to"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'to' field error")
		}

		if eos.AN(from) != ew.Gateway && eos.AN(to) != ew.Gateway {
			return nil, errors.New("Transaction is not related to the gateway")
		}

		quantity, ok := transfer["quantity"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'quantity' field error")
		}
		assert, err := eos.NewAsset(quantity)
		if err != nil {
			return nil, errors.New("Action Data 'quantity' field unmarshal error.")
		}
		if assert.Symbol.Precision != precision {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if assert.Symbol.Symbol != symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}

		memo, ok := transfer["memo"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field  error")
		}

		transferToken = &token.Transfer{
			From:		eos.AN(from),
			To:			eos.AN(to),
			Quantity:	assert,
			Memo:		memo,
		}
	}

	return transferToken, nil
}

func (ew *EOSWatcherMain) ActionDataSolventParse(data interface{}, precision uint8, symbol string) (*Solvent, error){
	var solventToken *Solvent
	var ok bool

	solventToken, ok = data.(*Solvent)
	if ok == true {
		// 扫块时，才会执行这里
		//
		if solventToken.Quantity.Symbol.Precision != precision  {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if solventToken.Quantity.Symbol.Symbol != symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}
	} else {
		// GetEventByTxid的时候才会执行这里
		//
		solvent, ok := data.(map[string]interface{})
		if ok == false {
			return nil, errors.New("Action Data reflect error.")
		}

		if len(solvent) != 3 {
			return nil, errors.New("Action Data length error.")
		}

		// 解析actionData的具体字段
		from, ok := solvent["from"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field error")
		}

		quantity, ok := solvent["quantity"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'amount' field error")
		}
		assert, err := eos.NewAsset(quantity)
		if err != nil {
			return nil, errors.New("Action Data 'quantity' field unmarshal error.")
		}
		if assert.Symbol.Precision != precision {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if assert.Symbol.Symbol != symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}

		memo, ok := solvent["memo"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field  error")
		}

		solventToken = &Solvent{
			From:		eos.AN(from),
			Quantity:	assert,
			Memo:		memo,
		}
	}


	return solventToken, nil
}

func (ew *EOSWatcherMain) ActionDataIssueParse(data interface{}, precision uint8, symbol string) (*token.Issue, error){
	// 扫块只需要扫溶币交易，不需要扫铸币交易
	//
	issue, ok := data.(map[string]interface{})
	if ok == false {
		return nil, errors.New("Action Data reflect error.")
	}

	if len(issue) != 3 {
		return nil, errors.New("Action Data length error.")
	}

	// 解析actionData的具体字段
	to, ok := issue["to"].(string)
	if ok == false {
		return nil, errors.New("Action Data 'user' field error")
	}

	quantity, ok := issue["quantity"].(string)
	if ok == false {
		return nil, errors.New("Action Data 'amount' field error")
	}
	assert, err := eos.NewAsset(quantity)
	if err != nil {
		return nil, errors.New("Action Data 'quantity' field unmarshal error.")
	}
	if assert.Symbol.Precision != precision {
		return nil, errors.New("Action Data 'quantity' field precision error.")
	}
	if assert.Symbol.Symbol != symbol {
		return nil, errors.New("Action Data 'quantity' field symbol error.")
	}

	memo, ok := issue["memo"].(string)
	if ok == false {
		return nil, errors.New("Action Data 'user' field  error")
	}

	issueToken := &token.Issue{
		To:			eos.AN(to),
		Quantity:	assert,
		Memo:		memo,
	}
	return issueToken, nil
}

func (ew *EOSWatcherMain) ActionDataParse(actionName eos.ActionName, data interface{}, precision uint8, symbol string) (*EOSPushEvent, error){
	switch actionName {
	case eos.ActN("transfer"):
		transferToken, err := ew.ActionDataTransferParse(data, precision, symbol)
		if err != nil {
			return nil, err
		}
		eosPushEvent := &EOSPushEvent{
			Memo:		transferToken.Memo,
			Amount:		uint64(transferToken.Quantity.Amount),
			Symbol:		transferToken.Quantity.Symbol.Symbol,
			Precision:	transferToken.Quantity.Symbol.Precision,
		}
		return eosPushEvent, nil
	case eos.ActN("solvent"):
		solventToken, err := ew.ActionDataSolventParse(data, precision, symbol)
		if err != nil {
			return nil, err
		}
		eosPushEvent := &EOSPushEvent{
			Memo:		solventToken.Memo,
			Amount:		uint64(solventToken.Quantity.Amount),
			Symbol:		solventToken.Quantity.Symbol.Symbol,
			Precision:	solventToken.Quantity.Symbol.Precision,
		}
		return eosPushEvent, nil
	case eos.ActN("issue"):
		issueToken, err := ew.ActionDataIssueParse(data, precision, symbol)
		if err != nil {
			return nil, err
		}
		eosPushEvent := &EOSPushEvent{
			Memo:		issueToken.Memo,
			Amount:		uint64(issueToken.Quantity.Amount),
			Symbol:		issueToken.Quantity.Symbol.Symbol,
			Precision:	issueToken.Quantity.Symbol.Precision,
		}
		return eosPushEvent, nil
	}
	return nil, errors.New("Action data parse error.")
}

func (ew *EOSWatcherMain) GetEventByTxid(txid string) (*EOSPushEvent, error) {
	transactionResp, err := ew.EosAPI.GetTransaction(txid)
	if err != nil {
		return nil, err
	}

	// 暂时忽略一笔交易中有多个action 的情况。（待优化）
	if len(transactionResp.Transaction.Transaction.Actions) != 1 {
		return nil, errors.New("Transaction actions' length is not one.")
	}
	action := transactionResp.Transaction.Transaction.Actions[0]

	var eosPushEvent *EOSPushEvent
	for _, tokenContract := range ew.TokenContracts {
		// 检查是否是需要的合约中交易
		if action.Account == tokenContract.ActionAccount {
			if action.Name == tokenContract.ActionNameDestroy {
				eosPushEvent, err = ew.ActionDataParse(action.Name, action.ActionData.Data, tokenContract.Precision, tokenContract.Symbol)
				if err == nil {
					// 填充其他 event 参数
					eosPushEvent.TxID = transactionResp.ID
					eosPushEvent.Account = action.Account
					eosPushEvent.Name = action.Name
					eosPushEvent.BlockNum = transactionResp.BlockNum
					eosPushEvent.Index = 0
					return eosPushEvent, nil
				}
			} else if action.Name == tokenContract.ActionNameCreate {
				eosPushEvent, err = ew.ActionDataParse(action.Name, action.ActionData.Data, tokenContract.Precision, tokenContract.Symbol)
				if err == nil {
					// 填充其他 event 参数
					eosPushEvent.TxID = transactionResp.ID
					eosPushEvent.Account = action.Account
					eosPushEvent.Name = action.Name
					eosPushEvent.BlockNum = transactionResp.BlockNum
					eosPushEvent.Index = 0
					return eosPushEvent, nil
				}
			}
		}
	}

	return nil, errors.New("Action Account doesn't match.")

	//dataType := reflect.TypeOf(action.ActionData.Data)
	//fmt.Println(dataType)
}

// 根据multisig下的PKMSign代码，移植过来
func (ew *EOSWatcherMain) PKMSign(tx *eos.SignedTransaction) (sig *ecc.Signature, err error) {
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
func (ew *EOSWatcherMain) GetPublickeyFromTx(tx *eos.SignedTransaction, sig *ecc.Signature) (out ecc.PublicKey, err error) {
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
func (ew *EOSWatcherMain) MergeSignedTx(tx *eos.SignedTransaction, sigs ...*ecc.Signature) (packedTx *eos.PackedTransaction, err error) {
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
func (ew *EOSWatcherMain) SendTx(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error) {
	out, err = ew.EosAPI.PushTransaction(tx)
	if err != nil {
		log.Error("send tx err:", err.Error())
		return nil, err
	}
	return
}

func (ew *EOSWatcherMain) CreateTransferAction(from, to string, amount int64, memo string) (*eos.Action, error) {
	return ew.CreateTransferActionDispatcher("eosio.token", from, to, amount, 4, "EOS", memo)
}

// 创建转账action
func (ew *EOSWatcherMain) CreateTransferActionDispatcher(actionAccount, from, to string, amount int64, precision uint8, symbol string, memo string) (*eos.Action, error) {
	//quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: 4, Symbol: "EOS"}}
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: precision, Symbol: symbol}}

	//tokenTransfer := token.NewTransfer(ew.ActionAccount, eos.AN(to), quantity, memo)

	//如果是自定义合约，那么不能利用已有的"token.NewTransfer"方法
	tokenTransfer := &eos.Action{
		Account: eos.AN(actionAccount),
		Name:    eos.ActN("transfer"),
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

// 创建新建action
func (ew *EOSWatcherMain) CreateCreateAction(actionAccount, issuer string, amount int64, precision uint8, symbol string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: precision, Symbol: symbol}}

	//tokenCreater := token.NewCreate(ew.ActionAccount, quantity)

	tokenCreater := &eos.Action{
		Account: eos.AN(actionAccount),
		Name:    eos.ActN("create"),
		Authorization: []eos.PermissionLevel{
			{Actor: ew.Gateway, Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(token.Create{
			Issuer:        eos.AN(issuer),
			MaximumSupply: quantity,
		}),
	}
	return tokenCreater, nil
}

func (ew *EOSWatcherMain) CreateIssueAction(actionAccount, to string, amount int64, symbol string, memo string) (*eos.Action, error) {
	return ew.CreateIssueActionDispatcher(actionAccount, to, amount, 8, symbol, memo)
}

// 创建发行action
func (ew *EOSWatcherMain) CreateIssueActionDispatcher(actionAccount, to string, amount int64, precision uint8, symbol string, memo string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: precision, Symbol: symbol}}

	//tokenIssuer := token.NewIssue(eos.AN(to), quantity, memo)

	tokenIssuer := &eos.Action{
		Account: eos.AN(actionAccount),
		Name:    eos.ActN("issue"),
		Authorization: []eos.PermissionLevel{
			{Actor: ew.Gateway, Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(token.Issue{
			To:       eos.AN(to),
			Quantity: quantity,
			Memo:     memo,
		}),
	}
	return tokenIssuer, nil
}

// 创建溶币action
func (ew *EOSWatcherMain) CreateSolventAction(actionAccount, from string, amount int64, precision uint8, symbol string, memo string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: precision, Symbol: symbol}}

	tokenolvent := &eos.Action{
		Account: eos.AN(actionAccount),
		Name:    eos.ActN("solvent"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(from), Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(Solvent{
			From:     eos.AN(from),
			Quantity: quantity,
			Memo:     memo,
		}),
	}
	return tokenolvent, nil
}

// 创建部署合约action，部署在网关账户上面
func (ew *EOSWatcherMain) CreateSetCodeAction(wasmFile, abiFile string) ([]*eos.Action, error) {
	setContract, err := system.NewSetContract(ew.Gateway, wasmFile, abiFile)
	if err != nil {
		return nil, err
	}
	return setContract, nil
}

// 根据action等，创建交易
func (ew *EOSWatcherMain) CreateTx(action *eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction([]*eos.Action{action}, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

// 根据多个actions等，创建交易
func (ew *EOSWatcherMain) CreateActionsTx(action []*eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction(action, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

func (ew *EOSWatcherMain) NewPublicKey(uncompresspubkey string) (*ecc.PublicKey, error) {
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
