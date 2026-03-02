package model

type ResourcePool struct {
	PoolID                 int64
	Name                   string
	MinCPUPercent          int
	MaxCPUPercent          int
	MinMemoryPercent       int
	MaxMemoryPercent       int
	CapCPUPercent          int
	MinIOPSPerVolume       int
	MaxIOPSPerVolume       int
	AffinitySchedulerRange string
}
