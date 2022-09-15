// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIGetStatefulPartitionAvailableCapacityEnrolled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test sending GetStatefulPartitionAvailableCapacity gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"lamzin@google.com", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
			"rbock@google.com",
			"bisakhmondal00@gmail.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco", "chrome"},
		Fixture:      "wilcoDTCAllowed",
	})
}

// APIGetStatefulPartitionAvailableCapacityEnrolled tests GetStatefulPartitionAvailableCapacity gRPC API.
func APIGetStatefulPartitionAvailableCapacityEnrolled(ctx context.Context, s *testing.State) {
	// absDiff performs math.Abs for int32 data type.
	absDiff := func(a, b int32) int32 {
		if a > b {
			return a - b
		}
		return b - a
	}

	resp := dtcpb.GetStatefulPartitionAvailableCapacityResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetStatefulPartitionAvailableCapacity",
		&dtcpb.GetStatefulPartitionAvailableCapacityRequest{}, &resp); err != nil {
		s.Fatal("Failed to get stateful partition available capacity: ", err)
	}

	if want := dtcpb.GetStatefulPartitionAvailableCapacityResponse_STATUS_OK; resp.Status != want {
		s.Fatalf("Unexpected status received from vsh rpc method call = got %v, want %v", resp.Status, want)
	}

	// Fetch information about mounted filesystem through statfs syscall.
	var stat unix.Statfs_t
	if err := unix.Statfs("/mnt/stateful_partition", &stat); err != nil {
		s.Fatal("Failed to get disk stats for the stateful partition: ", err)
	}

	realAvailableMb := int32(stat.Bavail * uint64(stat.Bsize) / uint64(1024) / uint64(1024))

	if resp.AvailableCapacityMb%int32(100) > 0 {
		s.Error("Invalid available capacity (not rounded to 100 MiB): ", resp.AvailableCapacityMb)
	}
	if absDiff(resp.AvailableCapacityMb, realAvailableMb) > int32(100) { // allowed error margin 100 MiB
		s.Errorf("Invalid available capacity = got %v, want %v +- %v", resp.AvailableCapacityMb, realAvailableMb, 100)
	}
}
