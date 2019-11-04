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
		Func: APIGetSysfsDataInvalid,
		Desc: "Test sending an invalid GetSysfsData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetSysfsDataInvalid(ctx context.Context, s *testing.State) {
	request := dtcpb.GetSysfsDataRequest{
		Type: 100,
	}
	response := dtcpb.GetSysfsDataResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
		s.Fatal("Unable to get Sysfs files: ", err)
	}

	if len(response.FileDump) != 0 {
		s.Fatalf("Unexpected file dumps available: %s", response.String())
	}
}
