// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorRing,
		Desc: "Tests that",
		Contacts: []string{
			"gwendal@chromium.com", // Chrome OS sensors point of contact
			"mathewk@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		SoftwareDeps: []string{"cros_config"},
		Attr:         []string{"informational"},
	})
}

// SensorRing does.
func SensorRing(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job to so sensors are not claimed by ARC")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	dutSensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	ring, err := iio.NewSensorRing(dutSensors)
	if err != nil {
		s.Log("Sensor ring not found")
		return
	}

	readings, err := ring.Open()
	if err != nil {
		s.Fatalf("Error opening ring: ", err)
	}
	defer ring.Close()

	for id, sn := range ring.Sensors {
		var curRead []iio.SensorReading
		s.Logf("Setting freq to %v %v", sn.MaxFrequency, sn.MaxFrequency)
		ring.Collect(id, sn.MaxFrequency, sn.MaxFrequency)

		timeout := time.After(2 * time.Second)
	T:
		for {
			select {
			case r := <-readings:
				if r.ID == id {
					curRead = append(curRead, r)
				}
			case <-timeout:
				break T
			}
		}

		ring.Collect(id, 0, 0)
		s.Logf("Got %v readings from %v", len(curRead), id)
	}
}
