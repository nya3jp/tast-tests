// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test, and through snapshots
// of values exported in procfs and sysfs.
package kernelmeter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// PSIMemoryStatsData contains stats from the Pressure Stall Information subsystem.
type PSIMemoryStatsData struct {
	SomeAvgShort, SomeAvgMedium, SomeAvgLong float64
	SomeTotal                                uint64
	FullAvgShort, FullAvgMedium, FullAvgLong float64
	FullTotal                                uint64
}

// PSIMemoryStats returns a snapshot of PSI memory values, or nil if PSI is not
// available on the system.
func PSIMemoryStats() (*PSIMemoryStatsData, error) {
	const psiFile = "/proc/pressure/memory"
	if _, err := os.Stat(psiFile); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	bytes, err := ioutil.ReadFile(psiFile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open PSI file")
	}
	lines := strings.Split(string(bytes), "\n")
	if len(lines) < 2 {
		return nil, errors.New("too few lines in PSI file")
	}
	// Example of /proc/pressure/memory content:
	// some avg10=0.00 avg60=0.00 avg300=0.00 total=1468431930
	// full avg10=0.00 avg60=0.00 avg300=0.00 total=658624525
	var avg [2][3]float64
	var t [2]uint64
	skip := [3]int{6, 6, 7}
	for i := 0; i < 2; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) != 5 {
			return nil, errors.New(fmt.Sprintf("unexpected line in PSI file: %q", lines[i]))
		}
		for f := 0; f < 3; f++ {
			avg[i][f], err = strconv.ParseFloat(fields[f+1][skip[f]:], 64)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("unexpected avg field in PSI line: %q", lines[i]))
			}
		}
		t[i], err = strconv.ParseUint(fields[4][6:], 10, 64)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unexpected total field in PSI line: %q", lines[1]))
		}
	}
	return &PSIMemoryStatsData{
		SomeAvgShort:  avg[0][0],
		SomeAvgMedium: avg[0][1],
		SomeAvgLong:   avg[0][2],
		SomeTotal:     t[0],
		FullAvgShort:  avg[1][0],
		FullAvgMedium: avg[1][1],
		FullAvgLong:   avg[1][2],
		FullTotal:     t[1],
	}, nil
}

// ZramStatsData contains stats from the zram block device.
type ZramStatsData struct{ Original, Compressed, Used uint64 }

// ZramStats returns zram block device usage counts.
func ZramStats(ctx context.Context) (*ZramStatsData, error) {
	const zramDir = "/sys/block/zram0"
	mmStats := filepath.Join(zramDir, "mm_stat")
	var fields []string
	bytes, err := ioutil.ReadFile(mmStats)
	if err == nil {
		// mm_stat contains a list of unlabeled numbers representing
		// various zram-related quantities.  We are interested in the
		// first three such numbers.
		fields = strings.Fields(string(bytes))
		if len(fields) < 3 {
			return nil, errors.New(fmt.Sprintf("unexpected mm_stat content: %q", bytes))
		}
	} else {
		testing.ContextLogf(ctx, "Cannot read %v, assuming legacy device", mmStats)
		for _, fn := range []string{"orig_data_size", "compressed_data_size", "mem_used_total"} {
			b, err := ioutil.ReadFile(filepath.Join(zramDir, fn))
			if err != nil {
				return nil, err
			}
			fields = append(fields, string(b))
		}
	}

	var values []uint64
	for _, f := range fields {
		n, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot convert %q to uint64", f)
		}
		values = append(values, n)
	}
	return &ZramStatsData{
		Original:   values[0],
		Compressed: values[1],
		Used:       values[2],
	}, nil
}
