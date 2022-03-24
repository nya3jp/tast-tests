// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

var pgrepRE = regexp.MustCompile(`(?m)^(\d+)$`)
var meminfoRE = regexp.MustCompile(`(?m)^(\S+):\s+(\d+) kB$`)

func manateeCommand(ctx context.Context, args ...string) *testexec.Cmd {
	return testexec.CommandContext(ctx, "manatee", "-a", "shell-notty", "--", "-c", strings.Join(args, " "))
}

// ManaTEEMetrics generates metrics for ManaTEE hypervisor memory usage.
func ManaTEEMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	_, err := exec.LookPath("manatee")
	if err != nil {
		// If the manatee binary doesn't exist, then assume we're not running on manatee.
		return nil
	}

	pgrepCmd := manateeCommand(ctx, "pgrep", "crosvm")
	pgrepOut, err := pgrepCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute pgrep")
	}
	var pids []int
	matches := pgrepRE.FindAllSubmatch(pgrepOut, -1)
	for _, match := range matches {
		pid, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return errors.Wrapf(err, "failed to parse crosvm pid %q", match[1])
		}
		pids = append(pids, pid)
	}

	var crosGuestRssKiB uint64
	for _, pid := range pids {
		smapsCmd := manateeCommand(ctx, "cat", fmt.Sprintf("/proc/%d/smaps", pid))
		smapsOut, err := smapsCmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to cat smaps")
		}
		smaps, err := ParseSmapsData(smapsOut)
		if err != nil {
			return errors.Wrap(err, "failed to parse smaps")
		}
		for _, smap := range smaps {
			if strings.Contains(smap.name, "crosvm_guest") {
				crosGuestRssKiB += smap.rss
			}
		}
	}

	if crosGuestRssKiB == 0 {
		return errors.Wrap(err, "failed to find crosvm guest memory")
	}

	meminfoCmd := manateeCommand(ctx, "cat", "/proc/meminfo")
	meminfoOut, err := meminfoCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to cat meminfo")
	}

	var memTotatKiB, memFreeKiB, unevictableKiB uint64 = 0, 0, 0
	matches = meminfoRE.FindAllSubmatch(meminfoOut, -1)
	for _, match := range matches {
		val, err := strconv.ParseUint(string(match[2]), 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse value %q", match[2])
		}
		if string(match[1]) == "MemTotal" {
			memTotatKiB = val
		} else if string(match[1]) == "MemFree" {
			memFreeKiB = val
		} else if string(match[1]) == "Unevictable" {
			unevictableKiB = val
		}
	}
	if memTotatKiB == 0 || memFreeKiB == 0 {
		return errors.Errorf("failed to read host memory %d %d", memTotatKiB, memFreeKiB)
	}

	hypervisorMemKiB := memTotatKiB - memFreeKiB - crosGuestRssKiB

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("manatee_total_mem%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(memTotatKiB)/KiBInMiB,
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("manatee_hypervisor_mem%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(hypervisorMemKiB)/KiBInMiB,
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("manatee_hypervisor_unevictable%s", suffix),
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		float64(unevictableKiB)/KiBInMiB,
	)

	return nil
}
