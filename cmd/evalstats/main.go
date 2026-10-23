// Command evalstats writes an A/B run's per-side statistics and Welch's t-test comparisons to stats.json.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/stats"
)

const (
	exitClean = 0
	exitError = 2
)

const statsNotes = "two-sided alpha=0.05 via t-critical table; no p-values computed"

type Comparison struct {
	Arm         string
	DeltaMean   float64
	Df          float64
	MetricId    string
	Note        string
	Significant bool
	WelchT      float64
}

type Group struct {
	Arm      string
	MetricId string
	Values   []float64
}

type groupCollector struct {
	groups []Group
	index  map[groupKey]int
}

// add appends one record's measures to its side's groups.
func (c *groupCollector) add(arm string, measures map[string]float64) {
	for _, measure := range slices.Sorted(maps.Keys(measures)) {
		key := groupKey{arm: arm, metric: measure}
		position, isSeen := c.index[key]
		if !isSeen {
			position = len(c.groups)
			c.index[key] = position
			group := Group{Arm: arm, MetricId: measure}
			c.groups = append(c.groups, group)
		}
		c.groups[position].Values = append(c.groups[position].Values, measures[measure])
	}
}

// groupKey addresses one group: a side key and a measure.
type groupKey struct {
	arm    string
	metric string
}

func main() {
	os.Exit(run())
}

func computeComparisons(groups []Group, baseline string) ([]Comparison, error) {
	byMetric := make(map[string][]Group)
	metricOrder := make([]string, 0)
	baselineSeen := false
	for _, group := range groups {
		if _, ok := byMetric[group.MetricId]; !ok {
			metricOrder = append(metricOrder, group.MetricId)
		}
		byMetric[group.MetricId] = append(byMetric[group.MetricId], group)
		if group.Arm == baseline {
			baselineSeen = true
		}
	}
	if !baselineSeen {
		return nil, fmt.Errorf("computeComparisons: baseline arm %q has no metric values", baseline)
	}

	var comparisons []Comparison
	for _, metricId := range metricOrder {
		metricGroups := byMetric[metricId]
		var base Group
		haveBase := false
		for _, group := range metricGroups {
			if group.Arm == baseline {
				base = group
				haveBase = true
			}
		}
		if !haveBase {
			continue
		}
		for _, group := range metricGroups {
			if group.Arm == baseline {
				continue
			}
			welchT, df := stats.WelchT(group.Values, base.Values)
			note := ""
			if df == 0 {
				note = "insufficient replicas for significance (need >=2 per arm)"
			}
			comparison := Comparison{
				Arm:         group.Arm,
				DeltaMean:   stats.Mean(group.Values) - stats.Mean(base.Values),
				Df:          df,
				MetricId:    metricId,
				Note:        note,
				Significant: stats.SignificantAt05(welchT, df),
				WelchT:      welchT,
			}
			comparisons = append(comparisons, comparison)
		}
	}
	return comparisons, nil
}

func computeGroups(file *evals.AbFile) []Group {
	collector := &groupCollector{index: make(map[groupKey]int)}
	sides := []*evals.AbSide{&file.Baseline, &file.Variant}
	for _, side := range sides {
		for _, record := range side.Records {
			collector.add(side.Key, record.Measures)
		}
	}
	return collector.groups
}

func run() int {
	abPath := flag.String("ab", "", "path to the ab.json to read")
	outPath := flag.String("out", "", "path to write stats.json")
	flag.Parse()

	if *abPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evalstats -ab <ab.json> -out <stats.json>")
		return exitError
	}

	if err := writeStats(*abPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return exitError
	}
	return exitClean
}

func statsFrom(file *evals.AbFile) (evals.Stats, error) {
	baseline := file.Baseline.Key
	groups := computeGroups(file)

	statGroups := make([]evals.StatsGroup, 0, len(groups))
	for _, group := range groups {
		note := ""
		if len(group.Values) == 0 {
			note = "no usable values"
		}
		statGroup := evals.StatsGroup{
			Arm:      group.Arm,
			Mean:     stats.Mean(group.Values),
			MetricId: group.MetricId,
			N:        len(group.Values),
			Note:     note,
			StdDev:   stats.StdDev(group.Values),
			Variance: stats.Variance(group.Values),
		}
		statGroups = append(statGroups, statGroup)
	}

	var comparisons []evals.StatsComparison
	if len(groups) > 0 {
		computed, err := computeComparisons(groups, baseline)
		if err != nil {
			return evals.Stats{}, err
		}
		for _, comparison := range computed {
			statsComparison := evals.StatsComparison{
				Arm:         comparison.Arm,
				DeltaMean:   comparison.DeltaMean,
				Df:          comparison.Df,
				MetricId:    comparison.MetricId,
				Note:        comparison.Note,
				Significant: comparison.Significant,
				WelchT:      comparison.WelchT,
			}
			comparisons = append(comparisons, statsComparison)
		}
	}

	result := evals.Stats{
		Baseline:    baseline,
		Comparisons: comparisons,
		Groups:      statGroups,
		Notes:       statsNotes,
	}
	return result, nil
}

func writeStats(abPath, outPath string) error {
	data, err := os.ReadFile(abPath)
	if err != nil {
		return fmt.Errorf("writeStats: Failed to read ab file: %w", err)
	}

	var file evals.AbFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("writeStats: Failed to parse ab file: %w", err)
	}

	result, err := statsFrom(&file)
	if err != nil {
		return fmt.Errorf("writeStats: %w", err)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("writeStats: Failed to marshal stats: %w", err)
	}

	if err := os.WriteFile(outPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("writeStats: Failed to write stats file: %w", err)
	}
	return nil
}
