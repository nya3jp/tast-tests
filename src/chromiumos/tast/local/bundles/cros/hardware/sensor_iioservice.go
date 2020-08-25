// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

const onErrorOccurred = "OnErrorOccurred:"
const succeedReadingSamples = "Number of success reads"

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioservice,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // Chrome OS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// SensorIioservice reads all devices' samples from daemon iioservice.
func SensorIioservice(ctx context.Context, s *testing.State) {
	var maxFreq int
	var strOut string

	// Call libmems' functions directly here to read and verify samples
	sensors, err := iio.GetSensors()
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, sn := range sensors {
		maxFreq = sn.MaxFrequency

		if sn.Name == "cros-ec-ring" {
			continue
		}

		frequency := fmt.Sprintf("--frequency=%f", float64(maxFreq)/1000)

		out, err := testexec.CommandContext(ctx, "iioservice_simpleclient",
			fmt.Sprintf("--device_id=%s", sn.Path[10:]), "--channels=timestamp",
			frequency).CombinedOutput()

		if err != nil {
			s.Error("Error reading samples on DUT: ", err)
		}

		strOut = string(out)
		if strings.Contains(strOut, onErrorOccurred) {
			s.Error("Too many failed readsamples on sensor: ", sn.Name)
		} else if !strings.Contains(strOut, succeedReadingSamples) {
			s.Error("Not enough successful readsamples on sensor: ", sn.Name)
		}
	}
}
