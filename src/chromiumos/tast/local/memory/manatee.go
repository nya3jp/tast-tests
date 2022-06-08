// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"fmt"
	"os"
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

func manateePgrep(ctx context.Context, args ...string) ([]int, error) {
	pgrepCmd := manateeCommand(ctx, append([]string{"pgrep"}, args...)...)
	pgrepOut, err := pgrepCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute pgrep")
	}
	var pids []int
	matches := pgrepRE.FindAllSubmatch(pgrepOut, -1)
	for _, match := range matches {
		pid, err := strconv.Atoi(string(match[1]))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse crosvm pid %q", match[1])
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// getGuestMemoryRssSizes gets the RSS of all crosvm_guest memfds mapped into
// |pid|, as identified by the memfd's inode.
func getGuestMemoryRssSizes(ctx context.Context, pid int) (map[uint64]uint64, error) {
	rssSizes := make(map[uint64]uint64)

	smapsCmd := manateeCommand(ctx, "cat", fmt.Sprintf("/proc/%d/smaps", pid))
	smapsOut, err := smapsCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to cat smaps")
	}
	smaps, err := ParseSmapsData(smapsOut)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse smaps")
	}
	for _, smap := range smaps {
		if strings.Contains(smap.name, "crosvm_guest") {
			rssSizes[smap.inode] = rssSizes[smap.inode] + smap.rss
		}
	}
	if len(rssSizes) == 0 {
		return nil, errors.New("no guest memory found")
	}

	return rssSizes, nil
}

// ManaTEEMetrics generates metrics for ManaTEE hypervisor memory usage.
func ManaTEEMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	// Check whether or not we're running on ManaTEE by looking for the dugong
	// upstart config. If it doesn't exist, skip the ManaTEE metrics.
	if _, err := os.Stat("/etc/init/dugong.conf"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	// The oldest crosvm process will be the main process of the CrOS guest.
	pids, err := manateePgrep(ctx, "-o", "crosvm")
	if err != nil || len(pids) != 1 {
		return errors.Wrapf(err, "failed to find oldest crosvm: %v", pids)
	}

	// Sibling VM memory is mapped into the CrOS guest for virtio-vhost-user. We
	// can use inode as a stable identifier for the memfds and then filter out
	// the sibling memory when we process the sibling smaps later.
	crosGuestRssSizes, err := getGuestMemoryRssSizes(ctx, pids[0])
	if err != nil {
		return errors.Wrap(err, "failed to get CrOS guest's info")
	}

	// Crosvm processes that are the direct child of trichechus are the main processes
	// of sibling VMs. There are currently two trichechus processes - the older one reaps
	// child processes, and the newer one is the main process.
	trichechusPids, err := manateePgrep(ctx, "trichechus", "-n")
	if err != nil || len(trichechusPids) != 1 {
		return errors.Wrap(err, "failed to find trichechus")
	}

	siblingPids, err := manateePgrep(ctx, "-P", strconv.Itoa(trichechusPids[0]), "crosvm")
	if err != nil {
		return errors.Wrap(err, "failed to find sibling crosvm")
	}

	var crosGuestRssKiB uint64
	for _, pid := range siblingPids {
		rssSizes, err := getGuestMemoryRssSizes(ctx, pid)
		if err != nil {
			return errors.Wrap(err, "failed to find guest memory size")
		}
		if len(rssSizes) != 1 {
			return errors.Errorf("Malformed sibling info: %v", rssSizes)
		}
		for inode, rssSizeKiB := range rssSizes {
			delete(crosGuestRssSizes, inode)
			crosGuestRssKiB += rssSizeKiB
		}
	}

	if len(crosGuestRssSizes) != 1 {
		return errors.Errorf("unexpected crosGuestRssSizes %v", crosGuestRssSizes)
	}
	for _, rssSizeKiB := range crosGuestRssSizes {
		crosGuestRssKiB += rssSizeKiB
	}

	meminfoCmd := manateeCommand(ctx, "cat", "/proc/meminfo")
	meminfoOut, err := meminfoCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to cat meminfo")
	}

	var memTotatKiB, memFreeKiB, unevictableKiB uint64 = 0, 0, 0
	matches := meminfoRE.FindAllSubmatch(meminfoOut, -1)
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
