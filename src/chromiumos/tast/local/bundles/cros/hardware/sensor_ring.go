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
		Desc: "Tests that all sensors in the cros-ec-ring can be enabled and read at their max frequency",
		Contacts: []string{
			"gwendal@chromium.com", // Chrome OS sensors point of contact
			"mathewk@chromium.org", // Test author
			"chromeos-sensors-eng@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func SensorRing(ctx context.Context, s *testing.State) {
	s.Log("Stop ui job to so sensors are not claimed by ARC")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	dutSensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	ring, err := iio.NewRing(dutSensors)
	if err != nil {
		s.Log("Sensor ring not found: ", err)
		return
	}

	readings, err := ring.Open(ctx)
	if err != nil {
		s.Fatal("Error opening ring: ", err)
	}
	defer ring.Close()

	for _, sn := range ring.Sensors {
		func() {
			start, err := iio.BootTime()
			if err != nil {
				s.Fatal("Error reading BootTime: ", err)
			}
			if err := sn.Enable(sn.Sensor.MaxFrequency, sn.Sensor.MaxFrequency); err != nil {
				s.Errorf("Error enabling sensor %v %v: %v",
					sn.Sensor.Location, sn.Sensor.Name, err)
				return
			}
			defer func() {
				if err := sn.Disable(); err != nil {
					s.Errorf("Error disabling sensor %v %v: %v",
						sn.Sensor.Location, sn.Sensor.Name, err)
				}
			}()

			var rs []*iio.SensorReading

			// Collect 2 seconds of data for each sensor.
			const collectTime = 2 * time.Second
			timeout := time.After(collectTime)
			for d := false; !d; {
				select {
				case r, ok := <-readings:
					if !ok {
						d = true
					} else if r.ID == sn.Sensor.ID {
						rs = append(rs, r)
					}
				case <-timeout:
					d = true
				case <-ctx.Done():
					s.Fatalf("Context closed while reading from sensor %v %v: %v",
						sn.Sensor.Location, sn.Sensor.Name, ctx.Err())
				}
			}
			end, err := iio.BootTime()
			if err != nil {
				s.Fatal("Error reading BootTime: ", err)
			}

			s.Logf("Got %v readings from %v %v",
				len(rs), sn.Sensor.Location, sn.Sensor.Name)

			if err := iio.Validate(rs, start, end, sn.Sensor, collectTime); err != nil {
				s.Fatal("Error validating data: ", err)
			}
		}()
	}
}
