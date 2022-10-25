// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetOsVersion,
		Desc: "Test sending GetOsVersion gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon",
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

func APIGetOsVersion(ctx context.Context, s *testing.State) {
	request := dtcpb.GetOsVersionRequest{}
	response := dtcpb.GetOsVersionResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetOsVersion", &request, &response); err != nil {
		s.Fatal("Unable to get OS version: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(response.Version) == 0 {
		s.Fatal(errors.Errorf("OS Version is blank: %s", response.String()))
	}
	if response.Milestone == 0 {
		s.Fatal(errors.Errorf("OS Milestone is 0: %s", response.String()))
	}
}
