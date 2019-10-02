// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
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
			"chromeos-sensors@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func SensorRing(ctx context.Context, s *testing.State) {
	s.Log("Restarting ui job to so sensors are not claimed by ARC")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

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
			start, err := boottime()
			if err != nil {
				s.Fatal("Error reading boottime: ", err)
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
			end, err := boottime()
			if err != nil {
				s.Fatal("Error reading boottime: ", err)
			}

			s.Logf("Got %v readings from %v %v",
				len(rs), sn.Sensor.Location, sn.Sensor.Name)
			validate(rs, start, end, sn.Sensor, collectTime, s)
		}()
	}
}

func validate(rs []*iio.SensorReading, start, end time.Duration, sn *iio.Sensor, collectTime time.Duration, s *testing.State) {
	var expected int

	if sn.Name == iio.Light {
		// Light is on-change only. At worse, we may not see any sample if the light is very steady.
		expected = 0
	} else {
		// Expect that there are at least half the number of samples for the given frequency.
		expected = int(float64(sn.MaxFrequency)/1e3*collectTime.Seconds()) / 2
	}

	if len(rs) < expected {
		s.Errorf("Not enough data collected for %v %v with %.2f Hz in %v: got %v; expected at least %v",
			sn.Location, sn.Name, float64(sn.MaxFrequency)/1e3, collectTime, len(rs), expected)
	}

	last := start
	for ix, sr := range rs {
		if sr.Timestamp < last {
			s.Errorf("Timestamp out of order for %v %v at index %v: got %v; want >= %v",
				sn.Location, sn.Name, ix, sr.Timestamp, last)
		}

		last = sr.Timestamp
		if sr.Timestamp > end {
			s.Errorf("Timestamp in future for %v %v at index %v: got %v; want <= %v",
				sn.Location, sn.Name, ix, sr.Timestamp, end)
		}
	}
}

// boottime returns the duration from the boot time of the DUT to now.
func boottime() (time.Duration, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, errors.Wrap(err, "error reading boottime")
	}
	return time.Duration(ts.Nano()), nil
}
