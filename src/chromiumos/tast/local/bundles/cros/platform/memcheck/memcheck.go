// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memcheck implements the test scenario for the platform.MemCheck test.
package memcheck

import (
	"context"
	"io/ioutil"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func physicalMemSize(ctx context.Context) (uint64, error) {
	out, err := testexec.CommandContext(ctx, "mosys", "memory", "spd", "print", "geometry", "-s", "size_mb").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}

	var ret uint64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		v, err := strconv.ParseUint(line, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "Failed to parse memory size in %q", line)
		}
		ret += v
	}
	return ret, nil
}

func testMemInfo(ctx context.Context, s *testing.State) {
	s.Log("Running testMemInfo")

	// Find expected minimum value of MemTotal and VmallocTotal size.
	var expectMinTotal, expectMinVTotal uint64
	switch runtime.GOARCH {
	case "arm":
		expectMinTotal = 700000 * 1024
		expectMinVTotal = 210000 * 1024
	default:
		expectMinTotal = 986392 * 1024
		expectMinVTotal = 102400 * 1024
	}

	// Also calculate the expected MemTotal from physical memory size,
	// and use the bigger one.
	phySize, err := physicalMemSize(ctx)
	if err != nil {
		s.Error("Failed to obtain physical memory size: ", err)
		return
	}
	const (
		osReserveRatio = 0.04
		osReserveMin   = 600000 * 1024
	)
	osReserve := uint64(float64(phySize) * osReserveRatio)
	if osReserve < osReserveMin {
		osReserve = osReserveMin
	}
	var nonReservedMemSize uint64
	if phySize > osReserve {
		nonReservedMemSize = phySize - osReserve
	}
	if nonReservedMemSize > expectMinTotal {
		expectMinTotal = nonReservedMemSize
	}

	// Find expected swap size.
	const disksizePath = "/sys/block/zram0/disksize"
	content, err := ioutil.ReadFile(disksizePath)
	if err != nil {
		s.Error("Failed to read swap size: ", err)
		return
	}
	expectSwap, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		s.Errorf("Failed to parse swap from %q: %v", s, err)
		return
	}

	// Check the meminfo.
	m, err := mem.VirtualMemory()
	if err != nil {
		s.Error("Failed to get meminfo: ", err)
		return
	}

	if m.Total < expectMinTotal {
		s.Errorf("Unexpected MemTotal: got %d; want >= %d", m.Total, expectMinTotal)
	}

	if m.VMallocTotal < expectMinVTotal {
		s.Errorf("Unexpected VmallocTotal: got %d; want >= %d", m.VMallocTotal, expectMinVTotal)
	}

	minSwap := uint64(float64(expectSwap) * 0.9)
	maxSwap := uint64(float64(expectSwap) * 1.1)
	if m.SwapTotal < minSwap || maxSwap < m.SwapTotal {
		s.Errorf("Unexpected SwapTotal: got %d; want in [%f, %f]", m.SwapTotal, minSwap, maxSwap)
	}
}

func testRAMSpeed(ctx context.Context, s *testing.State) {
	s.Log("Running testRAMSpeed")
	const speedRef = 1333

	out, err := testexec.CommandContext(ctx, "mosys", "memory", "spd", "print", "timings", "-s", "speeds").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to read timings from spd: ", err)
		return
	}

	// Result example: DDR-800, DDR3-1066, DDR3-1333, DDR3-1600
	pattern := regexp.MustCompile(`^[A-Z]*DDR(?:[3-9]|[1-9]\d+)[A-Z]*-(\d+)$`)
	for dimm, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		words := strings.Split(line, ", ")
		maxTiming := words[len(words)-1]
		match := pattern.FindStringSubmatch(maxTiming)
		if match == nil {
			s.Errorf("Failed to parse timings for dimm #%d: got %s", dimm, maxTiming)
			continue
		}

		if speed, err := strconv.ParseInt(match[1], 10, 64); err != nil {
			s.Errorf("Failed to parse speed %q: %v", match[1], err)
			continue
		} else if speed < speedRef {
			s.Errorf("Unexpected speed: got %d; want >= %d", speed, speedRef)
			continue
		}
	}
}

// RunTest runs subtests of the MemCheck test.
func RunTest(ctx context.Context, s *testing.State) {
	testMemInfo(ctx, s)
	testRAMSpeed(ctx, s)
}
