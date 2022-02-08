// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CapMessagePipe,
		LacrosStatus: testing.LacrosVariantNeeded,
		Fixture:      "telemetryExtension",
		Desc:         "Tests message pipe functionality between PWA and Chrome extension",
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
				ExtraHardwareDeps: dep.TargetModels(),
			},
			{
				Name:              "low_priority_target_models",
				ExtraHardwareDeps: dep.LowPriorityTargetModels(),
			},
		},
	})
}

// CapMessagePipe tests that PWA and Chrome extension have a capability to communicate with each other.
func CapMessagePipe(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	const messageID = 100

	type message struct {
		ID int `json:"id"`
	}

	var resp message
	if err := v.PwaConn.Call(ctx, &resp,
		"tast.promisify(chrome.runtime.sendMessage)",
		v.ExtID,
		message{ID: messageID},
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}

	if want := messageID + 1; resp.ID != want {
		s.Errorf("Unexpected response ID: got %d; want %d", resp.ID, want)
	}
}
