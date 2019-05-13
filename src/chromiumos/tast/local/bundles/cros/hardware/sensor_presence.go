// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"

	"chromiumos/tast/local/bundles/cros/hardware/crosconfig"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorPresence,
		Desc: "Tests that all sensors defined in model.yaml are present in the system",
		Contacts: []string{
			"gwendal@chromium.com", // Chrome OS sensors point of contact
			"mathewk@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		SoftwareDeps: []string{"cros_config"},
		Attr:         []string{"informational"},
	})
}

// SensorPresence verifies that sensors defined in a board's model.yaml file are
// defined as iio devices on the dut.
func SensorPresence(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job to so sensors are not claimed by arc")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	dutSensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on dut: ", err)
	}

	sensorDetected := func(name iio.SensorName, loc iio.SensorLocation) bool {
		for _, sensor := range dutSensors {
			if sensor.Name == name && sensor.Location == loc {
				return true
			}
		}

		return false
	}

	for _, tc := range []struct {
		prop     crosconfig.HardwareProperty
		name     iio.SensorName
		location iio.SensorLocation
	}{
		{crosconfig.HasBaseAccelerometer, iio.Accel, iio.Base},
		{crosconfig.HasBaseGyroscope, iio.Gyro, iio.Base},
		{crosconfig.HasBaseMagnetometer, iio.Mag, iio.Base},
		{crosconfig.HasLidAccelerometer, iio.Accel, iio.Lid},
		{crosconfig.HasLidGyroscope, iio.Gyro, iio.Lid},
		{crosconfig.HasLidMagnetometer, iio.Mag, iio.Lid},
	} {
		val, err := crosconfig.CheckHardwareProperty(ctx, tc.prop)

		if err != nil {
			s.Errorf("Failed to check property %v: %v", tc.prop, err)
			continue
		}

		hasSensor := sensorDetected(tc.name, tc.location)

		if val && !hasSensor {
			s.Errorf("Expected sensor %v %v to be present on the dut but it is missing",
				tc.location, tc.name)
		}

		if !val && hasSensor {
			s.Logf("Extra sensor %v %v; it should be added to the board's model.yaml",
				tc.location, tc.name)
		}
	}
}
