package internal

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic" // 1. 导入 atomic
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger" // 2. 导入 logger
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"golang.org/x/time/rate"
)

var configClients []config_client.IConfigClient
var dataIds []string

// 3. 增加 config 模式的全局原子计数器
var configOperationCounter atomic.Uint64

// 4. 增加 charset (原代码中缺失，但 generateRandomString 依赖它)
const configCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"



// 6. 增加一个用于初始化的、不带日志的发布函数
func publishConfigSimple(client config_client.IConfigClient, dataId string, contentLength int) {
	_, err := client.PublishConfig(vo.ConfigParam{
		DataId:  dataId,
		Group:   "DEFAULT_GROUP",
		Content: generateRandomString(contentLength),
	})
	if err != nil {
		// 这里使用 Warnf，因为它只在初始化时发生
		logger.Warnf("Init: Failed to publish config %s: %s", dataId, err.Error())
	}
}

func InitConfig(perfConfig PerfConfig) {
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
	var mu sync.Mutex
	fmt.Printf("Initializing %d config clients in parallel...\n", perfConfig.ClientCount)
	for i := 0; i < perfConfig.ClientCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, _ := clients.NewConfigClient(vo.NacosClientParam{
				ClientConfig:  &clientConfig,
				ServerConfigs: serverConfigs,
			})
			mu.Lock()
			configClients = append(configClients, client)
			mu.Unlock() // 7. 修复：将 Unlock 移出 defer，确保在 Done 之前执行
		}()
	}
	wg.Wait()
	fmt.Println("Init config client success")

	configClient1 := configClients[0]

	// 8. 优化：只检查一次，而不是在循环里每次都检查
	fmt.Println("Checking if configs need initialization...")
	lastDataId := "nacos.config.perf.test.dataId." + strconv.Itoa(perfConfig.ConfigCount-1)
	content, err := configClient1.GetConfig(vo.ConfigParam{
		DataId: lastDataId,
		Group:  "DEFAULT_GROUP",
	})

	needInitConfig := true
	if err == nil && content != "" {
		fmt.Println("Configs already exist. Skipping initialization.")
		needInitConfig = false
	}

	// 9. 优化：无论是否初始化，都需要填充 dataIds 列表
	for i := 0; i < perfConfig.ConfigCount; i++ {
		dataId := "nacos.config.perf.test.dataId." + strconv.Itoa(i)
		dataIds = append(dataIds, dataId)
	}

	if needInitConfig {
		// 10. 优化：并行发布所有配置，解决启动卡死
		fmt.Printf("Publishing %d configs in parallel (this may take a moment)...\n", perfConfig.ConfigCount)
		var wgInit sync.WaitGroup
		for _, dataId := range dataIds {
			wgInit.Add(1)
			go func(id string) {
				defer wgInit.Done()
				publishConfigSimple(configClient1, id, perfConfig.ConfigContentLength)
			}(dataId) // 必须将 dataId 作为参数传入
		}
		wgInit.Wait()
		fmt.Println("Config initialization complete.")
	}

	if perfConfig.PerfApi == "configSubscribe" {
		// 11. 优化：并行订阅，解决启动卡死
		fmt.Println("Subscribing to configs in parallel...")
		var wgSub sync.WaitGroup
		for _, client := range configClients {
			wgSub.Add(1)
			go func(cli config_client.IConfigClient) {
				defer wgSub.Done()
				dataId := dataIds[0] // 让所有客户端都订阅第一个 config
				cli.ListenConfig(vo.ConfigParam{
					DataId: dataId,
					Group:  "DEFAULT_GROUP",
					OnChange: func(namespace, group, dataId, data string) {
						// 订阅的回调，保持为空
					},
				})
			}(client) // 必须将 client 作为参数传入
		}
		wgSub.Wait()
		fmt.Println("Config subscription complete.")
	}
}

// 12. 优化：增加错误日志和原子计数
func publicConfig(client config_client.IConfigClient, limiter *rate.Limiter, dataId string, dataLength int) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	_, err := client.PublishConfig(vo.ConfigParam{ // 捕获 err
		DataId:  dataId,
		Group:   "DEFAULT_GROUP",
		Content: generateRandomString(dataLength),
	})
	if err != nil {
		logger.Warnf("PublishConfig failed for %s: %s", dataId, err.Error())
	}
	configOperationCounter.Add(1) // 增加计数
}

// 13. 优化：增加错误日志和原子计数
func getConfig(client config_client.IConfigClient, limiter *rate.Limiter, dataId string) {
	if err := limiter.Wait(context.Background()); err != nil {
		return
	}
	_, err := client.GetConfig(vo.ConfigParam{ // 捕获 err
		DataId: dataId,
		Group:  "DEFAULT_GROUP",
	})
	if err != nil {
		logger.Warnf("GetConfig failed for %s: %s", dataId, err.Error())
	}
	configOperationCounter.Add(1) // 增加计数
}

// 14. 优化：重写 RunConfigPerf，增加日志、等待和自动退出
func RunConfigPerf(perfConfig PerfConfig) {
	startTime := time.Now().UnixMilli()
	configCount := perfConfig.ConfigCount

	// 重置计数器
	configOperationCounter.Store(0)

	// 使用 WaitGroup 来等待所有 goroutine 结束
	var wg sync.WaitGroup

	switch perfConfig.PerfApi {
	case "configPub":
		pubLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigPubTps), 1)
		for _, client := range configClients { // 遍历所有客户端
			wg.Add(1)
			go func(cli config_client.IConfigClient) { // 传入 client 副本
				defer wg.Done()
				for {
					if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
						return
					}
					publicConfig(cli, pubLimiter, dataIds[rand.Intn(configCount)], perfConfig.ConfigContentLength)
				}
			}(client)
		}
	case "configGet":
		getConfigLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigGetTps), 1)
		for _, client := range configClients { // 遍历所有客户端
			wg.Add(1)
			go func(cli config_client.IConfigClient) { // 传入 client 副本
				defer wg.Done()
				for {
					if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
						return
					}
					getConfig(cli, getConfigLimiter, dataIds[rand.Intn(configCount)])
				}
			}(client)
		}
	case "configSubscribe":
		// 订阅模式下，我们也通过 "Publish" 来触发变更事件，以此进行压测
		pubLimiter := rate.NewLimiter(rate.Limit(perfConfig.ConfigPubTps), 1)
		for _, client := range configClients {
			wg.Add(1)
			go func(cli config_client.IConfigClient) {
				defer wg.Done()
				for {
					if perfConfig.PerfTimeSec > 0 && time.Now().UnixMilli()-startTime > int64(perfConfig.PerfTimeSec)*1000 {
						return
					}
					// 仅发布到 dataIds[0]，因为所有客户端都订阅了它
					publicConfig(cli, pubLimiter, dataIds[0], perfConfig.ConfigContentLength)
				}
			}(client)
		}
	default:
		panic("unknown perf api")
	}

	// ---- 新增的日志和等待逻辑 ----

	done := make(chan struct{})
	go func() {
		wg.Wait()   // 阻塞，直到所有 wg.Done() 被调用
		close(done) // 发送完成信号
	}()

	fmt.Printf("Performance test started. Will run for %d seconds...\n", perfConfig.PerfTimeSec)
	ticker := time.NewTicker(5 * time.Second) // 每5秒报告一次
	defer ticker.Stop()

	lastCount := uint64(0)
	totalDuration := float64(perfConfig.PerfTimeSec)
	if totalDuration <= 0 {
		totalDuration = 1 // 避免除零
	}

Loop:
	for {
		select {
		case <-ticker.C:
			// 报告进度
			currentCount := configOperationCounter.Load()
			currentTps := (currentCount - lastCount) / 5 // 过去5秒的TPS
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
				elapsed, perfConfig.PerfTimeSec, progress, currentTps, currentCount)

		case <-done:
			// 所有任务已完成
			fmt.Println("All workers finished.")
			break Loop // 退出 select 循环
		}
	}

	// ---- 压测结束，打印总结报告 ----
	finalCount := configOperationCounter.Load()
	totalTimeSec := (time.Now().UnixMilli() - startTime) / 1000
	if totalTimeSec <= 0 {
		totalTimeSec = 1 // 避免除零
	}
	avgTps := float64(finalCount) / float64(totalTimeSec)
	fmt.Println("\n--- Performance Test Summary ---")
	fmt.Printf("Total operations: %d\n", finalCount)
	fmt.Printf("Total time: %d seconds\n", totalTimeSec)
	fmt.Printf("Average TPS: %.2f\n", avgTps)
	fmt.Println("----------------------------------")
}