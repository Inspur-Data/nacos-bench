## Introduction  
This project provides a performance testing tool for Nacos core interfaces. It supports performance stress testing for Nacos registry services (service registration/deregistration, service query) and Nacos configuration center interfaces (configuration publishing, configuration query).  

## Usage Instructions  
### Startup Parameters Description  
| Parameter | Description | Default Value |  
|-----------|-------------|---------------|  
| --configContentLength | Configuration content length | 128 bytes |  
| --configCount | Number of configurations | 1000 |  
| --instanceCountPerService | Number of service providers registered per service | 3 |  
| --nacosClientCount | Number of simulated Nacos clients per process | 1000 |  
| --nacosServerAddr | Nacos server address | 127.0.0.1 |  
| --namingMetadataLength | Metadata size of service providers | 128 bytes |  
| --perfApi | Interface to be tested, options: namingReg, namingQuery, namingSubscribe, configPub, configGet | namingReg |  
| --perfMode | Testing mode, options: naming, config | naming |  
| --perfTime | Testing duration, in seconds | 600 |  
| --perfTps | Testing TPS/QPS | 500 |  
| --serviceCount | Number of simulated service names | 15000 |  

### Usage Examples  
1. Service Registration  
```  
./nacos-bench --nacosServerAddr=127.0.0.1 --perfMode=naming --perfApi=namingReg --perfTps=50 --perfTime=900 --nacosClientCount=100 --serviceCount=10000 --namingMetadataLength=64  
```  
2. Service Query  
```  
./nacos-bench --nacosServerAddr=127.0.0.1 --perfMode=naming --perfApi=namingQuery --perfTps=50 --perfTime=900 --nacosClientCount=100 --serviceCount=10000 --namingMetadataLength=64  
```  
3. Configuration Publishing  
```  
./nacos-bench --nacosServerAddr=127.0.0.1 --perfMode=config --perfApi=configPub --perfTps=5 --perfTime=900 --nacosClientCount=100 --configContentLength=64 --configCount=500  
```  
4. Configuration Query  
```  
./nacos-bench --nacosServerAddr=127.0.0.1 --perfMode=config --perfApi=configGet --perfTps=100 --perfTime=900 --nacosClientCount=100 --configContentLength=64 --configCount=500  
```
