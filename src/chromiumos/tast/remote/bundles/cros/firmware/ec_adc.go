// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECAdc,
		Desc:         "Sanity check for EC ADC temperature",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

// ECAdc mesaures the EC internal temperature sensors in a loop for
// couple of retries. This test might fail on boards which don't have
// "temps" EC command available.
func ECAdc(ctx context.Context, s *testing.State) {
	const (
		// Repeat read count
		ReadCount = 200
		// Maximum sensible EC temperature (in Kelvins)
		MaxECTemp = 373
		// Minimum sensible EC temperature (in Kelvins)
		MinECTemp = 273
	)

	h := s.PreValue().(*pre.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	h.Servo.RunECCommand("chan 0")
	s.Log("Reading EC internal temperature for ", ReadCount, " times")
	for i := 1; i <= ReadCount; i++ {
		ecTemperatureOut, err := h.Servo.RunECCommandGetOutput(ctx, "temps", []string{`ECInternal\s+: (\d+) K`})
		if err != nil {
			s.Fatal("Failed to read EC internal temperature temperature: ", err)
		}
		ecTemperatureStr := ecTemperatureOut[0].([]interface{})[1].(string)
		ecTemperature, err := strconv.ParseInt(ecTemperatureStr, 10, 64)
		if err != nil {
			s.Fatal(fmt.Sprintf("Failed to parse EC internal temperature (%d) as int: ",
				ecTemperatureStr,
				err))
		}
		if ecTemperature > MaxECTemp || ecTemperature < MinECTemp {
			s.Fatal("Abnormal EC temperature: ", ecTemperature)
		}
	}
}
