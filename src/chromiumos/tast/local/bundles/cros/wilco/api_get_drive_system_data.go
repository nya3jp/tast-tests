// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetDriveSystemData,
		Desc: "Test sending GetDriveSystemData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetDriveSystemData(ctx context.Context, s *testing.State) {
	request := dtcpb.GetDriveSystemDataRequest{
		Type: dtcpb.GetDriveSystemDataRequest_SMART_ATTRIBUTES,
	}
	response := dtcpb.GetDriveSystemDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetDriveSystemData", &request, &response); err != nil {
		s.Fatal("Unable to get drive data: ", err)
	}

	if response.Status != dtcpb.GetDriveSystemDataResponse_STATUS_OK {
		s.Fatalf("Unexpected GetDriveSystemData response status; got %s, want STATUS_OK", response.Status)
	}

	if !strings.HasPrefix(string(response.Payload), "smartctl") {
		s.Error("Payload is not smartctl output")
	}

	if !strings.Contains(string(response.Payload), "START OF SMART DATA SECTION") {
		s.Error("Payload is not SMART data")
	}

	request = dtcpb.GetDriveSystemDataRequest{
		Type: dtcpb.GetDriveSystemDataRequest_IDENTITY_ATTRIBUTES,
	}
	response = dtcpb.GetDriveSystemDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetDriveSystemData", &request, &response); err != nil {
		s.Fatal("Unable to get drive data: ", err)
	}

	if response.Status != dtcpb.GetDriveSystemDataResponse_STATUS_OK {
		s.Fatalf("Unexpected GetDriveSystemData response status; got %s, want STATUS_OK", response.Status)
	}

	if !strings.HasPrefix(string(response.Payload), "NVME Identify Controller") {
		s.Error("Payload is not nvme id-ctrl output")
	}
}
