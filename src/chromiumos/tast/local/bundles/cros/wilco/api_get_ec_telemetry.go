// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/common"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetEcTelemetry,
		Desc: "Test GetEcTelemetry in WilcoDtcSupportd",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func APIGetEcTelemetry(ctx context.Context, s *testing.State) {
	res, err := common.SetupSupportdForAPITest(ctx, s)
	ctx = res.TestContext
	defer common.TeardownSupportdForAPITest(res.CleanupContext, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	ecMsg := dtcpb.GetEcTelemetryRequest{}
	// Get EC firmware label following kernel driver
	// https://chromium.googlesource.com/chromiumos/third_party/kernel/+/d145cca29f845e55e353cbb86fa9391a71f71dbb/drivers/platform/chrome/wilco_ec/sysfs.c?pli=1#48
	ecMsg.Payload = []byte{0x38, 0x00, 0x00}
	ecRes := dtcpb.GetEcTelemetryResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetEcTelemetry", &ecMsg, &ecRes); err != nil {
		s.Fatal("Unable to get EC Telemetry: ", err)
	}

	if ecRes.Status != dtcpb.GetEcTelemetryResponse_STATUS_OK {
		s.Fatal(errors.Errorf(
			"unexpected EC telemetry response status: got %s, want GetEcTelemetryResponse_STATUS_OK", ecRes.Status))
	}
}
