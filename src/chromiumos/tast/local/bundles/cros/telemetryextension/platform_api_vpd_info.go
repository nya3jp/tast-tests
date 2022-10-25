// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformAPIVPDInfo,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.telemetry.getVpdInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

// PlatformAPIVPDInfo tests chrome.os.telemetry.getVpdInfo Chrome Extension API functionality.
func PlatformAPIVPDInfo(ctx context.Context, s *testing.State) {
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
	activateDate, err := fetchOptionalVpdField("/sys/firmware/vpd/rw/ActivateDate")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to fetch ActivateDate VPD field")
	}

	modelName, err := fetchOptionalVpdField("/sys/firmware/vpd/ro/model_name")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to fetch model_name VPD field")
	}

	serialNumber, err := fetchOptionalVpdField("/sys/firmware/vpd/ro/serial_number")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to fetch serial_number VPD field")
	}

	skuNumber, err := fetchOptionalVpdField("/sys/firmware/vpd/ro/sku_number")
	if err != nil {
		return vpdInfoResponse{}, errors.Wrap(err, "failed to fetch sku_number VPD field")
	}

	return vpdInfoResponse{
		ActivateDate: activateDate,
		ModelName:    modelName,
		SerialNumber: serialNumber,
		SkuNumber:    skuNumber,
	}, nil
}

// fetchOptionalVpdField returns the value of an optional VPD field or the empty
// string if this VPD field does not exist. However it returns an error if the
// existing VPD field is empty.
func fetchOptionalVpdField(path string) (string, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", errors.Wrapf(err, "failed to check whether %q exists", path)
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %q", path)
	}
	if len(bytes) == 0 {
		return "", errors.Errorf("%s is empty", path)
	}
	return string(bytes), nil
}
