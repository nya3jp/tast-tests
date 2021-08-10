// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"syscall"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetStatefulPartitionAvailableCapacity,
		Desc: "Test sending GetStatefulPartitionAvailableCapacity gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"rbock@google.com", // Test author
			"bisakhmondal00@gmail.com",
			"pmoy@chromium.org", // wilco_dtc_supportd author
			"lamzin@google.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco", "chrome"},
		Timeout:      10 * time.Minute,
		// TODO(bisakh): Create a fixture for wilco DTC.
		Fixture: "chromeEnrolledLoggedIn",
	})
}

func APIGetStatefulPartitionAvailableCapacity(ctx context.Context, s *testing.State) {
	// absDiff performs math.Abs for int32 data type.
	absDiff := func(a, b int32) int32 {
		if a > b {
			return a - b
		}
		return b - a
	}

	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Start wilco DTC vm & corresponding DTC daemon.
	if err := wilco.StartVM(ctx, &wilco.VMConfig{
		StartProcesses: false,
		TestDBusConfig: false,
	}); err != nil {
		s.Fatal("Unable to start the Wilco DTC VM: ", err)
	}
	defer wilco.StopVM(cleanupCtx)
	if err := wilco.StartSupportd(ctx); err != nil {
		s.Fatal("Unable to start the Wilco DTC Support Daemon: ", err)
	}
	defer wilco.StopSupportd(cleanupCtx)

	pb := fakedms.NewPolicyBlob()
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users.
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// After this point, IsUserAffiliated flag should be updated.
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
	// that IsUserAffiliated flag is updated and policy handler is triggered.
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})

	// After this point, the policy handler should be triggered.
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// Test the policy behaviour.
	resp := dtcpb.GetStatefulPartitionAvailableCapacityResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetStatefulPartitionAvailableCapacity",
		&dtcpb.GetStatefulPartitionAvailableCapacityRequest{}, &resp); err != nil {
		s.Fatal("Failed to get stateful partition available capacity: ", err)
	}

	if resp.Status != dtcpb.GetStatefulPartitionAvailableCapacityResponse_STATUS_OK {
		s.Fatalf("Unexpected status received from vsh rpc method call. Status code: %d", resp.Status)
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
