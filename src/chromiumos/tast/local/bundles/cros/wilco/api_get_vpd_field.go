// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetVPDField(ctx context.Context, s *testing.State) {
	getVPDField := func(vpd_field string) (string, error) {
		for _, dir := range []string{"ro/", "rw/"} {
			content, err := ioutil.ReadFile(filepath.Join("/sys/firmware/vpd/", dir, vpd_field))
			if err == nil {
				return string(content), nil
			}
		}
		return "", errors.New("vpd field does not exist")
	}

	for _, tc := range []struct {
		// grpcVpdField is sent as the request type to GetVpdField.
		grpcVpdField dtcpb.GetVpdFieldRequest_VpdField
		// vpdField is the name of VPD field.
		vpdField string
	}{
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_ACTIVATE_DATE,
			vpdField:     "ActivateDate",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_ASSET_ID,
			vpdField:     "asset_id",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_MANUFACTURE_DATE,
			vpdField:     "mfg_date",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_MODEL_NAME,
			vpdField:     "model_name",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_SERIAL_NUMBER,
			vpdField:     "serial_number",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_SKU_NUMBER,
			vpdField:     "sku_number",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_SYSTEM_ID,
			vpdField:     "system_id",
		},
		{
			grpcVpdField: dtcpb.GetVpdFieldRequest_FIELD_UUID_ID,
			vpdField:     "uuid_id",
		},
	} {
		s.Logf("Running gRPC API for getting %s VPD field", tc.vpdField)

		expectedValue, err := getVPDField(tc.vpdField)
		if err != nil {
			s.Logf("Ignoring %s VPD field, probably VPD field does not exist on the device: %v", tc.vpdField, err)
			continue
		}

		request := dtcpb.GetVpdFieldRequest{
			VpdField: tc.grpcVpdField,
		}
		response := dtcpb.GetVpdFieldResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetVpdField", &request, &response); err != nil {
			s.Errorf("Unable to perform gRPC request to get %s VPD field: %v", tc.vpdField, err)
			continue
		}

		if response.Status != dtcpb.GetVpdFieldResponse_STATUS_OK {
			s.Errorf("Unexpected status response; got %s, want STATUS_OK", response.Status)
			continue
		}

		if response.VpdFieldValue != expectedValue {
			s.Errorf("Unexpected VPD field value; got %s, want %s", response.VpdFieldValue, expectedValue)
			continue
		}
	}

}
