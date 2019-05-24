// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"math"

	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorAccel,
		Desc: "Tests that accelerometer sensors can be read and give sane data",
		Contacts: []string{
			"gwendal@chromium.com", // Chrome OS sensors point of contact
			"mathewk@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		SoftwareDeps: []string{"cros_config"},
		Attr:         []string{"informational"},
	})
}

const accel1g = 9.8185
const accelErr = accel1g * .25

// SensorAccel
func SensorAccel(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job to so sensors are not claimed by arc")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	dutSensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on dut: ", err)
	}

	for _, sensor := range dutSensors {
		if sensor.Name != iio.Accel {
			continue
		}

		reading, err := sensor.Reading()
		if err != nil {
			s.Errorf("Error reading data from %v %v", sensor.Location, sensor.Name)
		}

		mag := math.Sqrt(reading.X*reading.X + reading.Y*reading.Y + reading.Z*reading.Z)

		if math.Abs(mag-accel1g) > accelErr {
			s.Errorf("Accelerometer %v data out of range, expected %v +- %v got %v",
				sensor.Path, accel1g, accelErr, mag)
		}
	}
}
