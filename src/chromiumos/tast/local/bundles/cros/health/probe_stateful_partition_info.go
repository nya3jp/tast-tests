// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bufio"
	"context"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeStatefulPartitionInfo,
		Desc: "Checks that cros_healthd can fetch stateful partition info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

func ProbeStatefulPartitionInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryStatefulPartition}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get stateful partition telemetry info: ", err)
	}

	if len(records) < 2 {
		s.Fatalf("Wrong number of records: got %d; want 2", len(records))
	}

	// Verify the headers are correct.
	want := []string{"available_space", "total_space", "filesystem", "mount_source"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the values are correct.
	vals := records[1]
	if len(vals) != len(want) {
		s.Fatalf("Wrong number of values: got %d; want %d", len(vals), len(want))
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/mnt/stateful_partition", &stat); err != nil {
		s.Fatal("Failed to get disk stats for /mnt/stateful_partition: ", err)
	}

	realAvailable := stat.Bavail * uint64(stat.Bsize)
	margin := uint64(100000000) // 100MB
	realTotal := stat.Blocks * uint64(stat.Bsize)

	if available, err := strconv.ParseUint(vals[0], 10, 64); err != nil {
		s.Errorf("Failed to convert %q (available_space) to uint64: %v", vals[0], err)
	} else if absDiff(available, realAvailable) > margin {
		s.Errorf("Invalid available_space: got %v; want %v +- %v", available, realAvailable, margin)
	}

	if total, err := strconv.ParseUint(vals[1], 10, 64); err != nil {
		s.Errorf("Failed to convert %q (total_space) to uint64: %v", vals[1], err)
	} else if total != realTotal {
		s.Errorf("Invalid total_space: got %v; want %v", total, realTotal)
	}

	f, err := os.Open("/etc/mtab")
	if err != nil {
		s.Fatal("Failed to open /etc/mtab")
	}
	defer f.Close()

	var statefulPartitionInfo []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i != -1 {
			line = line[0:i]
		}
		fields := strings.Fields(line)
		if len(fields) != 6 {
			s.Errorf("Incorrect format in mtab: %q", sc.Text())
			continue
		}
		if fields[1] == "/mnt/stateful_partition" {
			statefulPartitionInfo = fields
		}
	}

	if statefulPartitionInfo == nil {
		s.Fatal("Failed to find stateful partition info in mtab")
	}
	if statefulPartitionInfo[2] != vals[2] {
		s.Fatalf("Wrong filesystem info: got %s; want %s", vals[2], statefulPartitionInfo[2])
	}
	if statefulPartitionInfo[0] != vals[3] {
		s.Fatalf("Wrong mount source info: got %s; want %s", vals[3], statefulPartitionInfo[0])
	}
}
