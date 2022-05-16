// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"path"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const c0State = "C0"

type cpuIdleTimeFile struct {
	cpuName string
	path    string
}

// computeCpuidleStateFiles returns a mapping from cpuidle states to files
// containing the corresponding residency information.
func computeCpuidleStateFiles(ctx context.Context) (map[string][]cpuIdleTimeFile, int, error) {
	ret := make(map[string][]cpuIdleTimeFile)
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

			name, err := readFirstLine(path.Join(cpuDir, cpuidle.Name(), "name"))
			if err != nil {
				return nil, 0, errors.Wrap(err, "failed to read cpuidle name")
			}
			latency, err := readFirstLine(path.Join(cpuDir, cpuidle.Name(), "latency"))
			if err != nil {
				return nil, 0, errors.Wrap(err, "failed to read cpuidle latency")
			}

			if latency == "0" && name == "POLL" {
				// C0 state. Kernel stats aren't right, so calculate by
				// subtracting all other states from total time (using epoch
				// timer since we calculate differences in the end anyway).
				// NOTE: Only x86 lists C0 under cpuidle, ARM does not.
				continue
			}

			ret[name] = append(ret[name], cpuIdleTimeFile{
				cpuName: cpuInfo.Name(),
				path:    path.Join(cpuDir, cpuidle.Name(), "time"),
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
	cpuIdleTimeFiles map[string][]cpuIdleTimeFile
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
	cpuIdleTimeFiles, numCpus, err := computeCpuidleStateFiles(ctx)
	if err != nil {
		return errors.Wrap(err, "error finding cpuidles")
	}
	cs.cpuIdleTimeFiles = cpuIdleTimeFiles
	cs.numCpus = numCpus
	return nil
}

// readCpuidleStateTimes reads the cpuidle timings.
func readCpuidleStateTimes(cpuIdleTimeFiles map[string][]cpuIdleTimeFile) (map[string](map[string]int64), time.Time, error) {
	ret := make(map[string](map[string]int64))
	for cpuidle, files := range cpuIdleTimeFiles {
		for _, file := range files {
			t, err := readInt64(file.path)
			if err != nil {
				return nil, time.Time{}, errors.Wrap(err, "failed to read cpuidle timing")
			}
			if _, isPresent := ret[file.cpuName]; !isPresent {
				ret[file.cpuName] = make(map[string]int64)
			}
			ret[file.cpuName][cpuidle] = t
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

	stats, statTime, err := readCpuidleStateTimes(cs.cpuIdleTimeFiles)
	if err != nil {
		return errors.Wrap(err, "failed to collect initial metrics")
	}
	cs.metrics[c0State] = perf.Metric{Name: "cpu-" + c0State, Unit: "percent",
		Direction: perf.SmallerIsBetter, Multiple: true}
	for cpu, perCpuStats := range stats {
		for name := range perCpuStats {
			if _, isPresent := cs.metrics[name]; !isPresent {
				// All-core stats
				cs.metrics[name] = perf.Metric{Name: "cpu-" + name, Unit: "percent",
					Direction: perf.SmallerIsBetter, Multiple: true}
			}
			// Per-core stats
			cs.metrics[cpu+"-"+name] = perf.Metric{Name: cpu + "-" + name, Unit: "percent",
				Direction: perf.SmallerIsBetter, Multiple: true}
		}
		cs.metrics[cpu+"-"+c0State] = perf.Metric{Name: cpu + "-" + c0State, Unit: "percent",
			Direction: perf.SmallerIsBetter, Multiple: true}

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

	stats, statTime, err := readCpuidleStateTimes(cs.cpuIdleTimeFiles)
	if err != nil {
		return errors.Wrap(err, "failed to collect metrics")
	}

	diffs := make(map[string](map[string]int64))
	for cpu, perCpuStats := range stats {
		diffs[cpu] = make(map[string]int64)
		for stateName, stat := range perCpuStats {
			diffs[cpu][stateName] = stat - cs.lastStats[cpu][stateName]
		}
	}

	timeSlice := statTime.Sub(cs.lastTime).Microseconds()
	total := timeSlice * int64(cs.numCpus)
	c0Residency := total
	totalResidency := make(map[string]int64)
	for cpu, perCpuDiffs := range diffs {
		perCpuC0Residency := timeSlice
		for stateName, diff := range perCpuDiffs {
			values.Append(cs.metrics[cpu+"-"+stateName], float64(diff)/float64(timeSlice))
			c0Residency -= diff
			perCpuC0Residency -= diff
			if _, isPresent := totalResidency[stateName]; isPresent {
				totalResidency[stateName] += diff
			} else {
				totalResidency[stateName] = diff
			}
		}
		values.Append(cs.metrics[cpu+"-"+c0State], float64(perCpuC0Residency)/float64(timeSlice))
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
