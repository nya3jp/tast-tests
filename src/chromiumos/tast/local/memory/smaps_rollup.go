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
	"sync/atomic"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/testing"
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
		result[field] = kb * KiB
	}
	return result, nil
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
// information on shared memory use (SharedMem).
type NamedSmapsRollup struct {
	Command   string
	Pid       int32
	SharedMem SharedInfo
	Rollup    map[string]uint64
}

// SharedInfoMap maps process ids to shared memory information.
type SharedInfoMap map[int32]*SharedInfo

func (t *NamedSmapsRollup) getPssAndSwap() uint64 {
	return t.Rollup["Pss"] + t.Rollup["SwapPss"] + t.SharedMem.SharedSwapPss
}

// smapsRollups returns a NamedSmapsRollup for every process in processes.
// It also fills the passed summary struct with PSS data about the crosvm * processes.
// Sizes are in bytes.
// The SharedMem field is initialized from sharedMemSummary, if provided.
func smapsRollups(ctx context.Context, processes []*process.Process, sharedMemSummary SharedInfoMap, crosvmPid int32, summary *HostSummary) ([]*NamedSmapsRollup, error) {
	rollups := make([]*NamedSmapsRollup, len(processes))
	g, ctx := errgroup.WithContext(ctx)
	for index, process := range processes {
		// All these are captured by value in the closure - and that is what we want.
		i := index
		p := process
		isCrosvmParent := (p.Pid == crosvmPid)
		isCrosvmChild := false
		if crosvmPid != -1 && !isCrosvmParent {
			parentPid, err := p.Ppid()
			if err == nil {
				isCrosvmChild = (parentPid == crosvmPid)
			}
		}
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
				// Process may have died between reading smapsData and now, so
				// just ignore errors here.
				testing.ContextLogf(ctx, "SmapsRollups failed to get Cmdline for process %d: %s", p.Pid, err)
				return nil
			}
			rollup, err := NewSmapsRollup(smapsData)
			if err != nil {
				return errors.Wrapf(err, "failed to parse /proc/%d/smaps_rollup", p.Pid)
			}
			sharedMemInfo := sharedMemSummary[p.Pid]
			namedRollup := &NamedSmapsRollup{
				Command: command,
				Pid:     p.Pid,
				Rollup:  rollup,
			}
			if sharedMemInfo != nil {
				namedRollup.SharedMem = *sharedMemInfo
			}
			rollups[i] = namedRollup
			if isCrosvmParent {
				atomic.AddUint64(&summary.CrosVMParentPss, namedRollup.getPssAndSwap())
				atomic.AddUint64(&summary.CrosVMParentGuestMap, namedRollup.SharedMem.CrosvmGuestPss)
			} else if isCrosvmChild {
				atomic.AddUint64(&summary.CrosVMChildrenPss, namedRollup.getPssAndSwap())
				atomic.AddUint64(&summary.CrosVMChildrenGuestMap, namedRollup.SharedMem.CrosvmGuestPss)
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

// getCrosVMMainPid returns the pid of the main CrosVm process.
func getCrosVMMainPid(ctx context.Context) (int32, error) {
	out, err := testexec.CommandContext(ctx, "pgrep", "vm_concierge").CombinedOutput()
	status, ok := testexec.ExitCode(err)
	vmConciergePid := strings.TrimSpace(string(out))
	if !ok || status != 0 {
		return -1, errors.Wrapf(err, "failed to get vm_concierge pid, err=%d: %s", status, vmConciergePid)
	}

	out, err = testexec.CommandContext(ctx, "pgrep", "-P", vmConciergePid, "-f", "^/usr/bin/crosvm run.*/arcvm").CombinedOutput()
	crosvmMainPid := strings.TrimSpace(string(out))
	status, ok = testexec.ExitCode(err)
	if !ok || status != 0 {
		return -1, errors.Wrapf(err, "failed to get crosvm main pid, err=%d: %s", status, crosvmMainPid)
	}

	rv, err := strconv.ParseInt(crosvmMainPid, 10, 32)
	if err != nil {
		return -1, errors.Wrapf(err, "Unable to convert crosvm pid to integer: %s", crosvmMainPid)
	}
	return int32(rv), err
}

// makeSharedMemSummary creates a map from Pid to the amount of SwapPss used by shared
// mappings per process. The SwapPss field in smaps_rollup does not include
// memory swapped out of shared mappings. In order to calculate a complete
// SwapPss, we parse smaps for all shared mappings in all processes, and then
// divide their "Swap" value by the number of times the shared memory is mapped.
func makeSharedMemSummary(ctx context.Context, processes []*process.Process) (SharedInfoMap, error) {
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
				pss := pssKiB * KiB
				swap := swapKiB * KiB
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
	sharedMemSummary := make(SharedInfoMap)
	for i, swaps := range procSwaps {
		pid := processes[i].Pid
		sharedInfo := &SharedInfo{}
		sharedMemSummary[pid] = sharedInfo
		for _, swap := range swaps {
			if strings.HasPrefix(swap.name, "/memfd:crosvm_guest") {
				sharedInfo.CrosvmGuestPss += swap.pss
			}
			sharedInfo.SharedSwapPss += swap.swap / mapCount[swap.name]
		}
	}
	return sharedMemSummary, nil
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

// HostMetrics writes a JSON file containing data from every running process'
// smaps_rollup file. If perf.Values is not nil, it adds metrics based on
// processCategories defined above. If outdir is "", then no logs are written.
func HostMetrics(ctx context.Context, vmEnabled bool, p *perf.Values, outdir, suffix string) (*HostSummary, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all processes")
	}

	var crosvmPid int32 = -1
	if vmEnabled {
		crosvmPid, err = getCrosVMMainPid(ctx)
		if err != nil {
			testing.ContextLogf(ctx, "Unable to get crosvm pid, continuing without it - err=%s", err)
		}
	}

	sharedMemSummary, err := makeSharedMemSummary(ctx, processes)
	if err != nil {
		return nil, err
	}

	summary := &HostSummary{}

	rollups, err := smapsRollups(ctx, processes, sharedMemSummary, crosvmPid, summary)
	if err != nil {
		return nil, err
	}

	meminfo, err := kernelmeter.ReadMemInfo()
	if err != nil {
		return nil, err
	}

	if len(outdir) > 0 {
		rollupsJSON, err := json.MarshalIndent(rollups, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert smaps_rollups to JSON")
		}
		filename := fmt.Sprintf("smaps_rollup%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), rollupsJSON, 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write smaps_rollups to %s", filename)
		}
	}

	if p == nil {
		// No perf.Values, so don't compute metrics.
		return nil, nil
	}

	summary.MemTotal = uint64(meminfo["MemTotal"])
	summary.MemFree = uint64(meminfo["MemFree"])
	summary.HostCachedKernel = uint64(meminfo["SReclaimable"] + meminfo["Buffers"] + meminfo["Cached"] - meminfo["Mapped"])

	metrics := make(map[string]struct{ pss, pssSwap float64 })
	for _, rollup := range rollups {
		for _, category := range processCategories {
			if category.commandRE.MatchString(rollup.Command) {
				metric := metrics[category.name]
				pss, ok := rollup.Rollup["Pss"]
				if !ok {
					return nil, errors.Errorf("smaps_rollup for process %d does not include Pss", rollup.Pid)
				}
				swapPss, ok := rollup.Rollup["SwapPss"]
				if !ok {
					return nil, errors.Errorf("smaps_rollup for process %d does not include SwapPss", rollup.Pid)
				}
				metric.pss += float64(pss) / MiB
				metric.pssSwap += float64(swapPss+rollup.SharedMem.SharedSwapPss) / MiB
				metrics[category.name] = metric
				// Only the first matching category should contain this process.
				break
			}
		}
	}

	for name, value := range metrics {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			value.pss,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss_swap", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			value.pssSwap,
		)
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s_pss_total", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			value.pss+value.pssSwap,
		)
	}
	return summary, nil
}
