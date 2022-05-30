// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/hardware/iio"
	"chromiumos/tast/local/bundles/cros/hardware/simpleclient"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const clientDisconnected = "SensorHalClient disconnected"

func init() {
	testing.AddTest(&testing.Test{
		Func: SensorIioserviceReconnect,
		Desc: "Tests that iioservice provides sensors' samples properly",
		Contacts: []string{
			"gwendal@chromium.com",      // ChromeOS sensors point of contact
			"chenghaoyang@chromium.org", // Test author
			"chromeos-sensors@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"iioservice"},
	})
}

// SensorIioserviceReconnect reads devices' samples from iioservice, and
// re-bootstrap the mojo network upon ui's (mojo broker) restart.
func SensorIioserviceReconnect(ctx context.Context, s *testing.State) {
	var maxFreq int
	var strOut string

	// Call libmems' functions directly here to read and verify samples
	sensors, err := iio.GetSensors(ctx)
	if err != nil {
		s.Fatal("Error reading sensors on DUT: ", err)
	}

	for _, sn := range sensors {
		maxFreq = sn.MaxFrequency

		if sn.Name == iio.Ring {
			s.Error("Kernel must be compiled with USE=iioservice")
		}

		if sn.Name == iio.Activity || sn.Name == iio.Light {
			continue
		}

		frequency := fmt.Sprintf("--frequency=%f", float64(maxFreq)/1000)
		samples := fmt.Sprintf("--samples=%d", maxFreq/200)

		cmd := testexec.CommandContext(ctx, "iioservice_simpleclient",
			fmt.Sprintf("--device_id=%d", sn.IioID), "--channels=timestamp",
			"--disconnect_tolerance=1", frequency, samples)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			s.Error("Failed to create stderr pipe")
		}
		if err := cmd.Start(); err != nil {
			s.Error("Failed to start reading: ", err)
		}

		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}

		bytes, err := ioutil.ReadAll(stderr)
		if err != nil {
			s.Error("Failed to read from stderr")
		}

		if err := cmd.Wait(); err != nil {
			s.Error("Error reading samples on DUT: ", err)
		}

		strOut = string(bytes)
		if strings.Contains(strOut, simpleclient.OnErrorOccurred) {
			s.Error("OnErrorOccurred: ", sn.Name)
		} else if !strings.Contains(strOut, simpleclient.SucceedReadingSamples) {
			s.Error("Not enough successful readsamples on sensor: ", sn.Name)
		} else if !strings.Contains(strOut, clientDisconnected) {
			s.Error("Didn't record SensorHalClient disconnected: ", sn.Name)
		} else {
			s.Logf("Test passed on device name: %v, id: %v", sn.Name, sn.IioID)
		}
	}
}
