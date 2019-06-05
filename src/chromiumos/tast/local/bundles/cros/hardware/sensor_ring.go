// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

/*
#include <time.h>

typedef struct timespec timespec;
*/
import "C"

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

const collectSec = 2

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
		var rs []iio.SensorReading
		start := boottime()
		ring.Collect(id, sn.MaxFrequency, sn.MaxFrequency)

		timeout := time.After(collectSec * time.Second)
		for d := false; !d; {
			select {
			case r, ok := <-readings:
				if !ok {
					d = true
				} else if r.ID == id {
					rs = append(rs, r)
				}
			case <-timeout:
				d = true
			}
		}
		end := boottime()

		ring.Collect(id, 0, 0)
		s.Logf("Got %v readings from %v %v", len(rs), sn.Location, sn.Name)
		validate(rs, start, end, sn, s)
	}
}

func validate(rs []iio.SensorReading, start, end int64, sn *iio.Sensor, s *testing.State) {
	expected := int(float64(sn.MaxFrequency) / 1e3 * collectSec)

	if len(rs) < expected/2 {
		s.Errorf("Not enough data collected for %v %v with %.2f Hz in %v seconds: got %v; expected at least %v",
			sn.Location, sn.Name, float64(sn.MaxFrequency)/1e3, collectSec, len(rs), expected/2)
	}

	last := start
	for ix, sr := range rs {
		if sr.Timestamp < last {
			s.Errorf("Timestamp out of order for %v %v at index %v: got %v; want >=%v",
				sn.Location, sn.Name, ix, sr.Timestamp, last)
		}

		last = sr.Timestamp
		if sr.Timestamp > end {
			s.Errorf("Timestamp in future for %v %v at index %v: got %v; want <=%v",
				sn.Location, sn.Name, ix, sr.Timestamp, end)
		}
	}
}

func boottime() int64 {
	var tv C.timespec
	C.clock_gettime(C.CLOCK_BOOTTIME, &tv)

	return int64(tv.tv_nsec) + int64(tv.tv_sec)*1e9
}
