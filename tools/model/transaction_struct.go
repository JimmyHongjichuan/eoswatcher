package model

type TransferData struct {
	BlockNum       uint32 `json:"block_num"`
	BlockID        string `json:"block_id"`
	BlockTimeStamp int64  `json:"block_timestamp"`
	Producer       string `json:"producer"`
	TxID           string `json:"tx_id"`
	Status         string `json:"status"`
	//CpuUsageUs uint32
	//NetUsageWords eos.Varuint32
	Expiration     int64  `json:"expiration"`
	RefBlockNum    uint16 `json:"ref_block_num"`
	RefBlockPrefix uint32 `json:"ref_block_prefix"`
	DelaySec       uint32 `json:"delay_sec"`
	Account        string `json:"account"`
	Name           string `json:"name"`
	From           string `json:"from"`
	To             string `json:"to"`
	Quantity       string `json:"quantity"`
	Amount         int64  `json:"amount"`
	Symbol         string `json:"symbol"`
	Precision      uint8  `json:"precision"`
	Memo           string `json:"memo"`
}

type Transfer struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Quantity string `json:"quantity"`
	Memo     string `json:"memo"`
}

type PushMessage struct {
	//TxID     uint8           `json:"tx_id"`
	TxID      string `json:"tx_id"`
	Status    int8   `json:"status"`
	BlockNum  uint32 `json:"block_num"`
	From      string `json:"from"`
	To        string `json:"to"`
	TimeStamp int64  `json:"timestamp"`
	Value     string `json:"value"`
}
