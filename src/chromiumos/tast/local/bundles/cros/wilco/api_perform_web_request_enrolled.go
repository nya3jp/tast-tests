// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIPerformWebRequestEnrolled,
		Desc: "Test sending PerformWebRequest to the Wilco DTC Support Daemon",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
			"bisakhmondal00@gmail.com",
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "wilcoDTCEnrolled",
	})
}

// APIPerformWebRequestEnrolled tests PerformWebRequest gRPC API.
func APIPerformWebRequestEnrolled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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

	for _, tc := range []struct {
		name       string
		url        string
		httpMethod dtcpb.PerformWebRequestParameter_HttpMethod
		wantStatus dtcpb.PerformWebRequestResponse_Status
		// Don't fail the test as the website may not be reachable from DUT. Instead just put a log.
		logOnly bool
	}{
		{
			name:       "chromium",
			url:        "https://bisakh.com",
			httpMethod: dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET,
			wantStatus: dtcpb.PerformWebRequestResponse_STATUS_OK,
			logOnly:    true,
		},
		{
			name:       "localhost",
			url:        "https://localhost/test",
			httpMethod: dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET,
			// Requests to localhost are blocked.
			wantStatus: dtcpb.PerformWebRequestResponse_STATUS_NETWORK_ERROR,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			response := dtcpb.PerformWebRequestResponse{}

			if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &dtcpb.PerformWebRequestParameter{
				HttpMethod: tc.httpMethod,
				Url:        tc.url,
			}, &response); err != nil {
				s.Fatal("Failed to call PerformWebRequest API method: ", err)
			}

			if response.Status != tc.wantStatus {
				if tc.logOnly {
					s.Logf("Request for %s failed with invalid status; got %s; expect %s", tc.url, response.Status, tc.wantStatus)
				} else {
					s.Errorf("Request for %s failed with invalid status; got %s; expect %s", tc.url, response.Status, tc.wantStatus)
				}
			}
		})
	}
}
