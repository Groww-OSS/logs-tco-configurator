package models

// will be replaced by WorkloadIngestedBytes
type IngestedBytes struct {
	Cluster  string
	Workload string
	Value    float64
}

type WorkloadIngestedBytes struct {
	Cluster  string
	Workload string
	Value    float64
}

type WorkloadResourceRequest struct {
	Cluster  string
	Workload string
	CPU      Cores
	Memory   Bytes
}

type OverBudgetWorkload struct {
	Cluster          string
	Workload         string
	Budget           GigaBytes
	CurrentIngestion GigaBytes
}

// Common workload struct - for future use
type Workload struct {
	Cluster          string
	Workload         string
	AvgCPUCores      float64
	AvgMemoryBytes   float64
	BudgetBaseline   float64
	BudgetOverride   float64
	CurrentIngestion float64
	BudgetFinal      float64
	OverBudget       bool
}

// custom types to reduce confusion
type MiliCores float64
type Cores float64
type Bytes float64
type GigaBytes float64
