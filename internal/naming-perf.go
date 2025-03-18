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
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client/naming_cache"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client/naming_grpc"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/http_agent"
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/common/nacos_server"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"golang.org/x/time/rate"
)

var namingClientSlice []*NacosClient

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var serviceNames []string
var mutex sync.Mutex

type NacosClient struct {
	namingClient naming_client.INamingClient
	grpcProxy    *naming_grpc.NamingGrpcProxy
	serviceNames []string
	cancel       context.CancelFunc
}

func safaAppend(client *NacosClient) {
	mutex.Lock()
	defer mutex.Unlock()
	namingClientSlice = append(namingClientSlice, client)
}
func initClient(clientConfig constant.ClientConfig, serverConfigs []constant.ServerConfig, perfConfig PerfConfig, wg *sync.WaitGroup) {
	client, _ := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})

	serviceInfoHolder := naming_cache.NewServiceInfoHolder(clientConfig.NamespaceId, clientConfig.CacheDir,
		clientConfig.UpdateCacheWhenEmpty, clientConfig.NotLoadCacheAtStart)
	ctx, cancel := context.WithCancel(context.Background())

	nacosServer, err := nacos_server.NewNacosServer(ctx, serverConfigs, clientConfig, &http_agent.HttpAgent{}, clientConfig.TimeoutMs, clientConfig.Endpoint)

	if err != nil {
		panic(err)
	}

	var grpcClientProxy *naming_grpc.NamingGrpcProxy
	if perfConfig.PerfApi == "namingQuery" {
		grpcClientProxy, err = naming_grpc.NewNamingGrpcProxy(ctx, clientConfig, nacosServer, serviceInfoHolder)

		if err != nil {
			panic(err)
		}
	}

	nacosClient := NacosClient{
		namingClient: client,
		serviceNames: make([]string, 0),
		grpcProxy:    grpcClientProxy,
		cancel:       cancel,
	}

	for i := 0; i < perfConfig.InstanceCountPerService; i++ {
		svc := serviceNames[rand.Intn(len(serviceNames))]
		nacosClient.serviceNames = append(nacosClient.serviceNames, svc)
		serviceNames = append(serviceNames, svc)
	}

	safaAppend(&nacosClient)
	wg.Done()
}
func InitNaming(perfConfig PerfConfig) {
	serviceNames = make([]string, 0)
	serviceNames = generateServiceNames(perfConfig.ServiceCount)

	clientConfig := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "info",
	}

	serverConfigs := make([]constant.ServerConfig, 0)
	for _, addr := range strings.Split(perfConfig.NacosAddr, ",") {
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr:      addr,
			ContextPath: "/nacos",
			Port:        8848,
		})
	}
	var wg sync.WaitGroup
	for i := 0; i < perfConfig.ClientCount; i++ {
		wg.Add(1)
		go initClient(clientConfig, serverConfigs, perfConfig, &wg)
	}
	wg.Wait()
	fmt.Println("init client success")

	for _, client := range namingClientSlice {
		for _, serviceName := range client.serviceNames {
			registerInstance(client.namingClient, "127.0.0.1", serviceName, 8080, 1, true, true, perfConfig.NamingMetadataLength)
		}
	}

	if perfConfig.PerfApi == "namingSubscribe" {
		for _, client := range namingClientSlice {
			for i := 0; i < 3*perfConfig.InstanceCountPerService; i++ {
				client.namingClient.Subscribe(&vo.SubscribeParam{
					ServiceName:       serviceNames[rand.Intn(len(serviceNames))],
					GroupName:         "DEFAULT_GROUP",
					SubscribeCallback: func(services []model.Instance, err error) {},
				})
			}
		}
	}
}

// 生成随机字符串
func generateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano()) // 使用当前时间作为随机数种子
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func registerInstance(namingClient naming_client.INamingClient, ip string, serviceName string, port int, weight float64, enabled bool, healthy bool, metadataLength int) {
	namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: serviceName,
		Weight:      weight,
		Enable:      enabled,
		Healthy:     healthy,
		Metadata: map[string]string{
			"key": generateRandomString(metadataLength),
		},
		Ephemeral: true,
	})
}

func queryInstance(proxy *naming_grpc.NamingGrpcProxy, serviceName string) {
	_, err := proxy.QueryInstancesOfService(serviceName, "DEFAULT_GROUP", "", 0, false)
	if err != nil {
		logger.Warn("queryInstance failed, caused: " + err.Error())
	}
}

func deregisterInstance(namingClient naming_client.INamingClient, ip string, serviceName string, port int) {

	namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: serviceName,
		Ephemeral:   true,
	})
}

func generateServiceNames(serviceCount int) []string {
	result := make([]string, 0)
	for i := 0; i < serviceCount; i++ {
		result = append(result, "nacos.push.perf."+strconv.Itoa(i))
	}

	return result
}

func regAndDereg(client *NacosClient, limiter *rate.Limiter, config PerfConfig) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	svc := client.serviceNames[rand.Intn(config.InstanceCountPerService)]
	deregisterInstance(client.namingClient, "127.0.0.1", svc, 8080)
	time.Sleep(1000 * time.Millisecond)
	registerInstance(client.namingClient, "127.0.0.1", svc, 8080, 1, true, true, config.NamingMetadataLength)
}

func queryService(client *NacosClient, limiter *rate.Limiter, perfConfig PerfConfig) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	queryInstance(client.grpcProxy, client.serviceNames[rand.Intn(perfConfig.InstanceCountPerService)])
}
func RunNamingPerf(config PerfConfig) {
	regLimiter := rate.NewLimiter(rate.Limit(config.NamingRegTps/2), 1)
	queryLimiter := rate.NewLimiter(rate.Limit(config.NamingQueryQps), 1)
	startTime := time.Now().UnixMilli()

	for _, client := range namingClientSlice {
		switch config.PerfApi {
		case "namingQuery":
			go func() {
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					queryService(client, queryLimiter, config)
				}
			}()
		case "namingReg":
			go func() {
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					regAndDereg(client, regLimiter, config)
				}
			}()
		case "namingSubscribe":
			go func() {
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					regAndDereg(client, regLimiter, config)
				}
			}()

		default:
			logger.Warn("unknown perf api: " + config.PerfApi)
		}
	}
}
