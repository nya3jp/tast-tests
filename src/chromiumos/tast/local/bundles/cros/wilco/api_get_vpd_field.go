// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetVpdField,
		Desc: "Test sending GetVpdField gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetVpdField(ctx context.Context, s *testing.State) {
	request := dtcpb.GetVpdFieldRequest{
		VpdField: dtcpb.GetVpdFieldRequest_FIELD_SERIAL_NUMBER,
	}
	response := dtcpb.GetVpdFieldResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetVpdField", &request, &response); err != nil {
		s.Fatal("Unable to get VPD field: ", err)
	}

	// Error conditions defined by the proto definition.
	if response.Status != dtcpb.GetVpdFieldResponse_STATUS_OK {
		s.Fatalf("Unable to get VPD field status: %s", response.Status)
	}

	if response.VpdFieldValue == "" {
		s.Fatal("VPD field value is empty")
	}

	s.Logf("GetVpdField FIELD_SERIAL_NUMBER %s", response.VpdFieldValue)
}
