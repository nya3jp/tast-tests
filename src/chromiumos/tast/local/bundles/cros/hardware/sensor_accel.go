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

// SensorAccel gets the current sensor reading of all accelerometer sensors and
// verifies that the data is within 25% of 1g.
func SensorAccel(ctx context.Context, ts *testing.State) {
	const (
		accel1g  = 9.8185
		accelErr = accel1g * .25
	)

	ts.Log("Restarting ui job to so sensors are not claimed by ARC")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		ts.Fatal("Failed to restart ui job: ", err)
	}

	sensors, err := iio.GetSensors()
	if err != nil {
		ts.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, s := range sensors {
		if s.Name != iio.Accel {
			continue
		}

		r, err := s.Reading()
		if err != nil {
			ts.Errorf("Error reading data from %v %v: %v", s.Location, s.Name, err)
		}

		mag := math.Hypot(r.X, math.Hypot(r.Y, r.Z))
		ts.Logf("%v %v magnitude is %v", s.Location, s.Name, mag)

		if math.Abs(mag-accel1g) > accelErr {
			ts.Errorf("Accelerometer %v data out of range: got %v; expected %v +- %v",
				s.Path, mag, accel1g, accelErr)
		}
	}
}
