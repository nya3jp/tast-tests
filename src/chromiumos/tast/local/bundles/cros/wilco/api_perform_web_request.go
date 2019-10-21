// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wilco/common"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIPerformWebRequest,
		Desc: "Test PerformWebRequest in WilcoDtcSupportd",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco", "chrome"},
	})
}

func APIPerformWebRequest(ctx context.Context, s *testing.State) {
	res, err := common.SetupSupportdForAPITest(ctx, s)
	ctx = res.TestContext
	defer common.TeardownSupportdForAPITest(res.CleanupContext, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	crctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()

	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(crctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(crctx)

	wrMsg := dtcpb.PerformWebRequestParameter{}
	wrRes := dtcpb.PerformWebRequestResponse{}

	wrMsg.HttpMethod = dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET
	wrMsg.Url = "https://google.com"

	if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &wrMsg, &wrRes); err != nil {
		s.Fatal("Unable to perform web request: ", err)
	}

	if wrRes.Status != dtcpb.PerformWebRequestResponse_STATUS_OK {
		s.Fatalf("Status not STATUS_OK: %s", wrRes.Status.String())
	}
}
