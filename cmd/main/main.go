package main

import (
	"flag"
	"log"

	bench_internal "github.com/nacos-group/nacos-bench/internal"
)

var initGoroutinePool *bench_internal.Pool

func main() {
	nacosServerAddr := flag.String("nacosServerAddr", "127.0.0.1", "nacos server address")
	nacosClientCount := flag.Int("nacosClientCount", 1000, "nacos client count")
	serviceCount := flag.Int("serviceCount", 15000, "service count")
	instanceCountPerService := flag.Int("instanceCountPerService", 3, "instance count per service")
	namingMetadataLength := flag.Int("namingMetadataLength", 128, "naming metadata length")
	perfMode := flag.String("perfMode", "naming", "perf mode")
	perfTps := flag.Int("perfTps", 500, "perfTps tps")
	configContentLength := flag.Int("configContentLength", 128, "config content length")
	configCount := flag.Int("configCount", 1000, "config count")
	perfTimeSec := flag.Int("perfTime", 600, "perf time second")
	perfApi := flag.String("perfApi", "namingReg", "perf api, include namingReg,namingQuery,namingSubscribe,configPub,configGet.")
	flag.Parse()

	initGoroutinePool = bench_internal.NewPool(100)
	initGoroutinePool.Run()

	perfConfig := bench_internal.PerfConfig{
		ClientCount:             *nacosClientCount,
		ConfigContentLength:     *configContentLength,
		ConfigGetTps:            *perfTps,
		InstanceCountPerService: *instanceCountPerService,
		NacosAddr:               *nacosServerAddr,
		ConfigPubTps:            *perfTps,
		PerfMode:                *perfMode,
		ServiceCount:            *serviceCount,
		NamingMetadataLength:    *namingMetadataLength,
		NamingQueryQps:          *perfTps,
		NamingRegTps:            *perfTps,
		PerfTimeSec:             *perfTimeSec,
		ConfigCount:             *configCount,
		PerfApi:                 *perfApi,
	}

	if perfConfig.PerfMode == "config" {
		bench_internal.InitConfig(perfConfig)
		bench_internal.RunConfigPerf(perfConfig)
	} else if perfConfig.PerfMode == "naming" {
		bench_internal.InitNaming(perfConfig)
		bench_internal.RunNamingPerf(perfConfig)
	} else {
		log.Fatal("PERF_MODE is required")
	}

	select {}
}
