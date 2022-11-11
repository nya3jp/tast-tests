// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"time"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type inputInfo struct {
	TouchpadLibraryName string              `json:"touchpad_library_name"`
	TouchscreenDevices  []touchscreenDevice `json:"touchscreen_devices"`
}

type touchscreenDevice struct {
	InputDevice           inputDevice `json:"input_device"`
	TouchPoints           int         `json:"touch_points"`
	HasStylus             bool        `json:"has_stylus"`
	HasStylusGarageSwitch bool        `json:"has_stylus_garage_switch"`
}

type inputDevice struct {
	Name             string `json:"name"`
	ConnectionType   string `json:"connection_type"`
	PhysicalLocation string `json:"physical_location"`
	IsEnabled        bool   `json:"is_enabled"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeInputInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for input info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		BugComponent: "b:982097",
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		Timeout:      1 * time.Minute,
	})
}

func ProbeInputInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryInput}
	var input inputInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &input); err != nil {
		s.Fatal("Failed to get input telemetry info: ", err)
	}

	// In later CL, we'll get input device node information from Chrome.
	// With that, we may be able to do some verification of some fields.
	// In this CL, we only make sure that there is no crash when we fetch
	// input data.
}
