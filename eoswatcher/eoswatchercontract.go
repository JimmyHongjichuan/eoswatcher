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

type EOSWatcherContract struct {
	// 链API url
	EosAPI						*eos.API
	// 代币所在合约
	ActionAccount				eos.AccountName
	// 网关名
	Gateway						eos.AccountName

	ScanBlockHeight				uint32

	HeadBlockNum				uint32
	LastIrreversibleBlockNum	uint32
	EOSPushEvents				[]*EOSPushEvent

	// 签名所用私钥 对应的公钥哈希值
	PubKeyHash 					string

	// 具体扫哪些操作
	ActionNameDestroy			eos.ActionName
	ActionNameCreate			eos.ActionName

	// 货币名称、精度
	Symbol						string
	Precision					uint8

	DB							*leveldb.DB
}

func NewEosWatcherContract(url, pubKeyHash, actionAccount, gateway, actionNameDestroy, actionNameCreate, symbol string, precision uint8, dirName string) (*EOSWatcherContract) {
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

	eos.RegisterAction(eos.AN(actionAccount), eos.ActN(actionNameDestroy), Solvent{})
	eos.RegisterAction(eos.AN(actionAccount), eos.ActN(actionNameCreate), token.Issue{})

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

	ew := &EOSWatcherContract{
		EosAPI:						api,
		ScanBlockHeight:			temp_sacn,
		HeadBlockNum:				0,
		LastIrreversibleBlockNum:	0,
		EOSPushEvents:				nil,
		PubKeyHash:					pubKeyHash,
		ActionAccount:				eos.AN(actionAccount),
		Gateway:					eos.AN(gateway),
		ActionNameDestroy:			eos.ActN(actionNameDestroy),
		ActionNameCreate:			eos.ActN(actionNameCreate),
		Symbol:						symbol,
		Precision:					precision,
		DB:							db,
	}
	return ew
}

func (ew *EOSWatcherContract) UpdatePubKeyHash(pubKeyHash string) {
	ew.PubKeyHash = pubKeyHash
}

func (ew *EOSWatcherContract) UpdateActionAccount(actionAccount string) {
	ew.ActionAccount = eos.AN(actionAccount)
	eos.RegisterAction(ew.ActionAccount, ew.ActionNameDestroy, Solvent{})
	eos.RegisterAction(ew.ActionAccount, ew.ActionNameCreate, token.Issue{})
}

func (ew *EOSWatcherContract) UpdateGateway(gateway string) {
	ew.Gateway = eos.AN(gateway)
}

func (ew *EOSWatcherContract) UpdateActionNameDestroy(actionNameDestroy string) {
	ew.ActionNameDestroy = eos.ActN(actionNameDestroy)
	eos.RegisterAction(ew.ActionAccount, ew.ActionNameDestroy, Solvent{})
}

func (ew *EOSWatcherContract) UpdateActionNameCreate(actionNameCreate string) {
	ew.ActionNameCreate = eos.ActN(actionNameCreate)
	eos.RegisterAction(ew.ActionAccount, ew.ActionNameCreate, token.Issue{})
}

//扫块开始
func (ew *EOSWatcherContract) StartWatch(scanBlockHeight, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent, channelCount int)  {
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
func (ew *EOSWatcherContract) UpdateInfo () {
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
func (ew *EOSWatcherContract) UpdateBlock (scanBlockHeight uint32)  *eos.BlockResp {
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
func (ew *EOSWatcherContract) UpdateEOSPushEvent (scanBlockResp *eos.BlockResp, scanBlockIndex uint32, eventChan chan<- *EOSPushEvent) {
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

			// 判断是否是ew.ActionAccount 合约 的ew.ActionNameDestroy操作
			if action.Account != ew.ActionAccount || action.Name != ew.ActionNameDestroy {
				continue
			}

			//dataType := reflect.TypeOf(action.ActionData.Data)
			//			//fmt.Println(dataType)
			// 解析action 中具体传给合约的参数 data
			destroyToken, ok := action.ActionData.Data.(*Solvent)
			if ok == false {
				continue
			}
			if destroyToken.Quantity.Symbol.Precision != ew.Precision {
				continue
			}
			if destroyToken.Quantity.Symbol.Symbol != ew.Symbol {
				continue
			}

			eosPushEvent := &EOSPushEvent{
				TxID:				transactionReceipt.Transaction.ID,

				Account:			action.Account,
				Name:				action.Name,
				Memo:				destroyToken.Memo,
				Amount:				uint64(destroyToken.Quantity.Amount),

				BlockNum:			scanBlockResp.BlockNum,
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

func (ew *EOSWatcherContract) GetEventByTxid(txid string) (*EOSPushEvent, error) {
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
		solvent, ok := action.ActionData.Data.(map[string]interface{})
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
		if assert.Symbol.Precision != ew.Precision {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if assert.Symbol.Symbol != ew.Symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}

		memo, ok := solvent["memo"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field  error")
		}

		destroyToken := &Solvent{
			From:		eos.AN(from),
			Quantity:	assert,
			Memo:		memo,
		}

		eosPushEvent := &EOSPushEvent{
			TxID:				transactionResp.ID,

			Account:			action.Account,
			Name:				action.Name,
			Memo:				destroyToken.Memo,
			Amount:				uint64(destroyToken.Quantity.Amount),

			BlockNum:			transactionResp.BlockNum,
			Index:				0, // 暂时没有好方法获取index
		}
		return eosPushEvent, nil
	}

	// 处理铸币
	if action.Name == ew.ActionNameCreate {
		issue, ok := action.ActionData.Data.(map[string]interface{})
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
		if assert.Symbol.Precision != ew.Precision {
			return nil, errors.New("Action Data 'quantity' field precision error.")
		}
		if assert.Symbol.Symbol != ew.Symbol {
			return nil, errors.New("Action Data 'quantity' field symbol error.")
		}

		memo, ok := issue["memo"].(string)
		if ok == false {
			return nil, errors.New("Action Data 'user' field  error")
		}

		createToken := &token.Issue{
			To:			eos.AN(to),
			Quantity:	assert,
			Memo:		memo,
		}

		eosPushEvent := &EOSPushEvent{
			TxID:				transactionResp.ID,

			Account:			action.Account,
			Name:				action.Name,
			Memo:				createToken.Memo,
			Amount:				uint64(createToken.Quantity.Amount),

			BlockNum:			transactionResp.BlockNum,
			Index:				0, // 暂时没有好方法获取index
		}
		return eosPushEvent, nil
	}

	return nil, errors.New("Name doesn't match.")
}

// 根据multisig下的PKMSign代码，移植过来
func (ew *EOSWatcherContract) PKMSign(tx *eos.SignedTransaction) (sig *ecc.Signature, err error) {
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
func (ew *EOSWatcherContract) GetPublickeyFromTx(tx *eos.SignedTransaction, sig *ecc.Signature) (out ecc.PublicKey, err error) {
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
func (ew *EOSWatcherContract) MergeSignedTx(tx *eos.SignedTransaction, sigs ...*ecc.Signature) (packedTx *eos.PackedTransaction, err error) {
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
func (ew *EOSWatcherContract) SendTx(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error) {
	out, err = ew.EosAPI.PushTransaction(tx)
	if err != nil {
		log.Error("send tx err:", err.Error())
		return nil, err
	}
	return
}

// 创建转账action
func (ew *EOSWatcherContract) CreateTransferAction(from, to string, amount int64, memo string) (*eos.Action, error) {
	//quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: 4, Symbol: "EOS"}}
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: ew.Precision, Symbol: ew.Symbol}}

	//tokenTransfer := token.NewTransfer(ew.ActionAccount, eos.AN(to), quantity, memo)

	//如果是自定义合约，那么不能利用已有的"token.NewTransfer"方法
	tokenTransfer := &eos.Action{
		Account: ew.ActionAccount,
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
func (ew *EOSWatcherContract) CreateCreateAction(issuer string, amount int64) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: ew.Precision, Symbol: ew.Symbol}}

	//tokenCreater := token.NewCreate(ew.ActionAccount, quantity)

	tokenCreater := &eos.Action{
		Account: ew.ActionAccount,
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

// 创建发行action
func (ew *EOSWatcherContract) CreateIssueAction(to string, amount int64, memo string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: ew.Precision, Symbol: ew.Symbol}}

	//tokenIssuer := token.NewIssue(eos.AN(to), quantity, memo)

	tokenIssuer := &eos.Action{
		Account: ew.ActionAccount,
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
func (ew *EOSWatcherContract) CreateSolventAction(from string, amount int64, memo string) (*eos.Action, error) {
	quantity := eos.Asset{Amount: amount, Symbol: eos.Symbol{Precision: ew.Precision, Symbol: ew.Symbol}}

	tokenolvent := &eos.Action{
		Account: ew.ActionAccount,
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
func (ew *EOSWatcherContract) CreateSetCodeAction(wasmFile, abiFile string) ([]*eos.Action, error) {
	setContract, err := system.NewSetContract(ew.Gateway, wasmFile, abiFile)
	if err != nil {
		return nil, err
	}
	return setContract, nil
}

// 根据action等，创建交易
func (ew *EOSWatcherContract) CreateTx(action *eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction([]*eos.Action{action}, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

// 根据多个actions等，创建交易
func (ew *EOSWatcherContract) CreateActionsTx(action []*eos.Action, duration time.Duration) (*eos.SignedTransaction, error) {
	infoResp, err := ew.EosAPI.GetInfo()

	//生成未签名交易
	tx := eos.NewTransaction(action, &eos.TxOptions{HeadBlockID: infoResp.HeadBlockID})
	tx.SetExpiration(duration)

	//生成签名交易
	stx := eos.NewSignedTransaction(tx)

	return stx, err
}

func (ew *EOSWatcherContract) NewPublicKey(uncompresspubkey string) (*ecc.PublicKey, error) {
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

type Solvent struct {
	From		eos.AccountName `json:"from"`
	Quantity	eos.Asset       `json:"quantity"`
	Memo		string          `json:"memo"`
}
