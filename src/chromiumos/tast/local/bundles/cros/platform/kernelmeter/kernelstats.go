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

// PSIMemoryLines returns a snapshot of /proc/pressure/memory as a list of
// lines, or nil if PSI is not available on the system.
func PSIMemoryLines() ([]string, error) {
	const psiFile = "/proc/pressure/memory"
	if _, err := os.Stat(psiFile); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	bytes, err := ioutil.ReadFile(psiFile)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(bytes), "\n")
	// Example of /proc/pressure/memory content:
	// some avg10=0.00 avg60=0.00 avg300=0.00 total=1468431930
	// full avg10=0.00 avg60=0.00 avg300=0.00 total=658624525
	if len(lines) != 3 {
		return nil, errors.New(fmt.Sprintf("unexpected PSI file content: %q", bytes))
	}
	return lines[:2], nil
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
