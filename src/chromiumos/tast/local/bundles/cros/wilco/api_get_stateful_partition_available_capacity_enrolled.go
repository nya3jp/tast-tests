// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"syscall"

	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetStatefulPartitionAvailableCapacityEnrolled,
		Desc: "Test sending GetStatefulPartitionAvailableCapacity gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"rbock@google.com",  // Test author
			"lamzin@google.com", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
			"bisakhmondal00@gmail.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco", "chrome"},
		Fixture:      "wilcoDTCEnrolled",
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

	if resp.Status != dtcpb.GetStatefulPartitionAvailableCapacityResponse_STATUS_OK {
		// s.Log("STOP NOW")
		// time.Sleep(time.Minute)
		s.Fatalf("Unexpected status received from vsh rpc method call. Status code: %s", resp.Status)
	}

	// Fetch information about mounted filesystem through statfs syscall.
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/mnt/stateful_partition", &stat); err != nil {
		s.Fatal("Failed to get disk stats for the stateful partition: ", err)
	}

	realAvailableMb := int32(stat.Bavail * uint64(stat.Bsize) / uint64(1024) / uint64(1024))

	if resp.AvailableCapacityMb%int32(100) > 0 {
		s.Error("Invalid available capacity (not rounded to 100 MiB): ", resp.AvailableCapacityMb)
	}
	if absDiff(resp.AvailableCapacityMb, realAvailableMb) > int32(100) { // allowed error margin 100 MiB
		s.Errorf("Invalid available capacity: got %v; want %v +- %v", resp.AvailableCapacityMb, realAvailableMb, 100)
	}
}
