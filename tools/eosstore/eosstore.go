package eosstore

import (
	"encoding/json"
	"eosc/tools/eosmanager"
	"eosc/tools/model"
	"io"
	"sync"
	"time"

	"github.com/eoscanada/eos-go"
	"github.com/gomodule/redigo/redis"
	log "github.com/inconshreveable/log15"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type EosStore struct {
	ew                 *eosmanager.EosWatcher
	esClient           *ESClient
	redisPool          *redis.Pool
	//irreversibleTxChan chan *model.TransferData
	irTxChanForGate    chan *model.TransferData
}

//type PushEvent struct {
//	TransferData       *model.TransferData
//}

func NewEosStore(baseUrl string, scanHeight uint32, redisPool *redis.Pool, esClient *ESClient) *EosStore {
	ew := eosmanager.NewEosWatcher(baseUrl, scanHeight, redisPool)
	return &EosStore{
		ew:                 ew,
		esClient:           esClient,
		redisPool:          redisPool,
		//irreversibleTxChan: make(chan *model.TransferData, 1000),
		irTxChanForGate:    make(chan *model.TransferData, 1000),
	}
}

func (es *EosStore) GetApi() *eos.API {
	return es.ew.GetEosClient().EosAPI
}

func (es *EosStore) GetIrTxChan() <-chan *model.TransferData {
	return es.irTxChanForGate
}

//获取到的区块信息，irreversible数据和unconfirm数据分别写入不同的ES index，
//irreversible向网关推送“交易确认”消息，unconfirm不推送消息
//固定时间间隔pop出unconfirm表中已不可逆转的交易，并删除
func (es *EosStore) Start() {
	es.ew.WatchAllBlock()
	es.ScanIrreversibleOfUnconfirmTx()

	irreversibleBlockChan := es.ew.GetIrreversibleBlockChan()
	unconfirmBlockChan := es.ew.GetUnConfirmBlockChan()
	//irreversibleTxChan := es.irreversibleTxChan
	go func() {
		for {
			select {
			case confirmBlock := <-irreversibleBlockChan:
				es.processConfirmBlock(confirmBlock)
			case unconfirmBlock := <-unconfirmBlockChan:
				es.processUnconfirmBlock(unconfirmBlock)
			//case irreversibleTx := <-irreversibleTxChan:
			//	es.saveTxtoEs(irreversibleTx, 1)
			}
		}
	}()


}

func (es *EosStore) processConfirmBlock(confirmBlock *model.BlockResp) bool {
	log.Info("processConfirmBlock", "Irreversible Block Height", confirmBlock.BlockNum)
	for _, tx := range confirmBlock.Transactions {
		for _, act := range tx.Actions {
			//log.Info("act_name", "data", act.Name)
			if act.Name == "transfer" && act.Account == "kylineoshjx3" {
				newTransferData := &model.TransferData{
					BlockNum:       confirmBlock.BlockNum,
					BlockID:        confirmBlock.BlockID,
					BlockTimeStamp: confirmBlock.BlockTimeStamp.Unix(),
					Producer:       confirmBlock.Producer,
					TxID:           tx.ID,
					Status:         tx.Status,
					Expiration:     tx.Expiration.Unix(),
					RefBlockNum:    tx.RefBlockNum,
					RefBlockPrefix: tx.RefBlockPrefix,
					DelaySec:       tx.DelaySec,
					Account:        act.Account,
					Name:           act.Name,
				}
				err := mapstructure.Decode(act.Data, newTransferData)
				if err != nil {
					log.Error("MapToStruct", "err", err)
					return false
				}

				if newTransferData.Quantity != "" {
					asset, err := eos.NewAsset(newTransferData.Quantity)
					if err != nil {
						log.Error("Parse Quantity", "err", err)
						return false
					}
					newTransferData.Amount = asset.Amount
					newTransferData.Symbol = asset.Symbol.Symbol
					newTransferData.Precision = asset.Precision
					/*
						tokeninfo := strings.Split(newTransferData.Quantity, " ")
						if len(tokeninfo) != 2 {
							log.Error("Unkonw Token Format", "Quantity", newTransferData.Quantity)
							return false
						}
						f, err := strconv.ParseFloat(tokeninfo[0], 32)
						if err != nil {
							log.Error("Strconv Float", "err", err)
						}
						newTransferData.Amount = int64(f * 10000) //hardcode
						newTransferData.Precision = 4
						newTransferData.Symbol = tokeninfo[1]
					*/
				}
				/*
					//t := reflect.TypeOf(act.Data)
					v := reflect.ValueOf(act.Data)
					//typeOfT := v.Type()
					for i := 0; i < v.NumField(); i++ {
						typeOfT := v.Type()
						f := v.Field(i)
						if typeOfT.Field(i).Name == "From" {
							data := f.Interface().(string)
							newTransferData.From = data
						} else if typeOfT.Field(i).Name == "To" {
							data := f.Interface().(string)
							newTransferData.To = data
						} else if typeOfT.Field(i).Name == "Memo" {
							data := f.Interface().(string)
							newTransferData.Memo = data
						} else if typeOfT.Field(i).Name == "Quantity" {
							v := reflect.ValueOf(f.Interface())
							for i := 0; i < v.NumField(); i++ {
								typeOfT := v.Type()
								f := v.Field(i)
								if typeOfT.Field(i).Name == "Amount" {
									data := f.Interface().(int64)
									newTransferData.Amount = data
								} else if typeOfT.Field(i).Name == "Symbol" {
									v := reflect.ValueOf(f.Interface())
									for i := 0; i < v.NumField(); i++ {
										typeOfT := v.Type()
										f := v.Field(i)
										if typeOfT.Field(i).Name == "Symbol" {
											data := f.Interface().(string)
											newTransferData.Symbol = data
										} else if typeOfT.Field(i).Name == "Precision" {
											data := f.Interface().(uint8)
											newTransferData.Precision = data
										}
									}
								}
							}
						}
						//fmt.Printf("%d: %s %s = %v\n", i,
						//	typeOfT.Field(i).Name, f.Type(), f.Interface())

					}*/
				//fmt.Printf("%+v\n", newTransferData)
				//将不可逆转交易，发送给网关
				log.Debug("es.irTxChanForGate")
				es.irTxChanForGate <- newTransferData
				if !es.saveTxtoEs(newTransferData, 1) {
					return false
				}
				//if viper.GetInt("REIDS.is_push") == 1 {
				//	es.publishTxMessage(newTransferData, 1)
				//}
			}
		}
	}
	return true
}

func (es *EosStore) processUnconfirmBlock(unconfirmBlock *model.BlockResp) bool {
	log.Info("processUnconfirmBlock", "Reversible Block Height", unconfirmBlock.BlockNum)
	for _, tx := range unconfirmBlock.Transactions {
		for _, act := range tx.Actions {
			//log.Info("act_name", "data", act.Name)
			if act.Name == "transfer" && act.Account == "kylineoshjx3" {
				newTransferData := &model.TransferData{
					BlockNum:       unconfirmBlock.BlockNum,
					BlockID:        unconfirmBlock.BlockID,
					BlockTimeStamp: unconfirmBlock.BlockTimeStamp.Unix(),
					Producer:       unconfirmBlock.Producer,
					TxID:           tx.ID,
					Status:         tx.Status,
					Expiration:     tx.Expiration.Unix(),
					RefBlockNum:    tx.RefBlockNum,
					RefBlockPrefix: tx.RefBlockPrefix,
					DelaySec:       tx.DelaySec,
					Account:        act.Account,
					Name:           act.Name,
				}
				err := mapstructure.Decode(act.Data, newTransferData)
				if err != nil {
					log.Error("MapToStruct", "err", err)
					return false
				}

				if newTransferData.Quantity != "" {
					asset, err := eos.NewAsset(newTransferData.Quantity)
					if err != nil {
						log.Error("Parse Quantity", "err", err)
						return false
					}
					newTransferData.Amount = asset.Amount
					newTransferData.Symbol = asset.Symbol.Symbol
					newTransferData.Precision = asset.Precision
					/*
						tokeninfo := strings.Split(newTransferData.Quantity, " ")
						if len(tokeninfo) != 2 {
							log.Error("Unkonw Token Format", "Quantity", newTransferData.Quantity)
							return false
						}
						f, err := strconv.ParseFloat(tokeninfo[0], 32)
						if err != nil {
							log.Error("Strconv Float", "err", err)
						}
						newTransferData.Amount = int64(f * 10000) //hardcode
						newTransferData.Precision = 4
						newTransferData.Symbol = tokeninfo[1]
					*/
				}
				//fmt.Printf("%+v\n", newTransferData)
				if !es.saveTxtoEs(newTransferData, 0) {
					return false
				}
				//go func() {
				//	es.publishTxMessage(newTransferData, 0)
				//}()
			}
		}
	}
	return true
}

//从ElasticSearch中读出unconfirmTx, 判断所在块高度是否已经不可逆， 如果不可逆， 则删除改交易
//目前EOS确认块（irreversible）大概是360个块（后面可能是两个块1s），也可以把当前块前推360个块来做确认。
func (es *EosStore) ScanIrreversibleOfUnconfirmTx() {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer func() {
			wg.Done()
		}()
		for {
			eosClient := es.ew.GetEosClient()
			lastIrreversibleBlockHeight := eosClient.GetIrreversibleBlockHeight()
			log.Info("Scan UnconfirmtxIndex", "lastirreversibleblockheight", lastIrreversibleBlockHeight)
			// Initialize scroller. Just don't call Do yet.
			scroll := es.esClient.client.Scroll("eos-tranx-unconfirm").Type("_doc").Size(1000)
			for {
				results, err := scroll.Do(es.esClient.ctx)
				if err == io.EOF {
					log.Info("Scan Over...., Rescan 60s later")
					break // all results retrieved
				}
				if err != nil {
					log.Error("ScanIrreversibleOfUnconfirmTx", "err", err)
					break
				}
				//handle results
				for _, hit := range results.Hits.Hits {
					var transferdata *model.TransferData
					err := json.Unmarshal(*hit.Source, &transferdata)
					if err != nil {
						log.Error("MarshalTransferData", "err", err)
						continue
					}

					if lastIrreversibleBlockHeight >= transferdata.BlockNum {

						//从Index中删除这条记录，
						es.esClient.DeleteUnconfirmTx(transferdata.TxID)
						//把这条记录append到irreversibleTxChan
						//es.irreversibleTxChan <- transferdata
					}
				}
			}
			//每间隔10s扫一次
			time.Sleep(time.Duration(60) * time.Second)
		}
	}()
	wg.Wait()
}

//info=0:已确认交易， info=1:未确认交易
func (es *EosStore) saveTxtoEs(data *model.TransferData, status int8) bool {
	if !es.esClient.UpdateTxInfo(data, status) {
		return false
	}
	return true
}

//info=0:已确认交易， info=1:未确认交易
func (es *EosStore) publishTxMessage(data *model.TransferData, status int8) bool {
	var timestamp int64
	switch status {
	case 0:
		timestamp = time.Now().Unix()
	case 1:
		timestamp = data.BlockTimeStamp
	default:
		timestamp = time.Now().Unix()
	}

	message := model.PushMessage{
		TxID:      data.TxID,
		Status:    status,
		BlockNum:  data.BlockNum,
		From:      data.From,
		To:        data.To,
		TimeStamp: timestamp,
		Value:     data.Quantity,
	}

	msg, err := json.Marshal(message)
	if err != nil {
		log.Error("Marshal publish msg", "err", err)
	}
	conn := es.redisPool.Get()
	defer conn.Close()
	//更改推送模式为队列PUSH
	_, err = conn.Do("LPUSH", viper.GetString("REDIS.tx_push_key"), string(msg))
	if err != nil {
		log.Error("Push Txmsg failed", "err", err)
		return false
	}

	return true
}
