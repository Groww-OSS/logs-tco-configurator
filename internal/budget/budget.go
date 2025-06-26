package budget

import (
	"configurator/internal/models"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Budget struct {
	Organizations []Organization `koanf:"orgs"`
}

type Organization struct {
	Name         string        `koanf:"name"`
	Environments []Environment `koanf:"envs"`
}
type Environment struct {
	Name      string     `koanf:"name"`
	Workloads []Workload `koanf:"workloads"`
}

type Workload struct {
	Name                 string `koanf:"name"`
	DailyIngestionBudget int    `koanf:"daily_ingestion_budget"`
}

// Global koanf instance. Use . as the key path delimiter. This can be / or anything.
var (
	k      = koanf.New(".")
	parser = yaml.Parser()
)

func New(path string) (Budget, error) {
	if err := k.Load(file.Provider(path), parser); err != nil {
		return Budget{}, fmt.Errorf("error loading budgetConfig: %v", err)
	}
	var budgetConfig Budget
	if err := k.Unmarshal("", &budgetConfig); err != nil {
		return Budget{}, fmt.Errorf("error unmarshaling budgetConfig: %v", err)
	}
	return budgetConfig, nil
}

func (b *Budget) ExtractBudget(orgName string, envName string) (map[string]models.GigaBytes, error) {

	log.Trace().
		Str("org", orgName).
		Str("env", envName).
		Msg("Extracting budget...")

	budgets := make(map[string]models.GigaBytes)
	for _, org := range b.Organizations {
		if org.Name == orgName {
			for _, env := range org.Environments {
				if env.Name == envName {
					for _, workload := range env.Workloads {
						budgets[workload.Name] = models.GigaBytes(workload.DailyIngestionBudget) // Convert to bytes
					}
				}
			}
		}
	}
	return budgets, nil
}

// Exract only workloads from the budget
func (b *Budget) ExtractWorkloads(orgName string, envName string) []string {
	log.Info().Msg(fmt.Sprintf("Extracting workloads for org: %v env: %v", orgName, envName))
	var workloads []string
	for _, org := range b.Organizations {
		if org.Name == orgName {
			for _, env := range org.Environments {
				if env.Name == envName {
					for _, workload := range env.Workloads {
						workloads = append(workloads, workload.Name)
					}
				}
			}
		}
	}
	return workloads
}

// CalculateDynamicBudget calculates the dynamic budget for each workload based on its resource requests.
func CalculateDynamicBudget(workloadResourceRequests []models.WorkloadResourceRequest, budgetOverideBytes map[string]models.GigaBytes, baselineBudgetMultiplier float64, minimumBudget float64) (map[string]models.GigaBytes, error) {

	var standardCPUCores models.Cores = 16.0

	b := 0.0

	log.Trace().
		Msg("calculating dynamic budget...")

	calculatedBudgetBytes := make(map[string]models.GigaBytes, len(workloadResourceRequests))

	for _, w := range workloadResourceRequests {

		b = float64(w.CPU/standardCPUCores) * baselineBudgetMultiplier

		if b < minimumBudget {
			b = minimumBudget
		}

		if override, ok := budgetOverideBytes[w.Workload]; ok {
			b = float64(override)

		}

		calculatedBudgetBytes[w.Workload] = models.GigaBytes(b) // Convert to bytes
	}

	return calculatedBudgetBytes, nil
}
