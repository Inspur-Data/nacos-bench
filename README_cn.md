## 简介
本工程提供了一个Nacos核心接口的性能测试工具，支持对Nacos注册中心服务注册/注销、服务查询，Nacos配置中心配置发布、配置查询接口进行性能压测。

## 使用方法
### 启动参数说明
|参数|说明|默认值|
|----|----|----|
|--configContentLength|配置内容长度|128字节|
|--configCount|配置个数|1000|
|--instanceCountPerService|每个服务注册的服务提供者数|3|
|--nacosClientCount|每个进程模拟的nacos client数|1000|
|--nacosServerAddr|nacos服务端地址|127.0.0.1|
|--namingMetadataLength|服务提供者metadata数据大小|128字节|
|--perfApi|需要压测的接口，可选值为： namingReg、namingQuery、namingSubscribe、configPub、configGet|namingReg|
|--perfMode|压测模式，可选为：naming、config|naming|
|--perfTime|压测时间，单位为秒|600|
|--perfTps|压测TPS/QPS|500|
|--serviceCount|模拟注册服务名数量|15000|
### 使用示例
1. 服务注册
```
./nacos-bench --nacosServerAddr=127.0.0.1 --nacosPort=8848 --perfMode=naming --perfApi=namingReg --perfTps=50 --perfTime=900 --nacosClientCount=100 --serviceCount=10000 --namingMetadataLength=64 --username=nacos --password=SW5zcHVyTmFjb3MxIQ==
```
2. 服务查询
```
./nacos-bench --nacosServerAddr=127.0.0.1 --nacosPort=8848 --perfMode=naming --perfApi=namingQuery --perfTps=50 --perfTime=900 --nacosClientCount=100 --serviceCount=10000 --namingMetadataLength=64 --username=nacos --password=SW5zcHVyTmFjb3MxIQ==
```
3. 配置发布
```
./nacos-bench --nacosServerAddr=127.0.0.1 --nacosPort=8848 --perfMode=config --perfApi=configPub --perfTps=5 --perfTime=900 --nacosClientCount=100 --configContentLength=64 --configCount=500  --username=nacos --password=SW5zcHVyTmFjb3MxIQ==
```
4. 配置查询
```
./nacos-bench --nacosServerAddr=127.0.0.1 --nacosPort=8848 --perfMode=config --perfApi=configGet --perfTps=100 --perfTime=900 --nacosClientCount=100 --configContentLength=64 --configCount=500 --username=nacos --password=SW5zcHVyTmFjb3MxIQ==
```