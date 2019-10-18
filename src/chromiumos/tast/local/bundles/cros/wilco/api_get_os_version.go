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
		Func: APIGetOsVersion,
		Desc: "Test GetOsVersion in WilcoDtcSupportd",
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

func APIGetOsVersion(ctx context.Context, s *testing.State) {
	res, err := common.SetupSupportdForAPITest(ctx, s)
	ctx = res.TestContext
	defer common.TeardownSupportdForAPITest(res.CleanupContext, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	osMsg := dtcpb.GetOsVersionRequest{}
	osRes := dtcpb.GetOsVersionResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetOsVersion", &osMsg, &osRes); err != nil {
		s.Fatal("Unable to get OS version: ", err)
	}

	// Error conditions defined by the proto definition.
	if len(osRes.Version) == 0 {
		s.Fatal(errors.Errorf("OS Version is blank: %s", osRes.String()))
	}
	if osRes.Milestone == 0 {
		s.Fatal(errors.Errorf("OS Milestone is 0: %s", osRes.String()))
	}
}
