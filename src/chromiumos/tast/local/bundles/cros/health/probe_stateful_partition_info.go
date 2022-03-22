// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bufio"
	"context"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/local/spaced"
	"chromiumos/tast/testing"
)

type statefulPartitionInfo struct {
	AvailableSpace jsontypes.Uint64 `json:"available_space"`
	Filesystem     string           `json:"filesystem"`
	MountSource    string           `json:"mount_source"`
	TotalSpace     jsontypes.Uint64 `json:"total_space"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeStatefulPartitionInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch stateful partition info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

func validateStatefulPartitionData(ctx context.Context, statefulPartition *statefulPartitionInfo) error {
	statefulMount := "/mnt/stateful_partition"
	margin := int64(100000000) // 100MB

	spaced, err := spaced.NewClient(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create spaced client")
	}

	realAvailable, err := spaced.FreeDiskSpace(ctx, statefulMount)
	if err != nil {
		return errors.Wrap(err, "failed to query free disk space")
	}

	realTotal, err := spaced.TotalDiskSpace(ctx, statefulMount)
	if err != nil {
		return errors.Wrap(err, "failed to query total disk space")
	}

	if absDiff(int64(statefulPartition.AvailableSpace), realAvailable) > margin {
		return errors.Errorf("invalid available_space: got %v; want %v +- %v", statefulPartition.AvailableSpace, realAvailable, margin)
	}

	if int64(statefulPartition.TotalSpace) != realTotal {
		return errors.Errorf("invalid total_space: got %v; want %v", statefulPartition.TotalSpace, realTotal)
	}

	f, err := os.Open("/etc/mtab")
	if err != nil {
		return errors.Wrap(err, "failed to open /etc/mtab")
	}
	defer f.Close()

	var realStatefulPartitionInfo []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i != -1 {
			line = line[0:i]
		}
		fields := strings.Fields(line)
		if len(fields) != 6 {
			continue
		}
		if fields[1] == "/mnt/stateful_partition" {
			realStatefulPartitionInfo = fields
		}
	}

	if realStatefulPartitionInfo == nil {
		return errors.New("failed to find stateful partition info in mtab")
	}
	if realStatefulPartitionInfo[2] != statefulPartition.Filesystem {
		return errors.Errorf("Wrong filesystem info: got %s; want %s", statefulPartition.Filesystem, realStatefulPartitionInfo[2])
	}
	if realStatefulPartitionInfo[0] != statefulPartition.MountSource {
		return errors.Errorf("Wrong mount source info: got %s; want %s", statefulPartition.MountSource, realStatefulPartitionInfo[0])
	}

	return nil
}

func ProbeStatefulPartitionInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryStatefulPartition}
	var statefulPartition statefulPartitionInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &statefulPartition); err != nil {
		s.Fatal("Failed to get stateful partition telemetry info: ", err)
	}

	if err := validateStatefulPartitionData(ctx, &statefulPartition); err != nil {
		s.Fatalf("Failed to validate stateful partition data, err [%v]", err)
	}
}
