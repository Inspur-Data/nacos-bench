package internal

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"golang.org/x/time/rate"
)

var configClients []config_client.IConfigClient
var dataIds []string

func InitConfig(perfConfig PerfConfig) {
	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "info",
		Username:             perfConfig.Username,
		Password:             perfConfig.Password,
	}

	serverConfigs := make([]constant.ServerConfig, 0)
	for _, addr := range strings.Split(perfConfig.NacosAddr, ",") {
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr:      addr,
			ContextPath: "/nacos",
			Port:        perfConfig.NacosPort,
		})
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < perfConfig.ClientCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer mu.Unlock()
			client, _ := clients.NewConfigClient(vo.NacosClientParam{
				ClientConfig:  &clientConfig,
				ServerConfigs: serverConfigs,
			})
			mu.Lock()
			configClients = append(configClients, client)
		}()
	}
	wg.Wait()

	configClient1 := configClients[0]
	content, err := configClient1.GetConfig(vo.ConfigParam{
		DataId: "nacos.config.perf.test.dataId." + strconv.Itoa(perfConfig.ConfigCount-1),
		Group:  "DEFAULT_GROUP",
	})

	needInitConfig := true
	if err == nil && content != "" {
		fmt.Println("config existed, will not init")
		needInitConfig = false
	}

	for i := 0; i < perfConfig.ConfigCount; i++ {
		dataId := "nacos.config.perf.test.dataId." + strconv.Itoa(i)
		dataIds = append(dataIds, dataId)
		if needInitConfig {
			res, err := configClient1.GetConfig(vo.ConfigParam{
				DataId: "nacos.config.perf.test.dataId." + strconv.Itoa(perfConfig.ConfigCount-1),
				Group:  "DEFAULT_GROUP",
			})
			if err == nil && res != "" {
				needInitConfig = false
				fmt.Println("config existed, will not init")
				continue
			}

			configClient1.PublishConfig(vo.ConfigParam{
				DataId:  dataId,
				Group:   "DEFAULT_GROUP",
				Content: generateRandomString(perfConfig.ConfigContentLength),
			})

		}

	}

	if perfConfig.PerfApi == "configSubscribe" {
		for _, client := range configClients {
			dataId := dataIds[0]
			client.ListenConfig(vo.ConfigParam{
				DataId: dataId,
				Group:  "DEFAULT_GROUP",
				OnChange: func(namespace, group, dataId, data string) {
				},
			})
		}
	}
}

func publicConfig(client config_client.IConfigClient, limiter *rate.Limiter, dataId string, dataLength int) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	client.PublishConfig(vo.ConfigParam{
		DataId:  dataId,
		Group:   "DEFAULT_GROUP",
		Content: generateRandomString(dataLength),
	})
}

func getConfig(client config_client.IConfigClient, limiter *rate.Limiter, dataId string) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	client.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  "DEFAULT_GROUP",
	})
}
func RunConfigPerf(perfConfig PerfConfig) {
	startTime := time.Now().UnixMilli()
	configCount := perfConfig.ConfigCount
	switch perfConfig.PerfApi {
	case "configPub":
		pubLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigPubTps), 1)
		for i := 0; i < perfConfig.ClientCount; i++ {
			go func() {
				for {
					if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
						return
					}
					publicConfig(configClients[i], pubLimiter, dataIds[rand.Intn(configCount)], perfConfig.ConfigContentLength)
				}
			}()
		}
	case "configGet":
		getConfigLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigGetTps), 1)
		for i := 0; i < perfConfig.ClientCount; i++ {
			go func() {
				for {
					if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
						return
					}
					getConfig(configClients[i], getConfigLimiter, dataIds[rand.Intn(configCount)])
				}
			}()
		}
	case "configSubscribe":
		//pubLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigPubTps), 1)
		//for i := 0; i < perfConfig.ClientCount; i++ {
		//	go func() {
		//		for {
		//			if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
		//				return
		//			}
		//			publicConfig(configClients[i], pubLimiter, dataIds[rand.Intn(configCount)], perfConfig.ConfigContentLength)
		//		}
		//	}()
		//}
	default:
		panic("unknown perf api")
	}

}
