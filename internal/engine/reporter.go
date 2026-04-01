package engine

import (
	"time"

	"github.com/teknikqa/upkeep/internal/provider"
)

// ProviderReport holds the aggregated outcome for one provider.
type ProviderReport struct {
	Name        string
	DisplayName string
	Status      string // "success" | "partial" | "failed" | "skipped" | "unavailable"
	Updated     int
	Deferred    int
	Skipped     int
	Failed      int
	Duration    time.Duration
	Error       error
}

// BuildReport derives per-provider reports from update results and scan results.
func BuildReport(
	providers []provider.Provider,
	scanResults map[string]provider.ScanResult,
	updateResults map[string]provider.UpdateResult,
) []ProviderReport {
	reports := make([]ProviderReport, 0, len(providers))
	for _, p := range providers {
		scan := scanResults[p.Name()]
		update, hasUpdate := updateResults[p.Name()]

		r := ProviderReport{
			Name:        p.Name(),
			DisplayName: p.DisplayName(),
		}

		if !scan.Available {
			r.Status = "unavailable"
			reports = append(reports, r)
			continue
		}

		if !hasUpdate {
			r.Status = "skipped"
			reports = append(reports, r)
			continue
		}

		r.Updated = len(update.Updated)
		r.Deferred = len(update.Deferred)
		r.Skipped = len(update.Skipped)
		r.Failed = len(update.Failed)
		r.Duration = update.Duration
		r.Error = update.Error

		switch {
		case update.Error != nil && r.Updated == 0 && r.Deferred == 0:
			r.Status = "failed"
		case r.Failed > 0 || (update.Error != nil):
			r.Status = "partial"
		case r.Deferred > 0 && r.Updated == 0 && r.Failed == 0:
			r.Status = "partial"
		default:
			r.Status = "success"
		}

		reports = append(reports, r)
	}
	return reports
}

// SummaryCounts returns total updated/deferred/skipped/failed across all reports.
func SummaryCounts(reports []ProviderReport) (updated, deferred, skipped, failed int) {
	for _, r := range reports {
		updated += r.Updated
		deferred += r.Deferred
		skipped += r.Skipped
		failed += r.Failed
	}
	return
}
