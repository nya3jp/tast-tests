// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECADC,
		Desc:         "Basic check for EC ADC temperature",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

// ECADC mesaures the EC internal temperature sensors in a loop for
// couple of retries. This test might fail on boards which don't have
// "temps" EC command available.
func ECADC(ctx context.Context, s *testing.State) {
	const (
		// Repeat read count
		readCount = 200
		// Maximum sensible EC temperature (in Kelvins)
		maxECTemp = 373
		// Minimum sensible EC temperature (in Kelvins)
		minECTemp = 273
	)

	h := s.PreValue().(*pre.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to send 'chan 0' to EC: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to send 'chan 0xffffffff' to EC: ", err)
		}
	}()

	s.Logf("Reading EC internal temperature for %d times", readCount)
	for i := 1; i <= readCount; i++ {
		ecTemperatureOut, err := h.Servo.RunECCommandGetOutput(ctx, "temps", []string{`ECInternal\s+: (\d+) K`})
		if err != nil {
			s.Fatal("Failed to read EC internal temperature temperature: ", err)
		}
		ecTemperatureStr := ecTemperatureOut[0].([]interface{})[1].(string)
		ecTemperature, err := strconv.ParseInt(ecTemperatureStr, 10, 64)
		if err != nil {
			s.Fatalf("Failed to parse EC internal temperature (%s) as int: %s",
				ecTemperatureStr,
				err)
		}
		if ecTemperature > maxECTemp || ecTemperature < minECTemp {
			s.Fatal("Abnormal EC temperature: ", ecTemperature)
		}
	}
}
