// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECAdc,
		Desc:         "Sanity check for EC ADC temperature",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECAdc(ctx context.Context, s *testing.State) {
	const (
		READ_COUNT  = 200
		MAX_EC_TEMP = 373
		MIN_EC_TEMP = 273
	)

	d := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// TODO: Determine how to check the necessity of this test by using
	// HasECCapability() as it seems like the helper.Config structure can
	// only be used internally by testing.AddTest()

	s.Log("Reading EC internal temperature for ", READ_COUNT, " times")
	for i := 1; i <= READ_COUNT; i++ {
		ecTemperatureOut, err := pxy.Servo().RunECCommandGetOutput(ctx, "temps", []string{`ECInternal\s+: (\d+) K`})
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
		if ecTemperature > MAX_EC_TEMP || ecTemperature < MIN_EC_TEMP {
			s.Fatal("Abnormal EC temperature: ", ecTemperature)
		}
	}
}
