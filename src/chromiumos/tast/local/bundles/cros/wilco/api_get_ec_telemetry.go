// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: APIGetEcTelemetry,
		Desc: "Test sending GetEcTelemetry gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetEcTelemetry(ctx context.Context, s *testing.State) {
	// Get EC firmware label from the following kernel driver
	// https://chromium.googlesource.com/chromiumos/third_party/kernel/+/d145cca29f845e55e353cbb86fa9391a71f71dbb/drivers/platform/chrome/wilco_ec/sysfs.c?pli=1#48
	// The first byte is CMD_EC_INFO (0x38), the second byte is always empty (0x00), and the third is the CMD_GET_EC_LABEL (0x00)
	request := dtcpb.GetEcTelemetryRequest{
		Payload: []byte{0x38, 0x00, 0x00},
	}
	response := dtcpb.GetEcTelemetryResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetEcTelemetry", &request, &response); err != nil {
		s.Fatal("Unable to get EC Telemetry: ", err)
	}

	if response.Status != dtcpb.GetEcTelemetryResponse_STATUS_OK {
		s.Fatal(errors.Errorf(
			"unexpected EC telemetry response status: got %s, want GetEcTelemetryResponse_STATUS_OK", response.Status))
	}
}
