// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"io/ioutil"
	"path"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

// getVPDFieldDataParam is the parameter to the APIGetVPDField test.
type getVPDFieldDataParam struct {
	// typeField is sent as the request type to GetVpdField.
	typeField dtcpb.GetVpdFieldRequest_VpdField
	// fileName is the name of the file in /sys/firmware/vpd/ro/ to compare the returned value to.
	fileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetVPDField,
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
		Params: []testing.Param{{
			Name: "serial",
			Val: getVPDFieldDataParam{
				typeField: dtcpb.GetVpdFieldRequest_FIELD_SERIAL_NUMBER,
				fileName:  "serial_number",
			},
		}},
	})
}

func APIGetVPDField(ctx context.Context, s *testing.State) {
	param := s.Param().(getVPDFieldDataParam)

	request := dtcpb.GetVpdFieldRequest{
		VpdField: param.typeField,
	}
	response := dtcpb.GetVpdFieldResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetVpdField", &request, &response); err != nil {
		s.Fatal("Unable to get VPD field: ", err)
	}

	if response.Status != dtcpb.GetVpdFieldResponse_STATUS_OK {
		s.Fatalf("Unexpected status response; got %s, want STATUS_OK", response.Status)
	}

	fileContent, err := ioutil.ReadFile(path.Join("/sys/firmware/vpd/ro/", param.fileName))
	if err != nil {
		s.Fatalf("Unable to read VPD file %s: %v", param.fileName, err)
	}

	expectedData := string(fileContent)

	if response.VpdFieldValue != expectedData {
		s.Fatalf("Unexpected VPD field value; got %s, want %s", response.VpdFieldValue, expectedData)
	}
}
