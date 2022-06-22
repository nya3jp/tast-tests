// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"reflect"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIVPDInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests chrome.os.telemetry.getVpdInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "target_models",
				Fixture:           "telemetryExtension",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				Fixture:           "telemetryExtension",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
			{
				Name:              "target_models_managed",
				Fixture:           "telemetryExtensionManaged",
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models_managed",
				Fixture:           "telemetryExtensionManaged",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
	})
}

// APIVPDInfo tests chrome.os.telemetry.getVpdInfo Chrome Extension API functionality.
func APIVPDInfo(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	want, err := fetchVPDInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get VPD info: ", err)
	}

	var resp vpdInfoResponse
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getVpdInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if !reflect.DeepEqual(resp, want) {
		s.Errorf("Unexpected VPD info: got %q; want %q", resp, want)
	}
}

type vpdInfoResponse struct {
	ActivateDate string `json:"activateDate"`
	ModelName    string `json:"modelName"`
	SerialNumber string `json:"serialNumber"`
	SkuNumber    string `json:"skuNumber"`
}

func fetchVPDInfo(ctx context.Context) (vpdInfoResponse, error) {
	activateDateBytes, err := ioutil.ReadFile("/sys/firmware/vpd/rw/ActivateDate")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to read ActivateDate VPD field")
	}
	if len(activateDateBytes) == 0 {
		return vpdInfoResponse{}, errors.New("ActivateDate VPD is empty")
	}

	modelNameBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/model_name")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to read model_name VPD field")
	}
	if len(modelNameBytes) == 0 {
		return vpdInfoResponse{}, errors.New("model_name VPD is empty")
	}

	serialNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/serial_number")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to read serial_number VPD field")
	}
	if len(serialNumberBytes) == 0 {
		return vpdInfoResponse{}, errors.New("serial_number VPD is empty")
	}

	skuNumberBytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/sku_number")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to read sku_number VPD field")
	}
	if len(skuNumberBytes) == 0 {
		return vpdInfoResponse{}, errors.New("sku_number VPD is empty")
	}

	return vpdInfoResponse{
		ActivateDate: string(activateDateBytes),
		ModelName:    string(modelNameBytes),
		SerialNumber: string(serialNumberBytes),
		SkuNumber:    string(skuNumberBytes),
	}, nil
}
