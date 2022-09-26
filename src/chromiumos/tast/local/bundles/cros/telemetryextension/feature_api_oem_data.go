// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FeatureAPIOEMData,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.telemetry.getOemData Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:telemetry_extension_hw"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.TelemetryExtension,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           fixture.TelemetryExtensionOverrideOEMName,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           fixture.TelemetryExtensionLacros,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           fixture.TelemetryExtensionOverrideOEMNameLacros,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

// FeatureAPIOEMData tests chrome.os.telemetry.getOemData Chrome Extension API functionality.
func FeatureAPIOEMData(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	want, err := oemData(ctx)
	if err != nil {
		s.Fatal("Failed to get OEM data: ", err)
	}

	type response struct {
		OemData string `json:"oemData"`
	}

	var resp response
	if err := v.ExtConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getOemData)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if got := resp.OemData; got != want {
		s.Errorf("Unexpected OEM data: got %q; want %q", got, want)
	}
}

func oemData(ctx context.Context) (string, error) {
	b, err := testexec.CommandContext(ctx, "/usr/share/cros/oemdata.sh").Output(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "oemdata.sh output: ", string(b))
		return "", errors.Wrap(err, "failed to get OEM data")
	}
	if len(b) == 0 {
		return "", errors.New("OEM data is empty")
	}
	return string(b), nil
}
