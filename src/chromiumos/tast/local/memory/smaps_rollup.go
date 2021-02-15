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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// SmapsRollup contains all the fields from a /proc/<pid>/smaps_rollup file.
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

var smapsRollupRE = regexp.MustCompile(`(?m)([[:xdigit:]]+)-([[:xdigit:]]+) (r|-)(w|-)(x|-)(s|p) ([[:xdigit:]]+) ([[:xdigit:]]+):([[:xdigit:]]+) ([[:digit:]]+) +\[rollup]
Rss: +(?P<Rss>[[:digit:]]+) kB
Pss: +(?P<Pss>[[:digit:]]+) kB
Pss_Anon: +(?P<PssAnon>[[:digit:]]+) kB
Pss_File: +(?P<PssFile>[[:digit:]]+) kB
Pss_Shmem: +(?P<PssShmem>[[:digit:]]+) kB
Shared_Clean: +(?P<SharedClean>[[:digit:]]+) kB
Shared_Dirty: +(?P<SharedDirty>[[:digit:]]+) kB
Private_Clean: +(?P<PrivateClean>[[:digit:]]+) kB
Private_Dirty: +(?P<PrivateDirty>[[:digit:]]+) kB
Referenced: +(?P<Referenced>[[:digit:]]+) kB
Anonymous: +(?P<Anonymous>[[:digit:]]+) kB
LazyFree: +(?P<LazyFree>[[:digit:]]+) kB
AnonHugePages: +(?P<AnonHugePages>[[:digit:]]+) kB
ShmemPmdMapped: +(?P<ShmemPmdMapped>[[:digit:]]+) kB
Shared_Hugetlb: +(?P<SharedHugetlb>[[:digit:]]+) kB
Private_Hugetlb: +(?P<PrivateHugetlb>[[:digit:]]+) kB
Swap: +(?P<Swap>[[:digit:]]+) kB
SwapPss: +(?P<SwapPss>[[:digit:]]+) kB
Locked: +(?P<Locked>[[:digit:]]+) kB
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

// NewSmapsRollup parses the contents of a /proc/<pid>/smaps_rollup file.
func NewSmapsRollup(smapsRollupFileData []byte) (*SmapsRollup, error) {
	match := smapsRollupRE.FindSubmatch(smapsRollupFileData)
	if match == nil {
		return nil, errors.Errorf("failed to parse smaps_rollup file %q", smapsRollupFileData)
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

// We don't match processes with Z or I in the stat column because zombies and
// kernel threads don't have smaps_rollup.
var psHasSmapsRollupRE = regexp.MustCompile("(?m)^ *([[:digit:]]+) +([DRSTtW]) +(.*)$")

// AllSmapsRollups uses ps to query running processes, and then collects a
// SmapsRollup for each, annotating each with it's pid and command line.
func AllSmapsRollups(ctx context.Context) ([]*NamedSmapsRollup, error) {
	psOut, err := testexec.CommandContext(ctx, "ps", "axo", "pid=,s=,command").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get running processes")
	}

	matches := psHasSmapsRollupRE.FindAllSubmatch(psOut, -1)
	if matches == nil {
		return nil, errors.Errorf("failed to parse ps output %q", psOut)
	}

	c := make(chan *NamedSmapsRollup)
	for _, match := range matches {
		pid, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse pid %s", match[1])
		}
		cmd := string(match[3])
		go func() {
			if smapsData, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/smaps_rollup", pid)); err != nil {
				c <- nil
			} else if rollup, err := NewSmapsRollup(smapsData); err != nil {
				c <- nil
			} else {
				c <- &NamedSmapsRollup{
					Command:     cmd,
					Pid:         pid,
					SmapsRollup: *rollup,
				}
			}
		}()
	}
	var rollups []*NamedSmapsRollup
	for range matches {
		rollup := <-c
		if rollup != nil {
			rollups = append(rollups, rollup)
		}
	}
	return rollups, nil
}

type processCategory struct {
	commandRE *regexp.Regexp
	name      string
}

// processCatagories defines categories used to aggregate per-process memory
// metrics. The first commandRE to match a process' command line defines its
// category.
var processCatagories = []processCategory{
	{
		commandRE: regexp.MustCompile(`^/usr/bin/crosvm run.*/arcvm.sock`),
		name:      "crosvm_arcvm",
	}, {
		commandRE: regexp.MustCompile(`^/usr/bin/crosvm`),
		name:      "crosvm_other",
	}, {
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome --type=renderer`),
		name:      "chrome_renderer",
	}, {
		commandRE: regexp.MustCompile(`^/opt/google/chrome/chrome`),
		name:      "chrome_other",
	}, {
		commandRE: regexp.MustCompile(`.*`),
		name:      "other",
	},
}

// SmapsMetrics writes a JSON file containing data from every running process'
// smaps_rollup file. If perf.Values is not nil, it adds metrics based on
// processCatagories defined above.
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

	metrics := make(map[string]float64)
	for _, rollup := range rollups {
		for _, category := range processCatagories {
			if category.commandRE.MatchString(rollup.Command) {
				metrics[category.name] += float64(rollup.Pss) / MiB
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
			value,
		)
	}
	return nil
}
