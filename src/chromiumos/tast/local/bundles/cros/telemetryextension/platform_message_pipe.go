// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformMessagePipe,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests message pipe functionality between PWA and Chrome extension",
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

// PlatformMessagePipe tests that PWA and Chrome extension have a capability to communicate with each other.
func PlatformMessagePipe(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	type telemetryRequest struct {
		InfoType string `json:"infoType"`
	}

	type request struct {
		Type      string           `json:"type"`
		Telemetry telemetryRequest `json:"telemetry"`
	}

	type response struct {
		Success   bool        `json:"success"`
		Telemetry interface{} `json:"telemetry"`
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var resp response
	if err := v.PwaConn.Call(ctx, &resp,
		"tast.promisify(chrome.runtime.sendMessage)",
		v.ExtID,
		request{Type: "telemetry", Telemetry: telemetryRequest{InfoType: "vpd"}},
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if want := true; resp.Success != want {
		s.Errorf("Unexpected response success: got %t; want %t. Response telemetry: %v", resp.Success, want, resp.Telemetry)
	}
}
