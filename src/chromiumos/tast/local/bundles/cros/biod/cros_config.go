// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package biod

import (
	"context"

	"chromiumos/tast/local/crosconfig"
	fp "chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosConfig,
		Desc: "Checks that the fingerprint cros-config is reasonable.",
		Contacts: []string{
			"hesling@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr: []string{"group:mainline", "informational", "group:fingerprint-cq"},
		// We omit any dependencies on fingerprint so that we can ensure that
		// the the fingerprint cros-config section is always reasonable.
	})
}

// CrosConfig checks that the /fingerprint section of cros-config is reasonable.
func CrosConfig(ctx context.Context, s *testing.State) {
	sensorLocation, err := crosconfig.Get(ctx, "/fingerprint", "sensor-location")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to invoke cros_config for sensor-location: ", err)
	}
	// When FP is not supported, sensor-location is not-defined or "none".
	// We check for "" in other parts of the code, but that is just a shortcut
	// for not-defined. Let's be strict with this rule here.
	if crosconfig.IsNotFound(err) ||
		fp.FPSensorLoc(sensorLocation) == fp.FPSensorLocNone {
		s.Log("Fingerprint is properly unsupported on device. Finishing.")
		return
	}
	if !fp.FPSensorLoc(sensorLocation).IsValid() {
		s.Fatal("Failed to invoke cros_config for board: ", err)
	}

	board, err := crosconfig.Get(ctx, "/fingerprint", "board")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to invoke cros_config for board: ", err)
	}
	if crosconfig.IsNotFound(err) {
		s.Fatal("Config /fingerprint board is missing.")
	}
	if !fp.FPBoardName(board).IsValid() {
		s.Fatal("Config /fingerprint board \"", board, "\" is invalid.")
	}

	sensorType, err := crosconfig.Get(ctx, "/fingerprint", "fingerprint-sensor-type")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to invoke cros_config for sensor_type: ", err)
	}
	if crosconfig.IsNotFound(err) {
		s.Log("Not using ", fp.FPSensorTypeStandAlone, " for fingerprint-sensor-type.")
		return
	}
	if !fp.FPSensorType(sensorType).IsValid() {
		s.Fatal("Config /fingerprint fingerprint-sensor-type \"",
			sensorType, "\" is invalid.")
	}

	// Additional Logic

	if fp.FPSensorLoc(sensorLocation) == fp.FPSensorLocPowerButtonTopLeft {
		if fp.FPSensorType(sensorType) != fp.FPSensorTypeOnPowerButton {
			s.Fatal("Sensor location is on power button, but type \"",
				sensorType,
				"\" doesn't match.")
		}
	}

}
