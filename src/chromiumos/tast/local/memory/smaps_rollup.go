// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory/kernelmeter"
)

var smapsRollupRE = regexp.MustCompile(`(?m)^([^:]+):\s*(\d+)\s*kB$`)

// NewSmapsRollup parses the contents of a /proc/<pid>/smaps_rollup file. All
// sizes are in bytes.
func NewSmapsRollup(smapsRollupFileData []byte) (map[string]uint64, error) {
	result := make(map[string]uint64)
	matches := smapsRollupRE.FindAllSubmatch(smapsRollupFileData, -1)
	if matches == nil {
		return nil, errors.Errorf("failed to parse smaps_rollup file %q", string(smapsRollupFileData))
	}
	for _, match := range matches {
		field := string(match[1])
		kbString := string(match[2])
		kb, err := strconv.ParseUint(kbString, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %q value from smaps_rollup: %q", field, kbString)
		}
		result[field] = kb
	}
	return result, nil
}

// CategoryHostMetrics has a summary of host HostMetrics
// we keep on a per-category basis for reporting.
// All values in Kilobytes.
type CategoryHostMetrics struct {
	Pss      uint64
	PssSwap  uint64
	PssGuest uint64
}

// HostSummary captures a few key data items that are used to compute
// overall system memory status.
// All values are expressed in Kilobytes.
type HostSummary struct {
	MemTotal         uint64
	MemFree          uint64
	HostCachedKernel uint64
	CategoryMetrics  map[string]*CategoryHostMetrics
}

// SharedInfo holds shared memory use information for one process.
// SharedSwapPss is the amount of swap used by shared memory regions divided by
// the number of times those regions are mapped.
// CrosvmGuestPss is the sum of the Pss used by the crosvm_guest region,
//  (which means memory from the VM mapped on the host).
type SharedInfo struct {
	SharedSwapPss  uint64
	CrosvmGuestPss uint64
}

// NamedSmapsRollup is a SmapsRollup plus the process name and ID, and
// information on shared memory use (Shared field).
type NamedSmapsRollup struct {
	Command string
	Pid     int32
	Shared  *SharedInfo
	Rollup  map[string]uint64
}

// SharedInfoMap maps process ids to shared memory information.
type SharedInfoMap map[int32]*SharedInfo

// smapsRollups returns a NamedSmapsRollup for every process in processes.
// It also fills the passed summary struct with PSS data about the crosvm * processes.
// Sizes are in bytes.
// The Shared field is initialized from sharedInfoMap, if provided.
func smapsRollups(ctx context.Context, processes []*process.Process, sharedInfoMap SharedInfoMap, summary *HostSummary) ([]*NamedSmapsRollup, error) {
	rollups := make([]*NamedSmapsRollup, len(processes))
	g, ctx := errgroup.WithContext(ctx)
	for index, process := range processes {
		// All these are captured by value in the closure - and that is what we want.
		i := index
		p := process

		g.Go(func() error {
			// We're racing with this process potentially exiting, so just
			// ignore errors and don't generate a NamesSmapsRollup if we fail to
			// read anything from proc.
			smapsData, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/smaps_rollup", p.Pid))
			if err != nil {
				// Not all processes have a smaps_rollup, this process may have
				// exited.
				return nil
			} else if len(smapsData) == 0 {
				// On some processes, smaps_rollups exists but is empty.
				return nil
			}
			command, err := p.Cmdline()
			if err != nil {
				return errors.Wrapf(err, "failed to get command line for process %d", p.Pid)
			}
			rollup, err := NewSmapsRollup(smapsData)
			if err != nil {
				return errors.Wrapf(err, "failed to parse /proc/%d/smaps_rollup", p.Pid)
			}
			rollups[i] = &NamedSmapsRollup{
				Command: command,
				Pid:     p.Pid,
				Rollup:  rollup,
				Shared:  sharedInfoMap[p.Pid],
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to wait for all smaps_rollup parsing to be done")
	}
	var result []*NamedSmapsRollup
	for _, rollup := range rollups {
		if rollup != nil {
			result = append(result, rollup)
		}
	}
	return result, nil
}

// sharedSwapPssRE matches smaps entries that are mapped shared, with the
// following match groups:
// [1] The name of the mapping.
// [2] The PSS for that mapping within this process, in kIB
// [3] The size of swapped out pages in the mapping, in kiB.
var sharedSwapPssRE = regexp.MustCompile(`(?m)^[[:xdigit:]]+-[[:xdigit:]]+ [-r][-w][-x]s [[:xdigit:]]+ [[:xdigit:]]+:[[:xdigit:]]+ [\d]+ +(\S[^\n]*)$
(?:^\w+: +[^\n]+$
)*^Pss: +(\d+) kB$
(?:^\w+: +[^\n]+$
)*^Swap: +(\d+) kB$`)

type sharedMapping struct {
	name string
	swap uint64
	pss  uint64
}

// makeSharedInfoMap creates a map from Pid to the amount of SwapPss used by shared
// mappings per process. The SwapPss field in smaps_rollup does not include
// memory swapped out of shared mappings. In order to calculate a complete
// SwapPss, we parse smaps for all shared mappings in all processes, and then
// divide their "Swap" value by the number of times the shared memory is mapped.
func makeSharedInfoMap(ctx context.Context, processes []*process.Process) (SharedInfoMap, error) {
	g, ctx := errgroup.WithContext(ctx)
	procSwaps := make([][]sharedMapping, len(processes))
	for index, process := range processes {
		i := index
		pid := process.Pid
		g.Go(func() error {
			smapsData, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/smaps", pid))
			if err != nil {
				// Not all processes have a smaps_rollup, this process may have
				// exited.
				return nil
			}
			matches := sharedSwapPssRE.FindAllSubmatch(smapsData, -1)
			for _, match := range matches {
				name := string(match[1])
				pssKiB, err := strconv.ParseUint(string(match[2]), 10, 64)
				if err != nil {
					return errors.Wrapf(err, "failed to parse pss value %q", match[2])
				}
				swapKiB, err := strconv.ParseUint(string(match[3]), 10, 64)
				if err != nil {
					return errors.Wrapf(err, "failed to parse swap value %q", match[3])
				}
				pss := pssKiB
				swap := swapKiB
				procSwaps[i] = append(procSwaps[i], sharedMapping{name, swap, pss})
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to wait for all smaps parsing to be done")
	}
	// Count how many times each shared mapping has been mapped.
	mapCount := make(map[string]uint64)
	for _, swaps := range procSwaps {
		for _, swap := range swaps {
			mapCount[swap.name]++
		}
	}
	// Use the counts to divide each mapping's swap size to compute SwapPss.
	// Also stack up each process's share of the crosvm_guest mapping
	sharedInfoMap := make(SharedInfoMap)
	for i, swaps := range procSwaps {
		pid := processes[i].Pid
		sharedInfo := &SharedInfo{}
		sharedInfoMap[pid] = sharedInfo
		for _, swap := range swaps {
			if strings.HasPrefix(swap.name, "/memfd:crosvm_guest") {
				sharedInfo.CrosvmGuestPss += swap.pss
			}
			sharedInfo.SharedSwapPss += swap.swap / mapCount[swap.name]
		}
	}
	return sharedInfoMap, nil
}

type processCategory struct {
	commandRE *regexp.Regexp
	name      string
}

// processCategories defines categories used to aggregate per-process memory
// metrics. The first commandRE to match a process' command line defines its
// category.
var processCategories = []processCategory{
	{
		commandRE: regexp.MustCompile(`^/usr/bin/crosvm run.*/arcvm.sock`),
		name:      "crosvm_arcvm",
	}, {
		commandRE: regexp.MustCompile(`^/usr/bin/crosvm`),
		name:      "crosvm_other",
	}, {
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome.*--type=renderer`),
		name:      "chrome_renderer",
	}, {
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome.*--type=gpu-process`),
		name:      "chrome_gpu",
	}, {
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome.*--type=`),
		name:      "chrome_other",
	}, { // The Chrome browser is the only chrome without a --type argument.
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome`),
		name:      "chrome_browser",
	}, {
		commandRE: regexp.MustCompile(`.*`),
		name:      "other",
	},
}

// GetHostMetrics writes a JSON file containing data from every running process'
// smaps_rollup file. If perf.Values is not nil, it adds metrics based on
// processCategories defined above. If outdir is "", then no logs are written.
func GetHostMetrics(ctx context.Context, outdir, suffix string) (*HostSummary, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all processes")
	}

	sharedInfoMap, err := makeSharedInfoMap(ctx, processes)
	if err != nil {
		return nil, err
	}

	summary := &HostSummary{CategoryMetrics: make(map[string]*CategoryHostMetrics)}

	rollups, err := smapsRollups(ctx, processes, sharedInfoMap, summary)
	if err != nil {
		return nil, err
	}

	meminfo, err := kernelmeter.ReadMemInfo()
	if err != nil {
		return nil, err
	}

	if len(outdir) > 0 {
		// Dump intermediate data.
		rollupsJSON, err := json.MarshalIndent(rollups, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert smaps_rollups to JSON")
		}
		filename := fmt.Sprintf("smaps_rollup%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), rollupsJSON, 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write smaps_rollups to %s", filename)
		}
	}

	// Convert reported totals in bytes into the common KiB unit.
	summary.MemTotal = uint64(meminfo["MemTotal"]) / KiB
	summary.MemFree = uint64(meminfo["MemFree"]) / KiB
	summary.HostCachedKernel = uint64(meminfo["SReclaimable"]+meminfo["Buffers"]+meminfo["Cached"]-meminfo["Mapped"]) / KiB

	metrics := summary.CategoryMetrics // Shallow copy, as it is a dictionary.
	for _, rollup := range rollups {
		for _, category := range processCategories {
			if category.commandRE.MatchString(rollup.Command) {
				metric := metrics[category.name]
				if metric == nil {
					// This is the first time seeing this category, so add it as zeroes.
					metric = &CategoryHostMetrics{}
					metrics[category.name] = metric
				}
				pss, ok := rollup.Rollup["Pss"]
				if !ok {
					return nil, errors.Errorf("smaps_rollup for process %d does not include Pss", rollup.Pid)
				}
				swapPss, ok := rollup.Rollup["SwapPss"]
				if !ok {
					return nil, errors.Errorf("smaps_rollup for process %d does not include SwapPss", rollup.Pid)
				}
				metric.Pss += pss
				metric.PssSwap += swapPss
				if rollup.Shared != nil {
					metric.PssSwap += rollup.Shared.SharedSwapPss
					metric.PssGuest += rollup.Shared.CrosvmGuestPss
				}
				// Only the first matching category should contain this process.
				break
			}
		}
	}
	return summary, nil
}

// ReportHostMetrics outputs a set of representative metrics
// into the supplied performance data dictionary.
func ReportHostMetrics(summary *HostSummary, p *perf.Values, suffix string) {
	for name, value := range summary.CategoryMetrics {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(value.Pss)/KiBInMiB,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss_swap", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(value.PssSwap)/KiBInMiB,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss_total", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(value.Pss+value.PssSwap)/KiBInMiB,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_guest_shared", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(value.PssGuest)/KiBInMiB,
		)
	}
}
