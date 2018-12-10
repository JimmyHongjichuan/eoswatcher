## api endpoint
    eos: jungle endpoint: http://jungle.eosbcn.com:8080
    keystore: http://47.97.167.221:8976

## 5 keypairs in jungletestnet:

    Private key: 5JW9TAHTW7oT5x7bUWdANf7QNjvWipnGaC9FXF1gst6LEF88KWm
    Public key: EOS7aiVFASATGuZiEbY5gPFV9JuZqd4uzhfdBVWFQPGW9AdftHhPL
    account:dgwnode11111

    Private key: 5KTPQr6fwiREETnufaGdPUZtmai5qE32anPtun9UrxnAByo4X5A
    Public key: EOS4zt2DTL5wnM3b9a1BAbbBEoFWjgP2UZHfjkUHhezYNM2A7K149
    account:dgwnode22222

    Private key: 5JJJThbBrCC8JH1VYTNa3KcvLcwrwMmqSzvNTf5fBuyFe6c4UM2
    Public key: EOS7pJfSfr9VmuLuGgccYEMGgmtPw88PZX7fkUCu9GanQgfmU1D6G
    account: dgwnode33333


    Private key: 5JmrZkTKJVxqGEVJqsn6AbX6W4Q7ZLje3rZiGdeSt7d3jttscPr
    Public key: EOS8JZvVEDjkVARoBsxugLn7jx7X5jmxyenvmZUVwo3ccsTLwzoZb
    account: dgwnode44444
            multisigkeys
            multisigperm

    Private key: 5HtJZn1SsHBGtvNhsA8QV3oCAyYxzrjncWiF9i17JJ4sGGXAZYt
    Public key: EOS5YuivmK5QiyCYttAivdV8CdrBEoTcMYfXFEPo5YoHd34N23jEa
    
    设置多签账户:
        cleos  -u http://dev.cryptolions.io:38888 set account permission multisigkeys owner '{
            "threshold":3,
            "keys":[
                {"key":"EOS7aiVFASATGuZiEbY5gPFV9JuZqd4uzhfdBVWFQPGW9AdftHhPL", "weight":"1"},{"key":"EOS5YuivmK5QiyCYttAivdV8CdrBEoTcMYfXFEPo5YoHd34N23jEa", "weight":"1"},{"key":"EOS7pJfSfr9VmuLuGgccYEMGgmtPw88PZX7fkUCu9GanQgfmU1D6G", "weight":"1"},{"key":"EOS8JZvVEDjkVARoBsxugLn7jx7X5jmxyenvmZUVwo3ccsTLwzoZb", "weight":"1"}]}' -p nultisigkeys@owner 


        multisigkeys:
        cleos -u http://dev.cryptolions.io:38888 set account permission multisigkeys active '{"threshold" : 3, "keys" : [{"key": "EOS5YuivmK5QiyCYttAivdV8CdrBEoTcMYfXFEPo5YoHd34N23jEa","weight": "1"},{"key": "EOS7aiVFASATGuZiEbY5gPFV9JuZqd4uzhfdBVWFQPGW9AdftHhPL","weight": "1"},{"key": "EOS7pJfSfr9VmuLuGgccYEMGgmtPw88PZX7fkUCu9GanQgfmU1D6G","weight": "1"},{"key": "EOS8JZvVEDjkVARoBsxugLn7jx7X5jmxyenvmZUVwo3ccsTLwzoZb","weight": "1"}], "accounts" : []}' owner

        cleos -u http://dev.cryptolions.io:38888 set account permission multisigkeys owner '{"threshold" : 3, "keys" : [{"key": "EOS5YuivmK5QiyCYttAivdV8CdrBEoTcMYfXFEPo5YoHd34N23jEa","weight": "1"},{"key": "EOS7aiVFASATGuZiEbY5gPFV9JuZqd4uzhfdBVWFQPGW9AdftHhPL","weight": "1"},{"key": "EOS7pJfSfr9VmuLuGgccYEMGgmtPw88PZX7fkUCu9GanQgfmU1D6G","weight": "1"},{"key": "EOS8JZvVEDjkVARoBsxugLn7jx7X5jmxyenvmZUVwo3ccsTLwzoZb","weight": "1"}], "accounts" : []}' -p multisigkeys@owner


        multisigperm:
        cleos -u http://dev.cryptolions.io:38888 set account permission multisigperm active '{"threshold" : 3, "keys" : [], "accounts" : [{"permission":{"actor":"dgwnode11111","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode22222","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode33333","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode44444","permission":"active"},"weight":1}]}' owner

        cleos -u http://dev.cryptolions.io:38888 set account permission multisigperm owner '{"threshold" : 3, "keys" : [], "accounts" : [{"permission":{"actor":"dgwnode11111","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode22222","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode33333","permission":"active"},"weight":1},{"permission":{"actor":"dgwnode44444","permission":"active"},"weight":1}]}' -p multisigperm@owner


## keystore
    service_id:
        {
        "code": 0,
        "msg": "",
        "data": {
            "serviceId": "cda298df-4c48-421f-b337-04c0ce245965",
            "privateKey": "A549FE4EF86F1B086FDB1A26D27DBE79B3DBA6B74FC5EAD39A2258C7E2FC6C42"
        }
        }

    4key pairs:
        {
            PubHashHex:10D17CF7247347164CD1F87B2EB406A8260D1AB4 PubHex:0447555D04E2FE248DAA28D5E1C17C60E6E530DA8D59B0981DE04F30E323F8D24A1C41C318581327ED6095B9305944ECA994BB113C0F6E447888CD9E7102E8E6E4
            }
        {
            PubHashHex:F77985086F02BB2A14852C0B3862CFBA1704EDD8 PubHex:04095C3C82C6ED0A9A0288457A26938C249B832D8306FF7F9EC20C55DBAE07614E62273ECB0C774BB0885EC60CE1EB066C5880D940187A908D37DB6E6186DAB029
            }
        {
            PubHashHex:C8877608C003AB111690027E7EA0226F756B5F4A PubHex:04A2339FECF1887CBAEDCA8D5C3E69D4164B064BDEAFD9CF574554184F8727726696835BA771748E3D9940C5443D2BF9EE2430D47C71945711392BDF16500FF20F
            }
        {
            PubHashHex:EA86238F370A0EB9278A21295E5AED1A463AC3BA PubHex:049A774EFF9F2A94D82E071772810DE9FDA18720D44A684001DEBF2BD801EC76CB87E90D8A4D2E660CBB513DC353621631FCDED82D2171BBCD08A3B58228C8A30D
            }
        对应四组公钥:
            EOS5RuUGM9pA4RZLkBRhJR4BUsszwBpc7zd3FRn8ZWiPDXDzRGdBX
            EOS6uMdHA9VJRHVqkCwvMf5PAixo3BtomA4fLtd8n6DHFZMbawv4L
            EOS84fkfp9JDczMZtxBUeq9vyn5sKpokytvyETpjpitNQ389CZnrp
            EOS81GA7e1orZzrKpxyDqMgC9bFAVYvdPWDqJJsZNnP5yYD3ZwYwA

        ##不知道这一组为啥不行
        cleos  set account permission multisigpkm1 active '{"threshold" : 3, "keys" : [{"key": "EOS5RuUGM9pA4RZLkBRhJR4BUsszwBpc7zd3FRn8ZWiPDXDzRGdBX","weight": "1"},{"key": "EOS6uMdHA9VJRHVqkCwvMf5PAixo3BtomA4fLtd8n6DHFZMbawv4L","weight": "1"},{"key": "EOS84fkfp9JDczMZtxBUeq9vyn5sKpokytvyETpjpitNQ389CZnrp","weight": "1"},{"key": "EOS81GA7e1orZzrKpxyDqMgC9bFAVYvdPWDqJJsZNnP5yYD3ZwYwA","weight": "1"}], "accounts" : []}' owner


        cleos set account permission multisigpkm1 active '{"threshold" : 3, "keys" : [{"key": "EOS5RuUGM9pA4RZLkBRhJR4BUsszwBpc7zd3FRn8ZWiPDXDzRGdBX","weight": "1"},{"key": "EOS6uMdHA9VJRHVqkCwvMf5PAixo3BtomA4fLtd8n6DHFZMbawv4L","weight": "1"},{"key": "EOS81GA7e1orZzrKpxyDqMgC9bFAVYvdPWDqJJsZNnP5yYD3ZwYwA","weight": "1"}], "accounts" : []}' owner


## target
    1.build tx
    2.sign
    3.merge sign
    4.send rawtransaction
    5.updateauth

