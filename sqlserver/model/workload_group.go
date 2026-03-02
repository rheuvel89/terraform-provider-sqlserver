package model

type WorkloadGroup struct {
	GroupID                      int64
	Name                         string
	PoolName                     string
	Importance                   string
	RequestMaxMemoryGrantPercent int
	RequestMaxCPUTimeSec         int
	RequestMemoryGrantTimeoutSec int
	MaxDOP                       int
	GroupMaxRequests             int
}
