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
		Func: APIGetVPDFieldInvalid,
		Desc: "Test sending an invalid GetVpdField gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
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

func APIGetVPDFieldInvalid(ctx context.Context, s *testing.State) {
	request := dtcpb.GetVpdFieldRequest{
		VpdField: 100,
	}
	response := dtcpb.GetVpdFieldResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetVpdField", &request, &response); err != nil {
		s.Fatal("Unable to get VPD field: ", err)
	}

	if response.Status != dtcpb.GetVpdFieldResponse_STATUS_ERROR_VPD_FIELD_UNKNOWN {
		s.Fatalf("Unexpected status response; got %s, want STATUS_ERROR_VPD_FIELD_UNKNOWN", response.Status)
	}
}
