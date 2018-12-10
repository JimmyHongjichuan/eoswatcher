package main

import (
	"eosc/tools/multisig"
	"fmt"
	"os"

	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("conf") //设置配置文件名(不带后缀)
	viper.AddConfigPath("./tools/multisig")
	err := viper.ReadInConfig() //搜索路径，读取配置
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	//generateEOSPubkey()
	//singleSignTx()
	//multiSignTx()

	//去私钥服务器多签名
	multiSignTxPKM()

	//验证签名
	//multisig.VerifySig()

}

func singleSignTx() {
	tx, err := multisig.BuildTokenTransferTx("zxj", "zxj111222333", "singlesigntx", 1)
	if err != nil {
		log.Error("err", err)

	}
	sign1, err := multisig.LocalSign(tx, "5JETpUNzkE7MxYHg4KR2ABYeBK3iqDoVymhCR3Cc2VAaNMb8CS9")
	if err != nil {
		log.Error("err", err)

	}
	packedTx, err := multisig.MergeSignedTx(tx, sign1)
	if err != nil {
		log.Error("err", err)

	}
	txResult, err := multisig.SendTx(packedTx)
	if err != nil {
		log.Error("err", err)

	}
	fmt.Printf("txResult %+v", txResult)
}

func multiSignTx() {
	tx, err := multisig.BuildTokenTransferTx("multisigkeys", "zxj", "multiSignTest", 1000)
	if err != nil {
		log.Error("err", err)

	}
	sign1, err := multisig.LocalSign(tx, "5JW9TAHTW7oT5x7bUWdANf7QNjvWipnGaC9FXF1gst6LEF88KWm")
	if err != nil {
		log.Error("err", err)

	}

	sign2, err := multisig.LocalSign(tx, "5HtJZn1SsHBGtvNhsA8QV3oCAyYxzrjncWiF9i17JJ4sGGXAZYt")
	if err != nil {
		log.Error("err", err)

	}
	sign3, err := multisig.LocalSign(tx, "5JJJThbBrCC8JH1VYTNa3KcvLcwrwMmqSzvNTf5fBuyFe6c4UM2")
	if err != nil {
		log.Error("err", err)

	}
	packedTx, err := multisig.MergeSignedTx(tx, sign1, sign2, sign3)
	if err != nil {
		log.Error("err", err)

	}
	txResult, err := multisig.SendTx(packedTx)
	if err != nil {
		log.Error("err", err)

	}
	fmt.Printf("txResult %+v", txResult)
}

func multiSignTxPKM() {
	//生成一笔简单的EOS转账交易
	tx, err := multisig.BuildTokenTransferTx("multisigpkm2", "kylineoshjx1", "multiSignVerifyTest3", 100)
	if err != nil {
		log.Error("err", err)
	}

	//生成第一个交易签名
	sign1, err := multisig.PKMSign(tx, "10D17CF7247347164CD1F87B2EB406A8260D1AB4")
	if err != nil {
		log.Error("err", err)
	}
	//解析签名 对应的公钥，并打印
	pubkey, err := multisig.GetPublickeyFromTx(tx, sign1)
	if err != nil {
		fmt.Println("err: ", err)
	}
	fmt.Println("public key: ", pubkey.String())

	//生成第二个交易签名
	sign2, err := multisig.PKMSign(tx, "F77985086F02BB2A14852C0B3862CFBA1704EDD8")
	if err != nil {
		log.Error("err", err)
	}
	//解析签名 对应的公钥，并打印
	pubkey2, err := multisig.GetPublickeyFromTx(tx, sign2)
	if err != nil {
		fmt.Println("err: ", err)
	}
	fmt.Println("pubkey2 key: ", pubkey2.String())

	//生成第三个交易签名
	sign3, err := multisig.PKMSign(tx, "EA86238F370A0EB9278A21295E5AED1A463AC3BA")
	if err != nil {
		log.Error("err", err)
	}
	//解析签名 对应的公钥，并打印
	pubkey3, err := multisig.GetPublickeyFromTx(tx, sign3)
	if err != nil {
		fmt.Println("err: ", err)
	}
	fmt.Println("pubkey3 key: ", pubkey3.String())

	//生成第四个交易签名
	sign4, err := multisig.PKMSign(tx, "C8877608C003AB111690027E7EA0226F756B5F4A")
	if err != nil {
		log.Error("err", err)
	}
	//解析签名 对应的公钥，并打印
	pubkey4, err := multisig.GetPublickeyFromTx(tx, sign4)
	if err != nil {
		fmt.Println("err: ", err)
	}
	fmt.Println("pubkey4 key: ", pubkey4.String())


	//把所有的签名 和交易，merge到一起
	packedTx, err := multisig.MergeSignedTx(tx, sign1, sign2, sign4)
	if err != nil {
		log.Error("multisig.MergeSignedTx error", err)
		os.Exit(1)
	}

	//发送交易
	txResult, err := multisig.SendTx(packedTx)
	if err != nil {
		log.Error("multisig.SendTx error", err)
		os.Exit(1)
	}
	fmt.Println("TxID: ", txResult.TransactionID)


	//再发送一笔相同交易体、不同签名的交易
	packedTx2, err := multisig.MergeSignedTx(tx, sign1, sign2, sign3)
	if err != nil {
		log.Error("multisig.MergeSignedTx error", err)
		os.Exit(1)
	}
	txResult2, err := multisig.SendTx(packedTx2)
	if err != nil {
		log.Error("multisig.SendTx error", err)
		os.Exit(1)
	}
	fmt.Println("TxID: ", txResult2.TransactionID)
}

func generateEOSPubkey() {
	pubHash := []string{
		"0447555D04E2FE248DAA28D5E1C17C60E6E530DA8D59B0981DE04F30E323F8D24A1C41C318581327ED6095B9305944ECA994BB113C0F6E447888CD9E7102E8E6E4",
		"04095C3C82C6ED0A9A0288457A26938C249B832D8306FF7F9EC20C55DBAE07614E62273ECB0C774BB0885EC60CE1EB066C5880D940187A908D37DB6E6186DAB029",
		"04A2339FECF1887CBAEDCA8D5C3E69D4164B064BDEAFD9CF574554184F8727726696835BA771748E3D9940C5443D2BF9EE2430D47C71945711392BDF16500FF20F",
		"049A774EFF9F2A94D82E071772810DE9FDA18720D44A684001DEBF2BD801EC76CB87E90D8A4D2E660CBB513DC353621631FCDED82D2171BBCD08A3B58228C8A30D",
	}
	for _, v := range pubHash {
		res, _ := multisig.NewPublicKey(v)
		fmt.Println(res)
	}
}
