// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	c0C1Key             = "C0_C1"
	aggregateNonC0C1Key = "non-C0_C1"
)

// fetchIntelCPUUarch gets the name of the x86 microarchitecture.
func fetchIntelCPUUarch() (string, error) {
	intelUarchTable := map[string]string{
		"06_4C": "Airmont",
		"06_1C": "Atom",
		"06_26": "Atom",
		"06_27": "Atom",
		"06_35": "Atom",
		"06_36": "Atom",
		"06_3D": "Broadwell",
		"06_47": "Broadwell",
		"06_4F": "Broadwell",
		"06_56": "Broadwell",
		"06_A5": "Comet Lake",
		"06_A6": "Comet Lake",
		"06_0D": "Dothan",
		"06_5C": "Goldmont",
		"06_7A": "Goldmont",
		"06_3C": "Haswell",
		"06_45": "Haswell",
		"06_46": "Haswell",
		"06_3F": "Haswell-E",
		"06_7D": "Ice Lake",
		"06_7E": "Ice Lake",
		"06_3A": "Ivy Bridge",
		"06_3E": "Ivy Bridge-E",
		"06_8E": "Kaby Lake",
		"06_9E": "Kaby Lake",
		"06_0F": "Merom",
		"06_16": "Merom",
		"06_17": "Nehalem",
		"06_1A": "Nehalem",
		"06_1D": "Nehalem",
		"06_1E": "Nehalem",
		"06_1F": "Nehalem",
		"06_2E": "Nehalem",
		"0F_03": "Prescott",
		"0F_04": "Prescott",
		"0F_06": "Presler",
		"06_2A": "Sandy Bridge",
		"06_2D": "Sandy Bridge",
		"06_37": "Silvermont",
		"06_4A": "Silvermont",
		"06_4D": "Silvermont",
		"06_5A": "Silvermont",
		"06_5D": "Silvermont",
		"06_4E": "Skylake",
		"06_5E": "Skylake",
		"06_55": "Skylake",
		"06_8C": "Tiger Lake",
		"06_8D": "Tiger Lake",
		"06_86": "Tremont",
		"06_96": "Tremont",
		"06_9C": "Tremont",
		"06_25": "Westmere",
		"06_2C": "Westmere",
		"06_2F": "Westmere",
	}
	out, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", errors.Wrap(err, "failed to read cpuinfo")
	}

	family := int64(0)
	familyRegex := regexp.MustCompile(`(?m)^cpu family\s+:\s+([0-9]+)$`)
	matches := familyRegex.FindAllStringSubmatch(string(out), -1)
	if matches != nil {
		family, err = strconv.ParseInt(matches[0][1], 10, 64)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse family")
		}
	}

	model := int64(0)
	modelRegex := regexp.MustCompile(`(?m)^model\s+:\s+([0-9]+)$`)
	matches = modelRegex.FindAllStringSubmatch(string(out), -1)
	if matches != nil {
		model, err = strconv.ParseInt(matches[0][1], 10, 64)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse model")
		}
	}

	return intelUarchTable[fmt.Sprintf("%02X_%02X", family, model)], nil
}

// fetchPackageStates gets a map of the package C-states to msr addresses.
func fetchPackageStates() (map[string]int64, error) {
	atomStates := map[string]int64{"C2": 0x3F8, "C4": 0x3F9, "C6": 0x3FA}
	nehalemStates := map[string]int64{"C3": 0x3F8, "C6": 0x3F9, "C7": 0x3FA}
	sandyBridgeStates := map[string]int64{"C2": 0x60D, "C3": 0x3F8, "C6": 0x3F9, "C7": 0x3FA}
	silvermontStates := map[string]int64{"C6": 0x3FA}
	goldmontStates := map[string]int64{"C2": 0x60D, "C3": 0x3F8, "C6": 0x3F9, "C10": 0x632}
	broadwellStates := map[string]int64{"C2": 0x60D, "C3": 0x3F8, "C6": 0x3F9, "C7": 0x3FA,
		"C8": 0x630, "C9": 0x631, "C10": 0x632}
	// model groups pulled from Intel SDM, volume 4
	// Group same package cstate using the older uarch name
	fullStateMap := map[string]map[string]int64{
		"Airmont":      silvermontStates,
		"Atom":         atomStates,
		"Broadwell":    broadwellStates,
		"Comet Lake":   broadwellStates,
		"Goldmont":     goldmontStates,
		"Haswell":      sandyBridgeStates,
		"Ice Lake":     broadwellStates,
		"Ivy Bridge":   sandyBridgeStates,
		"Ivy Bridge-E": sandyBridgeStates,
		"Kaby Lake":    broadwellStates,
		"Nehalem":      nehalemStates,
		"Sandy Bridge": sandyBridgeStates,
		"Silvermont":   silvermontStates,
		"Skylake":      broadwellStates,
		"Tiger Lake":   broadwellStates,
		"Tremont":      goldmontStates,
		"Westmere":     nehalemStates,
	}

	uarch, err := fetchIntelCPUUarch()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine uarch")
	}

	return fullStateMap[uarch], nil
}

// findCPUPerPackage returns a slice that contains 1 CPU from each package.
func findCPUPerPackage(ctx context.Context) ([]int, error) {
	packages := make(map[int]int)
	cpuInfos, err := ioutil.ReadDir("/dev/cpu")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /dev/cpu")
	}
	for _, cpuInfo := range cpuInfos {
		cpu, err := strconv.ParseInt(cpuInfo.Name(), 10, 32)
		if err != nil {
			// Skip misc files in the directory
			continue
		}
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology/physical_package_id", cpu)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.Wrap(err, "failed to determine package")
		}

		pkg, err := readInt64(ctx, path)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse package")
		}
		packages[int(pkg)] = int(cpu)
	}
	perPackageCPUs := make([]int, len(packages))
	for i, cpu := range packages {
		perPackageCPUs[i] = cpu
	}
	return perPackageCPUs, nil
}

func readMSR(addr int64, cpu int) (uint64, error) {
	file, err := os.Open(fmt.Sprintf("/dev/cpu/%d/msr", cpu))
	if err != nil {
		return 0, errors.Wrap(err, "failed to open msrs")
	}
	defer file.Close()
	data := make([]byte, 8)
	if _, err := file.ReadAt(data, addr); err != nil {
		return 0, errors.Wrap(err, "failed to read msrs")
	}
	return binary.LittleEndian.Uint64(data), nil
}

// readPackageCStates takes a list of CPUs which correspond to the device's
// packages and the package C-states, returns a map containing how long was
// spent in each state.
func readPackageCStates(perPackageCPUs []int, pCStates map[string]int64) (map[string]uint64, error) {
	ret := make(map[string]uint64)
	ret[c0C1Key] = 0
	ret[aggregateNonC0C1Key] = 0

	for _, cpu := range perPackageCPUs {
		tsc, err := readMSR(0x10, 0)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read tsc value")
		}
		ret[c0C1Key] += tsc
		for pcstate, addr := range pCStates {
			val, err := readMSR(addr, cpu)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read pcstate msr")
			}
			ret[pcstate] = val
			ret[c0C1Key] -= val
			ret[aggregateNonC0C1Key] += val
		}
	}
	return ret, nil
}

// PackageCStatesMetrics records the package C-states of the DUT. This metric
// is only supported on Intel devices.
type PackageCStatesMetrics struct {
	pCStates       map[string]int64
	perPackageCPUs []int
	lastStats      map[string]uint64
	metrics        map[string]perf.Metric
}

// Assert that PackageCStatesMetrics can be used in perf.Timeline.
var _ perf.TimelineDatasource = &PackageCStatesMetrics{}

// NewPackageCStatesMetrics creates a timeline metric to collect package
// C-state numbers.
func NewPackageCStatesMetrics() *PackageCStatesMetrics {
	return &PackageCStatesMetrics{nil, nil, nil, make(map[string]perf.Metric)}
}

// Setup determines what C-states are supported and which CPUs should be queried.
func (cs *PackageCStatesMetrics) Setup(ctx context.Context, prefix string) error {
	// ARM is not supported
	if arch := runtime.GOARCH; arch == "arm" || arch == "arm64" {
		return nil
	}

	pCStates, err := fetchPackageStates()
	if err != nil {
		return errors.Wrap(err, "error finding package C-states")
	}
	if pCStates == nil {
		testing.ContextLog(ctx, "Failed to find package C-states")
		return nil
	}
	perPackageCPUs, err := findCPUPerPackage(ctx)
	if err != nil {
		return errors.Wrap(err, "error finding per package cpus")
	}
	cs.pCStates = pCStates
	cs.perPackageCPUs = perPackageCPUs
	return nil
}

// Start collects initial package cstate numbers which we can use to
// compute the residency between now and the first Snapshot.
func (cs *PackageCStatesMetrics) Start(ctx context.Context) error {
	if cs.pCStates == nil {
		return nil // Not supported.
	}
	stats, err := readPackageCStates(cs.perPackageCPUs, cs.pCStates)
	if err != nil {
		return errors.Wrap(err, "failed to collect initial metrics")
	}
	for name := range stats {
		cs.metrics[name] = perf.Metric{Name: "package-" + name, Unit: "percent",
			Direction: perf.SmallerIsBetter, Multiple: true}
	}
	cs.lastStats = stats
	return nil
}

// Snapshot computes the package cstate residency between this and
// the previous snapshot, and reports them as metrics.
func (cs *PackageCStatesMetrics) Snapshot(ctx context.Context, values *perf.Values) error {
	if cs.pCStates == nil {
		return nil // Not supported
	}

	stats, err := readPackageCStates(cs.perPackageCPUs, cs.pCStates)
	if err != nil {
		return errors.Wrap(err, "failed to collect metrics")
	}

	diffs := make(map[string]uint64)
	for name, stat := range stats {
		diffs[name] = stat - cs.lastStats[name]
	}

	total := diffs[c0C1Key] + diffs[aggregateNonC0C1Key]
	for name, diff := range diffs {
		values.Append(cs.metrics[name], float64(diff)/float64(total))
	}
	cs.lastStats = stats
	return nil
}

// Stop does nothing.
func (cs *PackageCStatesMetrics) Stop(ctx context.Context, values *perf.Values) error {
	return nil
}
