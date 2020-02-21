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
		Func: APIInvalidEnum,
		Desc: "Test sending invalid gRPC requests with enums out of range from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
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

func APIInvalidEnum(ctx context.Context, s *testing.State) {
	{
		request := dtcpb.GetDriveSystemDataRequest{
			Type: 100,
		}
		response := dtcpb.GetDriveSystemDataResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetDriveSystemData", &request, &response); err != nil {
			s.Fatal("Unable to get drive data: ", err)
		}

		if response.Status != dtcpb.GetDriveSystemDataResponse_STATUS_ERROR_REQUEST_TYPE_UNKNOWN {
			s.Errorf("Unexpected GetDriveSystemData response status; got %s, want STATUS_ERROR_REQUEST_TYPE_UNKNOWN", response.Status)
		}
	}

	{
		request := dtcpb.GetProcDataRequest{
			Type: 100,
		}
		response := dtcpb.GetProcDataResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetProcData", &request, &response); err != nil {
			s.Fatal("Unable to get Proc files: ", err)
		}

		if len(response.FileDump) != 0 {
			s.Errorf("Unexpected file dumps available: %s", response.String())
		}
	}

	{
		request := dtcpb.GetSysfsDataRequest{
			Type: 100,
		}
		response := dtcpb.GetSysfsDataResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetSysfsData", &request, &response); err != nil {
			s.Fatal("Unable to get Sysfs files: ", err)
		}

		if len(response.FileDump) != 0 {
			s.Errorf("Unexpected file dumps available: %s", response.String())
		}
	}

	{
		request := dtcpb.GetVpdFieldRequest{
			VpdField: 100,
		}
		response := dtcpb.GetVpdFieldResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetVpdField", &request, &response); err != nil {
			s.Fatal("Unable to get VPD field: ", err)
		}

		if response.Status != dtcpb.GetVpdFieldResponse_STATUS_ERROR_VPD_FIELD_UNKNOWN {
			s.Errorf("Unexpected status response; got %s, want STATUS_ERROR_VPD_FIELD_UNKNOWN", response.Status)
		}
	}

	{
		request := dtcpb.PerformWebRequestParameter{
			HttpMethod: 100,
			Url:        "https://google.com",
		}

		response := dtcpb.PerformWebRequestResponse{}

		if err := wilco.DPSLSendMessage(ctx, "PerformWebRequest", &request, &response); err != nil {
			s.Fatal("Unable to perform web request: ", err)
		}

		if response.Status != dtcpb.PerformWebRequestResponse_STATUS_ERROR_REQUIRED_FIELD_MISSING {
			s.Errorf("Unexpected GetRoutineUpdate response status; got %s, want STATUS_ERROR_REQUIRED_FIELD_MISSING", response.Status)
		}
	}
}
