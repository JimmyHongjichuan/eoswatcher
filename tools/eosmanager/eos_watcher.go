package eosmanager

import (
	"eosc/tools/model"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"

	log "github.com/inconshreveable/log15"
)

type EosWatcher struct {
	eosClient             *EosClient
	scanHeight            uint32 //已扫过的最高区块高度
	scanIrHeight          uint32 //已扫过的最高不可逆转区块高度
	irreversibleBlockChan chan *model.BlockResp
	unConfirmBlockChan    chan *model.BlockResp
	redisPool             *redis.Pool
}

func NewEosWatcher(baseURL string, scanHeight uint32, redisPool *redis.Pool) *EosWatcher {
	eosClient := NewEosClient(baseURL)
	eosClient.SetChainID()
	ew := EosWatcher{
		eosClient:             eosClient,
		scanHeight:            scanHeight,
		scanIrHeight:          scanHeight,
		irreversibleBlockChan: make(chan *model.BlockResp, 250),
		unConfirmBlockChan:    make(chan *model.BlockResp, 250),
		redisPool:             redisPool,
	}
	return &ew
}

func (ew *EosWatcher) GetEosClient() *EosClient {
	return ew.eosClient
}

func (ew *EosWatcher) GetIrreversibleBlockChan() <-chan *model.BlockResp {
	return ew.irreversibleBlockChan
}
func (ew *EosWatcher) GetUnConfirmBlockChan() <-chan *model.BlockResp {
	return ew.unConfirmBlockChan
}

func (ew *EosWatcher) CheckConfirmOfTx(txid string) bool {
	return true
}

/*
func (ew *EosWatcher) WatchIrreversibleBlock() {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer func() {
			wg.Done()
		}()
		for {
			blockHeight := ew.eosClient.GetIrreversibleBlockHeight()
			if blockHeight-ew.scanConfirmHeight <= 0 {
				time.Sleep(time.Duration(1) * time.Second)
				continue
			}
			for {
				blockData, err := ew.eosClient.GetBlockByID(ew.scanConfirmHeight)
				if err != nil {
					break
				}
				select {
				case ew.confirmBlockChan <- blockData:
					ew.scanConfirmHeight++
				default:
					time.Sleep(time.Duration(1) * time.Second)
				}
			}
		}
	}()
}
*/

//链上数据分为已确认和未确认的写入chan
func (ew *EosWatcher) WatchAllBlock() {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer func() {
			wg.Done()
		}()
		//从redis中获取持久化高度
		log.Info("blockHeightFromRedis", "height", ew.scanIrHeight)
		if ew.scanIrHeight == 0 {
			log.Info("No ScanHeight Pass in")
			n := ew.GetScanHeightFromRedis()
			ew.scanIrHeight = n
			log.Debug("Get Irreversible block number from redis:", "high", n)
		}

		for {
			//当前块高度、不可逆转块高度
			lastIrreversibleBlockHeight := ew.eosClient.GetIrreversibleBlockHeight()
			//此处blockHeight-ew.scanIrHeight, 如果小于0（uint32），就溢出了
			if lastIrreversibleBlockHeight <= ew.scanIrHeight {
				log.Info("watcher sleeping...., restart 1s later")
				time.Sleep(time.Duration(1) * time.Second)
				continue
			}
			for {
				infoResp, err := ew.eosClient.GetInfo()
				if err != nil {
					log.Error("Get info error!")
					break
				}
				blockHeight := infoResp.HeadBlockNum
				lastIrreversibleBlockHeight := infoResp.LastIrreversibleBlockNum
				if ew.scanIrHeight < lastIrreversibleBlockHeight {
					for {
						if ew.scanIrHeight == lastIrreversibleBlockHeight {
						//if true{
							break
						}
						blockData, err := ew.eosClient.GetBlockByID(ew.scanIrHeight)
						log.Debug("Irreversible Block scaning:", "scanIrHeight: ", ew.scanIrHeight)
						if err != nil {
						//if true{
							log.Error("err: ", err, "blockheight: ", ew.scanIrHeight)
							ew.scanIrHeight++
							break
						}
						ew.irreversibleBlockChan <- blockData
						//每1000个区块存一次区块高度到redis
						if ew.scanIrHeight%1000 == 0 {
							ew.UpdateScanHeightToRedis(ew.scanIrHeight)
						}
						ew.scanIrHeight++
					}
				} else {
					if ew.scanIrHeight == lastIrreversibleBlockHeight && ew.scanHeight < blockHeight {
						if ew.scanHeight <= ew.scanIrHeight {
							ew.scanHeight = ew.scanIrHeight + 1
						}
						blockData, err := ew.eosClient.GetBlockByID(ew.scanHeight)
						log.Debug("Reversible Block scaning:", "scanHeight: ", ew.scanHeight)
						if err != nil {
							log.Error("err: ", err, "blockheight: ", ew.scanHeight)
							ew.scanHeight++
							break
						}
						ew.unConfirmBlockChan <- blockData
					}
				}
				//blockData, err := ew.eosClient.GetBlockByID(ew.scanHeight)
				//if err != nil {
				//	log.Error("err", err, "blockheight", ew.scanHeight)
				//	ew.scanHeight++
				//	fmt.Println("999999999", ew.scanHeight)
				//	break
				//}
				//fmt.Println("blockData.BlockNum: ", blockData.BlockNum)
				//fmt.Println("lastIrreversibleBlockHeight: ", lastIrreversibleBlockHeight)
				//if blockData.BlockNum > lastIrreversibleBlockHeight {
				//	ew.unConfirmBlockChan <- blockData
				//} else {
				//	ew.irreversibleBlockChan <- blockData
				//}
				//ew.scanHeight++
				////每1000个区块存一次区块高度到redis
				//if ew.scanHeight%1000 == 0 {
				//	ew.UpdateScanHeightToRedis(ew.scanHeight)
				//}

				if ew.scanIrHeight >= lastIrreversibleBlockHeight {
					time.Sleep(time.Duration(1) * time.Second)
					break
				}
			}
		}
	}()
	wg.Wait()
}

func (ew *EosWatcher) UpdateScanHeightToRedis(height uint32) bool {
	conn := ew.redisPool.Get()
	defer conn.Close()
	conn.Do("SELECT", 6) //确定redis
	n, err := conn.Do("SET", "eosscanirheight", int64(height))
	if err != nil {
		log.Error("Update ScanHeight To Redis", "err", err)
		return false
	}
	if n != "OK" {
		log.Error("Update ScanHeight To Redis", "resp n", n)
		return false
	}
	return true
	//heightString := strconv.FormatUint(uint64(height), 10)
	//imap := map[string]string{"eosscanheight": heightString}

}

func (ew *EosWatcher) GetScanHeightFromRedis() uint32 {
	conn := ew.redisPool.Get()
	defer conn.Close()
	conn.Do("SELECT", 6)
	n, err := redis.Int64(conn.Do("Get", "eosscanirheight"))
	if err != nil {
		log.Error("Get EosScanHeight", "err", err)
		return 0
	} else {
		return uint32(n)
	}

}
