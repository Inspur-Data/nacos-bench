package internal

import (
	"context"
	"fmt"
	"math/rand" // 1. 确保导入 math/rand
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

var operationCounter atomic.Uint64

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
		// 修复(删除)BUG：并发写入全局切片
		// serviceNames = append(serviceNames, svc)
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
		Username:            perfConfig.Username,
		Password:            perfConfig.Password,
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
	for i := 0; i < perfConfig.ClientCount; i++ {
		wg.Add(1)
		go initClient(clientConfig, serverConfigs, perfConfig, &wg)
	}
	wg.Wait()
	fmt.Println("init client success")

	var wgReg sync.WaitGroup
	fmt.Printf("Registering %d initial instances in parallel (this may take a moment)...\n", len(namingClientSlice)*perfConfig.InstanceCountPerService)
	for _, client := range namingClientSlice {
		for _, serviceName := range client.serviceNames {
			wgReg.Add(1)
			go func(cli *NacosClient, svcName string) {
				defer wgReg.Done()
				registerInstance(cli.namingClient, "127.0.0.1", svcName, 8080, 1, true, true, perfConfig.NamingMetadataLength)
			}(client, serviceName)
		}
	}
	wgReg.Wait()
	fmt.Println("Initial instance registration complete.")

	if perfConfig.PerfApi == "namingSubscribe" {
		var wgSub sync.WaitGroup
		fmt.Println("Subscribing to services in parallel...")
		for _, client := range namingClientSlice {
			for i := 0; i < 3*perfConfig.InstanceCountPerService; i++ {
				wgSub.Add(1)
				go func(cli *NacosClient) {
					defer wgSub.Done()
					cli.namingClient.Subscribe(&vo.SubscribeParam{
						ServiceName:       serviceNames[rand.Intn(len(serviceNames))],
						GroupName:         "DEFAULT_GROUP",
						SubscribeCallback: func(services []model.Instance, err error) {},
					})
				}(client)
			}
		}
		wgSub.Wait()
		fmt.Println("Service subscription complete.")
	}
}

// 生成随机字符串
func generateRandomString(length int) string {
	// 2. 修复：不要在并发调用的函数内部调用 rand.Seed()。
	// 全局的 rand Source 会自动在程序启动时
	// rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))] // math/rand.Intn 内部有锁，是并发安全的
	}
	return string(b)
}

// 3. 修改：增加错误检查和日志打印
func registerInstance(namingClient naming_client.INamingClient, ip string, serviceName string, port int, weight float64, enabled bool, healthy bool, metadataLength int) {
	_, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
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
	if err != nil {
		// 打印错误日志
		logger.Warnf("RegisterInstance failed for service %s: %s", serviceName, err.Error())
	}
}

func queryInstance(proxy *naming_grpc.NamingGrpcProxy, serviceName string) {
	_, err := proxy.QueryInstancesOfService(serviceName, "DEFAULT_GROUP", "", 0, false)
	if err != nil {
		logger.Warn("queryInstance failed, caused: " + err.Error())
	}
}

// 4. 修改：增加错误检查和日志打印
func deregisterInstance(namingClient naming_client.INamingClient, ip string, serviceName string, port int) {
	_, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: serviceName,
		Ephemeral:   true,
	})
	if err != nil {
		// 打印错误日志
		logger.Warnf("DeregisterInstance failed for service %s: %s", serviceName, err.Error())
	}
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

	operationCounter.Add(1)
}

func queryService(client *NacosClient, limiter *rate.Limiter, perfConfig PerfConfig) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	queryInstance(client.grpcProxy, client.serviceNames[rand.Intn(perfConfig.InstanceCountPerService)])

	operationCounter.Add(1)
}

func RunNamingPerf(config PerfConfig) {
	regLimiter := rate.NewLimiter(rate.Limit(config.NamingRegTps/2), 1)
	queryLimiter := rate.NewLimiter(rate.Limit(config.NamingQueryQps), 1)
	startTime := time.Now().UnixMilli()

	operationCounter.Store(0)

	var wg sync.WaitGroup

	for _, client := range namingClientSlice {
		wg.Add(1)
		go func(c *NacosClient) {
			defer wg.Done()

			switch config.PerfApi {
			case "namingQuery":
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					queryService(c, queryLimiter, config)
				}
			case "namingReg":
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					regAndDereg(c, regLimiter, config)
				}
			case "namingSubscribe":
				for {
					if config.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(config.PerfTimeSec)*1000 {
						return
					}
					regAndDereg(c, regLimiter, config)
				}
			default:
				logger.Warn("unknown perf api: " + config.PerfApi)
				return
			}
		}(client)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	fmt.Printf("Performance test started. Will run for %d seconds...\n", config.PerfTimeSec)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastCount := uint64(0)
	totalDuration := float64(config.PerfTimeSec)
	if totalDuration <= 0 {
		totalDuration = 1
	}

Loop:
	for {
		select {
		case <-ticker.C:
			currentCount := operationCounter.Load()
			currentTps := (currentCount - lastCount) / 5
			lastCount = currentCount
			elapsed := (time.Now().UnixMilli() - startTime) / 1000
			if elapsed < 0 {
				elapsed = 0
			}

			progress := (float64(elapsed) / totalDuration) * 100
			if progress > 100 {
				progress = 100
			}

			fmt.Printf("[%ds/%ds, %.1f%%] Current TPS: %d, Total Ops: %d\n",
				elapsed, config.PerfTimeSec, progress, currentTps, currentCount)

		case <-done:
			fmt.Println("All workers finished.")
			break Loop
		}
	}

	finalCount := operationCounter.Load()
	totalTimeSec := (time.Now().UnixMilli() - startTime) / 1000
	if totalTimeSec <= 0 {
		totalTimeSec = 1
	}
	avgTps := float64(finalCount) / float64(totalTimeSec)
	fmt.Println("\n--- Performance Test Summary ---")
	fmt.Printf("Total operations: %d\n", finalCount)
	fmt.Printf("Total time: %d seconds\n", totalTimeSec)
	fmt.Printf("Average TPS: %.2f\n", avgTps)
	fmt.Println("----------------------------------")
}