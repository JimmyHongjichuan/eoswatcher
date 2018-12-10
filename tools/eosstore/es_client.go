package eosstore

import (
	"context"
	"eosc/tools/model"
	"strings"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
)

type ESClient struct {
	sync.Mutex
	client *elastic.Client
	ctx    context.Context
	bulk   *elastic.BulkService
}

func NewESClient() *ESClient {
	ctx := context.Background()
	client, err := elastic.NewClient(
		elastic.SetURL(viper.GetString("ES.server")),
		elastic.SetSniff(false),
		//elastic.SetRetrier()
	)
	if err != nil {
		log.Error("Create ES client", "err", err)
		return nil
	}

	bulk := client.Bulk()

	return &ESClient{
		ctx:    ctx,
		client: client,
		bulk:   bulk,
	}
}

func (es *ESClient) UpdateTxInfo(transferData *model.TransferData, status int8) bool {
	es.Lock()
	defer es.Unlock()
	aliasName := viper.GetString("ES.alias_name")

	var indexDate string
	var indexName string
	if status == 0 { //未确认的交易
		indexName = strings.Join([]string{"eos-tranx-gate", "unconfirm"}, "-")
	} else if status == 1 { //不可逆转交易
		indexDate = time.Unix(transferData.BlockTimeStamp, 0).Format("200601")
		indexName = strings.Join([]string{"eos-tranx-gate", indexDate}, "-")
	} else {
		log.Error("unknow trx status", "status", status)
		return false
	}
	// Check if the index exists
	exists, err := es.client.IndexExists(indexName).Do(es.ctx)
	if err != nil {
		// Handle error
		log.Error("Check Index Fail", "err", err)
		return false
	}

	//add index if nessessary
	if !exists {
		createIndex, err := es.client.CreateIndex(indexName).BodyString(TxMapping).Do(es.ctx)
		if err != nil {
			// Handle error
			log.Error("Create Index Fail", "err", err)
			return false
		}
		if !createIndex.Acknowledged {
			// Not acknowledged
			log.Error("Create Index Fail", "acknowledged", createIndex.Acknowledged)
			return false
		}
	}

	//add alias  if nessessary
	if aliasName != indexName {
		aliasCreate, err := es.client.Alias().Add(indexName, aliasName).Do(es.ctx)
		if err != nil {
			log.Error("Add Alias Fail", "err", err)
			return false
		}
		if !aliasCreate.Acknowledged {
			// Not acknowledged
			log.Error("Create Index Fail", "acknowledged", aliasCreate.Acknowledged)
			return false
		}
	}

	log.Debug("Update TX to ES", "indexName", indexName, "TX_ID", transferData.TxID)
	updateData := elastic.NewBulkUpdateRequest().Index(indexName).Type("_doc").Id(string(transferData.TxID)).Doc(transferData).DocAsUpsert(true)
	es.bulk.Add(updateData)
	if es.bulk.NumberOfActions() != 1 {
		log.Error("Update Tx to Es Add bulk Failed")
		return false
	}
	//bulkResp, err := es.bulk.Do(es.ctx)
	_, err = es.bulk.Do(es.ctx)
	if err != nil {
		log.Error("Bulk Update Failed", "err", err)
		return false
	}
	if es.bulk.NumberOfActions() != 0 {
		log.Error("Bulk Update not Finished")
	}
	return true
}

func (es *ESClient) DeleteUnconfirmTx(TxId string) bool {
	res, err := es.client.Delete().
		Index("eos-tranx-gate-unconfirm").
		Type("_doc").
		Id(TxId).
		Do(es.ctx)
	if err != nil {
		log.Error("Delete Tx", "err", err)
		return false
	}
	if res.Status == 404 {
		log.Error("Delete tx", "err", "NOT FOUND")
		return true
	}
	return true
}
