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

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// SmapsRollup contains all the fields from a /proc/<pid>/smaps_rollup file.
// All sizes are in bytes.
type SmapsRollup struct {
	Rss,
	Pss,
	PssAnon,
	PssFile,
	PssShmem,
	SharedClean,
	SharedDirty,
	PrivateClean,
	PrivateDirty,
	Referenced,
	Anonymous,
	LazyFree,
	AnonHugePages,
	ShmemPmdMapped,
	SharedHugetlb,
	PrivateHugetlb,
	Swap,
	SwapPss,
	Locked uint64
}

var smapsRollupRE = regexp.MustCompile(`(?m)([[:xdigit:]]+)-([[:xdigit:]]+) (r|-)(w|-)(x|-)(s|p) ([[:xdigit:]]+) ([[:xdigit:]]+):([[:xdigit:]]+) ([\d]+) +\[rollup]
Rss: +(?P<Rss>[\d]+) kB
Pss: +(?P<Pss>[\d]+) kB
Pss_Anon: +(?P<PssAnon>[\d]+) kB
Pss_File: +(?P<PssFile>[\d]+) kB
Pss_Shmem: +(?P<PssShmem>[\d]+) kB
Shared_Clean: +(?P<SharedClean>[\d]+) kB
Shared_Dirty: +(?P<SharedDirty>[\d]+) kB
Private_Clean: +(?P<PrivateClean>[\d]+) kB
Private_Dirty: +(?P<PrivateDirty>[\d]+) kB
Referenced: +(?P<Referenced>[\d]+) kB
Anonymous: +(?P<Anonymous>[\d]+) kB
LazyFree: +(?P<LazyFree>[\d]+) kB
AnonHugePages: +(?P<AnonHugePages>[\d]+) kB
ShmemPmdMapped: +(?P<ShmemPmdMapped>[\d]+) kB
Shared_Hugetlb: +(?P<SharedHugetlb>[\d]+) kB
Private_Hugetlb: +(?P<PrivateHugetlb>[\d]+) kB
Swap: +(?P<Swap>[\d]+) kB
SwapPss: +(?P<SwapPss>[\d]+) kB
Locked: +(?P<Locked>[\d]+) kB
`)

func bytesFromSmapsRollupSubmatch(name string, match [][]byte) uint64 {
	submatch := match[smapsRollupRE.SubexpIndex(name)]
	kb, err := strconv.ParseUint(string(submatch), 10, 64)
	if err != nil {
		// We will only hit this if there is an error in the regexp above, or
		// smaps_rollup reports numbers too large to fit in 64 bit.
		panic(fmt.Sprintf("Failed to parse size from smaps_rollup field %q = %q: %s", name, submatch, err))
	}
	return kb * 1024
}

// NewSmapsRollup parses the contents of a /proc/<pid>/smaps_rollup file. All
// sizes are in bytes.
func NewSmapsRollup(smapsRollupFileData []byte) (*SmapsRollup, error) {
	match := smapsRollupRE.FindSubmatch(smapsRollupFileData)
	if match == nil {
		return nil, errors.Errorf("failed to parse smaps_rollup file %q %q", smapsRollupRE.String(), smapsRollupFileData)
	}
	return &SmapsRollup{
		Rss:            bytesFromSmapsRollupSubmatch("Rss", match),
		Pss:            bytesFromSmapsRollupSubmatch("Pss", match),
		PssAnon:        bytesFromSmapsRollupSubmatch("PssAnon", match),
		PssFile:        bytesFromSmapsRollupSubmatch("PssFile", match),
		PssShmem:       bytesFromSmapsRollupSubmatch("PssShmem", match),
		SharedClean:    bytesFromSmapsRollupSubmatch("SharedClean", match),
		SharedDirty:    bytesFromSmapsRollupSubmatch("SharedDirty", match),
		PrivateClean:   bytesFromSmapsRollupSubmatch("PrivateClean", match),
		PrivateDirty:   bytesFromSmapsRollupSubmatch("PrivateDirty", match),
		Referenced:     bytesFromSmapsRollupSubmatch("Referenced", match),
		Anonymous:      bytesFromSmapsRollupSubmatch("Anonymous", match),
		LazyFree:       bytesFromSmapsRollupSubmatch("LazyFree", match),
		AnonHugePages:  bytesFromSmapsRollupSubmatch("AnonHugePages", match),
		ShmemPmdMapped: bytesFromSmapsRollupSubmatch("ShmemPmdMapped", match),
		SharedHugetlb:  bytesFromSmapsRollupSubmatch("SharedHugetlb", match),
		PrivateHugetlb: bytesFromSmapsRollupSubmatch("PrivateHugetlb", match),
		Swap:           bytesFromSmapsRollupSubmatch("Swap", match),
		SwapPss:        bytesFromSmapsRollupSubmatch("SwapPss", match),
		Locked:         bytesFromSmapsRollupSubmatch("Locked", match),
	}, nil
}

// NamedSmapsRollup is a SmapsRollup plus the process name and ID.
type NamedSmapsRollup struct {
	Command string
	Pid     int
	SmapsRollup
}

// AllSmapsRollups returns a NamedSmapsRollup for every running process. Sizes
// are in bytes.
func AllSmapsRollups(ctx context.Context) ([]*NamedSmapsRollup, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all processes")
	}

	rollups := make([]*NamedSmapsRollup, len(processes))
	g, ctx := errgroup.WithContext(ctx)
	for index, process := range processes {
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
			}
			command, err := p.Cmdline()
			if err != nil {
				return errors.Wrapf(err, "failed to get command line for process %d", p.Pid)
			}
			rollup, err := NewSmapsRollup(smapsData)
			if err != nil {
				return err
			}
			rollups[i] = &NamedSmapsRollup{
				Command:     command,
				Pid:         int(p.Pid),
				SmapsRollup: *rollup,
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

// SmapsMetrics writes a JSON file containing data from every running process'
// smaps_rollup file. If perf.Values is not nil, it adds metrics based on
// processCategories defined above.
func SmapsMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	rollups, err := AllSmapsRollups(ctx)
	if err != nil {
		return err
	}
	rollupsJSON, err := json.MarshalIndent(rollups, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to convert smaps_rollups to JSON")
	}
	filename := fmt.Sprintf("smaps_rollup%s.json", suffix)
	if err := ioutil.WriteFile(path.Join(outdir, filename), rollupsJSON, 0644); err != nil {
		return errors.Wrapf(err, "failed to write smaps_rollups to %s", filename)
	}

	if p == nil {
		// No perf.Values, so don't compute metrics.
		return nil
	}

	metrics := make(map[string]struct{ pss, pssSwap float64 })
	for _, rollup := range rollups {
		for _, category := range processCategories {
			if category.commandRE.MatchString(rollup.Command) {
				metric := metrics[category.name]
				metric.pss += float64(rollup.Pss) / MiB
				metric.pssSwap += float64(rollup.SwapPss) / MiB
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
	}
	return nil
}
