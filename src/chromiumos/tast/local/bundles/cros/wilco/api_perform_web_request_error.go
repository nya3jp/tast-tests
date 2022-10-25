// Copyright 2020 The ChromiumOS Authors
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
		Func: APIPerformWebRequestError,
		Desc: "Test sending PerformWebRequest gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon when not enrolled",
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

func APIPerformWebRequestError(ctx context.Context, s *testing.State) {
	request := dtcpb.PerformWebRequestParameter{
		HttpMethod: dtcpb.PerformWebRequestParameter_HTTP_METHOD_GET,
		Url:        "https://localhost/test",
	}
	response := dtcpb.PerformWebRequestResponse{}

	if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &request, &response); err != nil {
		s.Fatal("Unable to get configuration data: ", err)
	}

	// Error conditions defined by the proto definition.
	if response.Status != dtcpb.PerformWebRequestResponse_STATUS_INTERNAL_ERROR {
		s.Errorf("Unexpected Status; got %s, want STATUS_INTERNAL_ERROR", response.Status)
	}

	if len(response.ResponseBody) > 0 {
		s.Errorf("Unexpected ResponseBody; got %v, want an empty response", response.ResponseBody)
	}
}
