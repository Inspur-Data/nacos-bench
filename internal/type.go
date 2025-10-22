package internal

type PerfConfig struct {
	NacosAddr               string
	NacosPort               uint64
	Username                string
	Password                string
	ServiceCount            int
	InstanceCountPerService int
	ClientCount             int
	NamingRegTps            int
	NamingQueryQps          int
	PerfMode                string
	ConfigPubTps            int
	ConfigGetTps            int
	ConfigContentLength     int
	ConfigCount             int
	NamingMetadataLength    int
	PerfTimeSec             int
	PerfApi                 string
}
