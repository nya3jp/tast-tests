// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sensors

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/sensors/crosconfig"
	"chromiumos/tast/local/bundles/cros/sensors/iio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Presence,
		Desc: "Tests that all sensors defined in model.yaml are present in the system",
		Contacts: []string{
			"mathewk@chromium.org", // Test author
		},
		SoftwareDeps: []string{"cros_config"},
		Attr:         []string{"informational"},
	})
}

// Presence verifies that sensors defined in a board's model.yaml file are
// defined as iio devices on the dut.
func Presence(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job to so sensors are not claimed by arc")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	dutSensors, err := iio.GetSensors()

	if err != nil {
		s.Fatal("Error reading sensors on dut: ", err)
	}

	findSensor := func(name iio.SensorName, loc iio.SensorLocation) (iio.Sensor, error) {
		var empty iio.Sensor

		for _, sensor := range dutSensors {
			if sensor.Name == name && sensor.Location == loc {
				return sensor, nil
			}
		}

		return empty, errors.New("Sensor not found")
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
	} {
		val, err := crosconfig.CheckHardwareProperty(ctx, tc.prop)

		if err != nil {
			s.Fatal("Failed to check property: ", err)
		}

		_, err = findSensor(tc.name, tc.location)

		if val && err != nil {
			s.Errorf("Expected sensor %v %v to be present on the dut but it is missing",
				tc.location, tc.name)
		}
	}
}
