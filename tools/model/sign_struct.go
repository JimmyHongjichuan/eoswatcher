package model

type SignInput struct {
	InputHex   string `json:"inputHex"`
	PubHashHex string `json:"pubHashHex"`
	TimeStamp  int64  `json:"timestamp"`
	ServiceID  string `json:"serviceId"`
	Type       int    `json:"type"`
}

//SignResponse 秘钥服务签名返回结构
type SignResponse struct {
	Code int       `json:"code"`
	Data *SignData `json:"data"`
	Msg  string    `json:"msg"`
}

//SignData 签数结果
type SignData struct {
	R                string `json:"r"`
	S                string `json:"s"`
	SignatureDerHex  string `json:"signatureDerHex"`
	RecID            int    `json:"recId"`
	SignatureCompact string `json:"signatureCompactHex"`
}

//PubkeyData 秘钥服务公钥结果
type PubkeyData struct {
	PubHashHex string `json:"pubHashHex"`
	PubHex     string `json:"pubHex"`
}

//GenerateData 秘钥服务生成公钥接口数据
type GenerateData struct {
	Keys []*PubkeyData `json:"keys"`
}

type GenerateInput struct {
	Count     int    `json:"count"`
	ServiceID string `json:"serviceId"`
	TimeStamp int64  `json:"timestamp"`
}

//GenerateResponse 秘钥服务生成公钥接口返回结构
type GenerateResponse struct {
	Code int `json:"code"`
	Data *GenerateData
	Msg  string `json:"msg"`
}
