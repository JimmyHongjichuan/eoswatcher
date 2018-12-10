package eosmanager

import (
	"bytes"
	"encoding/json"
	"eosc/tools/model"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/eoscanada/eos-go"
	log "github.com/inconshreveable/log15"
)

type EosClient struct {
	HttpClient *http.Client
	EosAPI     *eos.API
	BaseURL    string
	ChainID    []byte
}

func NewEosClient(baseURL string) *EosClient {
	api := eos.New(baseURL)
	newEosClient := &EosClient{
		HttpClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableKeepAlives:     true, // default behavior, because of `nodeos`'s lack of support for Keep alives.
			},
		},
		EosAPI:  api,
		BaseURL: baseURL,
	}
	return newEosClient
}

func (ec *EosClient) SetChainID() {
	resp, err := ec.GetInfo()
	if err != nil {
		log.Error("SetChainID Fail", "err", err)
		return
	}
	ec.ChainID = resp.ChainID
}

func (ec *EosClient) call(baseAPI string, endpoint string, body interface{}, out interface{}) error {
	jsonBody, err := enc(body)
	if err != nil {
		return err
	}

	targetURL := fmt.Sprintf("%s/v1/%s/%s", ec.BaseURL, baseAPI, endpoint)
	req, err := http.NewRequest("POST", targetURL, jsonBody)
	if err != nil {
		return fmt.Errorf("NewRequest: %s", err)
	}

	resp, err := ec.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s", req.URL.String(), err)
	}
	defer resp.Body.Close()
	var cnt bytes.Buffer
	_, err = io.Copy(&cnt, resp.Body)

	if err != nil {
		return fmt.Errorf("Copy: %s", err)
	}

	if resp.StatusCode == 404 {
		return ErrNotFound
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("%s: status code=%d, body=%s", req.URL.String(), resp.StatusCode, cnt.String())
	}
	if err := json.Unmarshal(cnt.Bytes(), &out); err != nil {
		return fmt.Errorf("Unmarshal: %s", err)
	}
	return nil
}

var ErrNotFound = errors.New("resource not found")

func enc(v interface{}) (io.Reader, error) {
	if v == nil {
		return nil, nil
	}

	cnt, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(cnt), nil
}

func (ec *EosClient) GetInfo() (out *eos.InfoResp, err error) {
	err = ec.call("chain", "get_info", nil, &out)
	if err != nil {
		log.Error("API_GETINFO", "error:", err)
		return nil, err
	}
	return
}

func (ec *EosClient) GetIrreversibleBlockHeight() uint32 {
	blockInfo, err := ec.GetInfo()
	if err != nil {
		return 0
	}
	return blockInfo.LastIrreversibleBlockNum
}

func (ec *EosClient) GetNewestBlockHeight() uint32 {
	blockInfo, err := ec.GetInfo()
	if err != nil {
		return 0
	}
	return blockInfo.HeadBlockNum
}

func (ec *EosClient) PushSignedTransaction(tx *eos.PackedTransaction) (out *eos.PushTransactionFullResp, err error) {
	err = ec.call("chain", "push_transaction", tx, &out)
	if err != nil {
		log.Error("API_PUSHSIGNEDTRANSACTION", "error:", err)
		return nil, err
	}
	return
}

func (ec *EosClient) GetTransaction(id string) (out interface{}, err error) {
	err = ec.call("history", "get_transaction", map[string]interface{}{"transaction_id": id}, &out)
	if err != nil {
		log.Error("API_GETTRANSACTION", "error:", err)
		return nil, err
	}
	return
}

func (ec *EosClient) GetTransactionFormat(id string) (out *eos.TransactionResp, err error) {
	return ec.EosAPI.GetTransaction(id)
}

func (ec *EosClient) GetBlockByID(query uint32) (resp *model.BlockResp, err error) {
	out := model.BlockResp{}
	err = ec.call("chain", "get_block", map[string]interface{}{"block_num_or_id": query}, &out)
	if err != nil {
		log.Error("API_GETBLOCKBYID", "error:", err)
		return nil, err
	}
	resp, err1 := ec.BlockRespDecodeTrx(&out)
	if err1 != nil {
		return nil, err
	}
	return
}

func (ec *EosClient) BlockRespDecodeTrx(blockResp *model.BlockResp) (out *model.BlockResp, err error) {
	var tempTransactions []model.Transaction
	for _, transaction := range blockResp.Transactions {
		if len(transaction.RawTrx) == 66 {
			//如果交易收据是rxid类型，需要再请求一次交易
			txid := string(transaction.RawTrx[1:65])
			log.Debug("Txid in block", "Txid", txid)

			//res2 , err2 := ec.GetTransaction(txid)
			//if err2 != nil {
			//	log.Error("Get transaction", "Txid", txid)
			//	continue
			//}
			//fmt.Println(res2)

			resTransaction, err := ec.GetTransactionFormat(txid)
			if err != nil {
				log.Error("Get transaction", "Txid", txid)
				continue
			}
			//temp := resTransaction.Traces[0].Action.ActionData.Data
			//
			//v2, ok := temp.(map[string]interface{})
			//if ok {
			//	fmt.Println(v2)
			//}
			//
			//a := fmt.Sprintf("%v", temp)
			//fmt.Println(a)

			trx := model.Trx{}
			trx.ID = txid
			trx.TransactionInfo.Expiration = model.JSONTime{}
			trx.TransactionInfo.RefBlockNum = 0
			trx.TransactionInfo.RefBlockPrefix = 0
			trx.TransactionInfo.DelaySec = 0
			action := model.Action{}
			action.Account = string(resTransaction.Traces[0].Action.Account)
			action.Name = string(resTransaction.Traces[0].Action.Name)
			action.Data = resTransaction.Traces[0].Action.ActionData.Data //有待商榷
			actions := []model.Action{action}
			trx.TransactionInfo.Actions = actions
			transaction.Trx = trx
			tempTransactions = append(tempTransactions, transaction)

			//fmt.Println(resTransaction, temp)
		} else {
			trx := model.Trx{}
			err = json.Unmarshal(transaction.RawTrx, &trx)
			if err != nil {
				/*  忽略了不能decode的交易
					"transactions": [{
					  "status": "executed",
					  "cpu_usage_us": 1536,
					  "net_usage_words": 0,
					  "trx": "6a19363e60307fb734be969e5bf9e9f1707f4de1adacc24430fa147bbec242f3"
					}
				  ]*/
				log.Info("Decode blockRespTrx", "err", err, "msg", "ignore transaction can't decode")
				continue
			}
			transaction.Trx = trx
			tempTransactions = append(tempTransactions, transaction)
		}
	}
	blockResp.Transactions = tempTransactions
	out = blockResp
	return out, nil
}

func (ec *EosClient) GetCurrencyBalance(account string, symbol string) (out []eos.Asset, err error) {
	params := map[string]string{"code": "eosio.token", "account": account}
	if symbol != "" {
		params["symbol"] = symbol
	}
	err = ec.call("chain", "get_currency_balance", map[string]string{"code": "eosio.token", "account": account, "symbol": symbol}, &out)
	if err != nil {
		log.Error("GET_CURRENCY", "error:", err)
		return nil, err
	}
	fmt.Println("0000-->", out)
	return
}
