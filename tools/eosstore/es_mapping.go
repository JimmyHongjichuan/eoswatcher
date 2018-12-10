package eosstore

//更改ref_block_num 和block_num为类型为long
const TxMapping = `
{
	"settings":{
		"number_of_shards": 6,
		"number_of_replicas": 0
	},
	"mappings": {
		"_doc": {
			"properties": {
				"block_num": {
					"type": "long"
				},
				"block_id": {
					"type": "keyword"
				},
				"block_timestamp": {
					"type": "date"
				},
				"producer": {
					"type": "keyword"
				},
				"tx_id": {
					"type": "keyword"
				},
				"status": {
					"type": "keyword"
				},
				"expiration": {
					"type": "date"
				},
				"ref_block_num": {
					"type": "integer"
				},
				"ref_block_prefix": {
					"type": "long"
				},
				"delay_sec": {
					"type": "integer"
				},
				"account": {
					"type": "keyword"
				},
				"name":{
					"type": "keyword"
				},
				"from": {
					"type": "keyword"
				},
				"to": {
					"type": "keyword"
				},
				"quantity": {
					"type": "text"
				},
				"amount": {
					"type": "long"
				},
				"memo": {
					"type": "text"
				},
				"precision": {
					"type": "byte"
				},
				"symbol": {
					"type": "keyword"
				}
			}
		}
	}
}`
