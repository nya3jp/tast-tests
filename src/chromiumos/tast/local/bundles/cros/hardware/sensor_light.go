// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"

	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorLight,
		Desc: "Tests that ambient light sensor can be read and give valid data",
		Contacts: []string{
			"gwendal@chromium.com", // ChromeOS sensors point of contact
			"chromeos-sensors-eng@google.com",
		},
		Attr: []string{"group:sensors"},
	})
}

// SensorLight gets the current sensor reading of all light sensors.
func SensorLight(ctx context.Context, s *testing.State) {
	sensors, err := iio.GetSensors(ctx)
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, sn := range sensors {
		if sn.Name != iio.Light {
			continue
		}

		r, err := sn.Read()
		if err != nil {
			s.Errorf("Error reading data from %v %v: %v", sn.Location, sn.Name, err)
			continue
		}

		// Light sensor may have one value or RGB three values.
		if len(r.Data) != 3 && len(r.Data) != 1 {
			s.Errorf("Got %v from %v %v; want 1 or 3 values", r.Data, sn.Location, sn.Name)
			continue
		}
	}
}
