// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const failReadingEvents = "Too many failed readevents, stopping thread."
const succeedReadingEvents = "Number of success readevents"

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioservice,
		Desc: "Tests that iioservice provides sensors' events properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// SensorIioservice reads all devices' events from daemon iioservice.
func SensorIioservice(ctx context.Context, s *testing.State) {
	var maxFreq int
	var strOut string

	// Call libmems' functions directly here to read and verify events
	sensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, sn := range sensors {
		maxFreq = sn.MaxFrequency

		s.Logf("name: %s", sn.Name)
		if sn.Name == "cros-ec-ring" {
			continue
		}

		frequency := fmt.Sprintf("--frequency=%d", maxFreq)
		samplingFrequency := fmt.Sprintf("--sampling_frequency=%f",
			float64(maxFreq)/1000)
		out, err := testexec.CommandContext(ctx, "events_thread",
			fmt.Sprintf("--device_id=%s", sn.Path[10:]), "--channels=timestamp",
			frequency, samplingFrequency).CombinedOutput()

		if err != nil {
			s.Error("Error reading events on DUT: ", err)
		}

		strOut = string(out)
		if strings.Contains(strOut, failReadingEvents) {
			s.Error("Too many failed readevents on sensor: ", sn.Name)
		} else if !strings.Contains(strOut, succeedReadingEvents) {
			s.Error("Not enough success readevents on sensor: ", sn.Name)
		}
	}
}
