// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const c0State = "C0"

type cpuidleTimeFile struct {
	stateName string
	path      string
}

// computeCpuidleStateFiles returns a mapping from cpus to files
// containing the total time spent in the idle states.
func computeCpuidleStateFiles(ctx context.Context) (map[string][]cpuidleTimeFile, int, error) {
	ret := make(map[string][]cpuidleTimeFile)
	numCpus := 0

	const cpusDir = "/sys/devices/system/cpu/"
	cpuInfos, err := ioutil.ReadDir(cpusDir)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to find cpus")
	}

	for _, cpuInfo := range cpuInfos {
		// Match files with name cpu0, cpu1, ....
		if match, err := regexp.MatchString(`^cpu\d+$`, cpuInfo.Name()); err != nil {
			return nil, 0, errors.Wrap(err, "error trying to match cpu name")
		} else if !match {
			continue
		}
		numCpus++

		cpuDir := path.Join(cpusDir, cpuInfo.Name(), "cpuidle")
		cpuidles, err := ioutil.ReadDir(cpuDir)
		if err != nil {
			testing.ContextLogf(ctx, "System does not expose %v, skipping CPU", cpuDir)
			continue
		}

		for _, cpuidle := range cpuidles {
			// Match files with name state0, state1, ....
			if match, err := regexp.MatchString(`^state\d+$`, cpuidle.Name()); err != nil {
				return nil, 0, errors.Wrap(err, "error trying to match idle state name")
			} else if !match {
				continue
			}

			stateName, err := readFirstLine(path.Join(cpuDir, cpuidle.Name(), "name"))
			if err != nil {
				return nil, 0, errors.Wrap(err, "failed to read cpuidle name")
			}
			latency, err := readFirstLine(path.Join(cpuDir, cpuidle.Name(), "latency"))
			if err != nil {
				return nil, 0, errors.Wrap(err, "failed to read cpuidle latency")
			}

			if latency == "0" && stateName == "POLL" {
				// C0 state. Kernel stats aren't right, so calculate by
				// subtracting all other states from total time (using epoch
				// timer since we calculate differences in the end anyway).
				// NOTE: Only x86 lists C0 under cpuidle, ARM does not.
				continue
			}

			ret[cpuInfo.Name()] = append(ret[cpuInfo.Name()], cpuidleTimeFile{
				stateName: stateName,
				path:      path.Join(cpuDir, cpuidle.Name(), "time"),
			})
		}
	}

	return ret, numCpus, nil
}

// CpuidleStateMetrics records the C-states of the DUT.
//
// NOTE: The cpuidle timings are measured according to the kernel. They resemble
// hardware cstates, but they might not have a direct correspondence. Furthermore,
// they generally may be greater than the time the CPU actually spends in the
// corresponding cstate, as the hardware may enter shallower states than requested.
type CpuidleStateMetrics struct {
	cpuidleTimeFiles map[string][]cpuidleTimeFile
	numCpus          int
	lastTime         time.Time
	lastStats        map[string](map[string]int64)
	metrics          map[string]perf.Metric
}

// Assert that CpuidleStateMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &CpuidleStateMetrics{}

// NewCpuidleStateMetrics creates a timeline metric to collect C-state numbers.
func NewCpuidleStateMetrics() *CpuidleStateMetrics {
	return &CpuidleStateMetrics{nil, 0, time.Time{}, nil, make(map[string]perf.Metric)}
}

// Setup determines what C-states are supported and which CPUs should be queried.
func (cs *CpuidleStateMetrics) Setup(ctx context.Context, prefix string) error {
	cpuidleTimeFiles, numCpus, err := computeCpuidleStateFiles(ctx)
	if err != nil {
		return errors.Wrap(err, "error finding cpuidles")
	}
	cs.cpuidleTimeFiles = cpuidleTimeFiles
	cs.numCpus = numCpus
	return nil
}

// readCpuidleStateTimes reads the cpuidle timings and return a mapping from cpu idle states and cpu names
// to the time spent in the state & cpu pairs so far.
func readCpuidleStateTimes(cpuidleTimeFiles map[string][]cpuidleTimeFile) (map[string](map[string]int64), time.Time, error) {
	ret := make(map[string](map[string]int64))
	for cpuName, files := range cpuidleTimeFiles {
		for _, file := range files {
			t, err := readInt64(file.path)
			if err != nil {
				return nil, time.Time{}, errors.Wrap(err, "failed to read cpuidle timing")
			}
			if _, isPresent := ret[cpuName]; !isPresent {
				ret[cpuName] = make(map[string]int64)
			}
			ret[cpuName][file.stateName] = t
		}
	}
	return ret, time.Now(), nil
}

// Start collects initial cpuidle numbers which we can use to
// compute the residency between now and the first Snapshot.
func (cs *CpuidleStateMetrics) Start(ctx context.Context) error {
	if cs.numCpus == 0 {
		return nil
	}

	stats, statTime, err := readCpuidleStateTimes(cs.cpuidleTimeFiles)
	if err != nil {
		return errors.Wrap(err, "failed to collect initial metrics")
	}

	cs.metrics[c0State] = perf.Metric{Name: "cpu-" + c0State, Unit: "percent",
		Direction: perf.SmallerIsBetter, Multiple: true}

	for cpuName, perCPUStats := range stats {

		// Per-cpu stats
		cs.metrics[cpuName+"-"+c0State] = perf.Metric{Name: cpuName + "-" + c0State, Unit: "percent",
			Direction: perf.SmallerIsBetter, Multiple: true}

		for stateName := range perCPUStats {
			// Per-cpu stats
			cs.metrics[cpuName+"-"+stateName] = perf.Metric{Name: cpuName + "-" + stateName, Unit: "percent",
				Direction: perf.SmallerIsBetter, Multiple: true}

			if _, isPresent := cs.metrics[stateName]; !isPresent {
				// Aggregated metrics of all the cpus
				cs.metrics[stateName] = perf.Metric{Name: "cpu-" + stateName, Unit: "percent",
					Direction: perf.SmallerIsBetter, Multiple: true}

			}
		}

	}

	cs.lastStats = stats
	cs.lastTime = statTime
	return nil
}

// Snapshot computes the cpuidle residency between this and
// the previous snapshot, and reports them as metrics.
func (cs *CpuidleStateMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	if cs.numCpus == 0 {
		return nil
	}

	stats, statTime, err := readCpuidleStateTimes(cs.cpuidleTimeFiles)
	if err != nil {
		return errors.Wrap(err, "failed to collect metrics")
	}

	diffs := make(map[string](map[string]int64))
	for cpuName, perCPUStats := range stats {
		diffs[cpuName] = make(map[string]int64)
		for stateName, stat := range perCPUStats {
			diffs[cpuName][stateName] = stat - cs.lastStats[cpuName][stateName]
		}
	}

	timeSlice := statTime.Sub(cs.lastTime).Microseconds()
	total := timeSlice * int64(cs.numCpus)
	c0Residency := total
	// Total time spent in a state by all the cpus
	totalResidency := make(map[string]int64)

	for cpuName, perCPUDiffs := range diffs {
		perCPUC0Residency := timeSlice
		for stateName, diff := range perCPUDiffs {
			values.Append(cs.metrics[cpuName+"-"+stateName], float64(diff)/float64(timeSlice))
			c0Residency -= diff
			perCPUC0Residency -= diff

			totalResidency[stateName] += diff
		}
		values.Append(cs.metrics[cpuName+"-"+c0State], float64(perCPUC0Residency)/float64(timeSlice))
	}

	for stateName, diff := range totalResidency {
		values.Append(cs.metrics[stateName], float64(diff)/float64(total))
	}
	values.Append(cs.metrics[c0State], float64(c0Residency)/float64(total))

	cs.lastStats = stats
	cs.lastTime = statTime
	return nil
}

// Stop does nothing.
func (cs *CpuidleStateMetrics) Stop(ctx context.Context, values *perf.Values) error {
	return nil
}

type cpuidleCounter struct {
	// Static values that don't change
	name                                                       string
	latency                                                    uint64
	cpuidleStateDirs                                           []string
	timeMetric, usageMetric, timeTotalMetric, usageTotalMetric perf.Metric
}

type cpuidleCounters []cpuidleCounter

func (cs cpuidleCounters) Len() int {
	return len(cs)
}

func (cs cpuidleCounters) Less(i, j int) bool {
	return cs[i].latency < cs[j].latency
}

func (cs cpuidleCounters) Swap(i, j int) {
	temp := cs[i]
	cs[i] = cs[j]
	cs[j] = temp
}

func (cs cpuidleCounters) readValues() (cpuidleCounterValues, error) {
	var values []cpuidleCounterValue
	for _, counter := range cs {
		value := cpuidleCounterValue{}
		for _, stateDir := range counter.cpuidleStateDirs {
			idleTimeBytes, err := os.ReadFile(filepath.Join(stateDir, "time"))
			if err != nil {
				return nil, errors.Wrap(err, "failed to read cpuidle state time")
			}
			idleTimeString := strings.TrimSpace(string(idleTimeBytes))
			idleTime, err := strconv.ParseUint(idleTimeString, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse cpu idle state time %q", idleTimeString)
			}
			value.time += idleTime

			idleUsageBytes, err := os.ReadFile(filepath.Join(stateDir, "usage"))
			if err != nil {
				return nil, errors.Wrap(err, "failed to read cpuidle state usage")
			}
			idleUsageString := strings.TrimSpace((string(idleUsageBytes)))
			idleUsage, err := strconv.ParseUint(idleUsageString, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse cpu idle state usage %q", idleUsage)
			}
			value.usage += idleUsage
		}
		values = append(values, value)
	}
	return values, nil
}

type cpuidleCounterValue struct {
	usage, time uint64
}

type cpuidleCounterValues []cpuidleCounterValue

func (vs cpuidleCounterValues) reportDiff(counters cpuidleCounters, prevValues cpuidleCounterValues, duration time.Duration, p *perf.Values, total bool) error {
	if len(vs) != len(counters) || len(vs) != len(prevValues) {
		return errors.New("mismatch number of cpuidle counters")
	}
	seconds := duration.Seconds()
	// Loop from deepest to shalowest sleep.
	var totalUsage, totalTime uint64
	for i := len(vs) - 1; i >= 0; i-- {
		counter := counters[i]
		curr := vs[i]
		prev := prevValues[i]
		totalUsage += curr.usage - prev.usage
		totalTime += curr.time - prev.time
		if total {
			p.Set(counter.timeTotalMetric, (time.Duration(totalTime)*time.Microsecond).Seconds()/seconds)
			p.Set(counter.usageTotalMetric, float64(totalUsage)/seconds)
		} else {
			p.Append(counter.timeMetric, (time.Duration(totalTime)*time.Microsecond).Seconds()/seconds)
			p.Append(counter.usageMetric, float64(totalUsage)/seconds)
		}

	}
	return nil
}

// AggregateCpuidleMetrics aggregates idle time from all CPUs and states.
//
// NOTE: The cpuidle timings are measured according to the kernel. They resemble
// hardware cstates, but they might not have a direct correspondence. Furthermore,
// they generally may be greater than the time the CPU actually spends in the
// corresponding cstate, as the hardware may enter shallower states than requested.
type AggregateCpuidleMetrics struct {
	counters    cpuidleCounters
	startTime   time.Time
	startValues cpuidleCounterValues
	prevTime    time.Time
	prevValues  cpuidleCounterValues
}

// NewAggregateCpuidleMetrics creates a TimelineDatasource to log aggregate
// cpuidle time and usage metrics.
func NewAggregateCpuidleMetrics() perf.TimelineDatasource {
	return &AggregateCpuidleMetrics{
		prevTime: time.Time{},
		counters: nil,
	}
}

// Setup enumerates all the cpuidle state directories.
func (a *AggregateCpuidleMetrics) Setup(ctx context.Context, prefix string) error {
	nameFiles, err := filepath.Glob("/sys/devices/system/cpu/cpu*/cpuidle/state*/name")
	if err != nil {
		return errors.Wrap(err, "failed to search for cpuidle names")
	}
	counters := make(map[string]*cpuidleCounter)
	for _, nameFile := range nameFiles {
		nameBytes, err := os.ReadFile(nameFile)
		if err != nil {
			return errors.Wrap(err, "failed to read cpuidle state name")
		}
		name := strings.TrimSpace(string(nameBytes))
		stateDir := filepath.Dir(nameFile)
		// state, err := strconv.ParseUint(filepath.Base(stateDir)[5:], 10, 64)
		// if err != nil {
		// 	return errors.Wrapf(err, "failed to parse state number from dir %q", stateDir)
		// }
		latencyBytes, err := os.ReadFile(filepath.Join(stateDir, "latency"))
		if err != nil {
			return errors.Wrap(err, "failed to read cpuidle state latency")
		}
		latency, err := strconv.ParseUint(strings.TrimSpace(string(latencyBytes)), 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse latency from %q", string(latencyBytes))
		}
		if counter, ok := counters[name]; ok {
			if counter.latency != latency {
				return errors.Wrapf(err, "cpuidle counter %q has different latencies", name)
			}
			counter.cpuidleStateDirs = append(counter.cpuidleStateDirs, stateDir)
		} else {
			counters[name] = &cpuidleCounter{
				name:             name,
				latency:          latency,
				cpuidleStateDirs: []string{stateDir},
				timeMetric: perf.Metric{
					Name:      fmt.Sprintf("cpuidle_ratio_deeper_%s", name),
					Unit:      "cpus",
					Direction: perf.BiggerIsBetter,
					Multiple:  true,
				},
				usageMetric: perf.Metric{
					Name:      fmt.Sprintf("cpuidle_wakes_deeper_%s", name),
					Unit:      "wakes_per_s",
					Direction: perf.SmallerIsBetter,
					Multiple:  true,
				},
				timeTotalMetric: perf.Metric{
					Name:      fmt.Sprintf("cpuidle_ratio_deeper_%s_total", name),
					Unit:      "cpus",
					Direction: perf.SmallerIsBetter,
					Multiple:  true,
				},
				usageTotalMetric: perf.Metric{
					Name:      fmt.Sprintf("cpuidle_wakes_deeper_%s_total", name),
					Unit:      "wakes_per_s",
					Direction: perf.SmallerIsBetter,
					Multiple:  true,
				},
			}
		}
	}
	for _, counter := range counters {
		a.counters = append(a.counters, *counter)
	}
	// Sort counters based on their latency values, if there is a tie, we use
	// stable sort so that the original order in the state dirs is preserved.
	sort.Stable(a.counters)

	return nil
}

// Start records the initial time and usage values.
func (a *AggregateCpuidleMetrics) Start(ctx context.Context) error {
	values, err := a.counters.readValues()
	if err != nil {
		return err
	}
	a.startTime = time.Now()
	a.prevTime = a.startTime
	a.startValues = values
	a.prevValues = values
	return nil
}

// Snapshot computes the
func (a *AggregateCpuidleMetrics) Snapshot(ctx context.Context, p *perf.Values) error {
	t0 := time.Now()
	values, err := a.counters.readValues()
	if err != nil {
		return err
	}
	t1 := time.Now()
	duration := t1.Sub(a.prevTime)
	updateTime := t1.Sub(t0).Seconds()
	if updateTime/duration.Seconds() > 0.001 {
		testing.ContextLogf(ctx, "AggregateCpuidleMetrics took %fs to update", updateTime)
	}

	values.reportDiff(a.counters, a.prevValues, duration, p, false)
	a.prevValues = values
	a.prevTime = t1

	return nil
}

// Stop computes total metrics.
func (a *AggregateCpuidleMetrics) Stop(ctx context.Context, p *perf.Values) error {
	t0 := time.Now()
	values, err := a.counters.readValues()
	if err != nil {
		return err
	}
	t1 := time.Now()
	duration := t1.Sub(a.startTime)
	updateTime := t1.Sub(t0).Seconds()
	if updateTime/duration.Seconds() > 0.001 {
		testing.ContextLogf(ctx, "AggregateCpuidleMetrics took %fs to update", updateTime)
	}

	values.reportDiff(a.counters, a.startValues, duration, p, true)

	return nil
}
