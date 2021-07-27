// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

type fluffyPort struct {
	Port     int
	Device   string
	MaxPwrMW float64
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     FluffyInteropStd,
		Desc:     "Uses fluffy with the standard charger configuration to check that expected voltages are reached",
		Contacts: []string{"aaboagye@chromium.org"},
		Data:     []string{"fluffy_interop_std_config.json"},
		Vars:     []string{"ServodPort", "ServodHost", "ServodSSHPort"},
		VarDeps:  []string{"usbc.MaxPwrReqMW"},
		// This test requires a specific setup and due to the availability of fluffy, is a manual test.
	})
}

func FluffyInteropStd(c context.Context, s *testing.State) {
	// Obtain the servod host and port if provided.
	scfg, ok := s.Var("ServodHost")
	if !ok {
		scfg = "localhost"
	}
	servodPort, ok := s.Var("ServodPort")
	if ok {
		scfg = fmt.Sprintf("%s:%s", scfg, servodPort)
	}
	servodSSHPort, ok := s.Var("ServodSSHPort")
	if ok {
		scfg = fmt.Sprintf("%s:ssh:%s", scfg, servodSSHPort)
	}

	// Retrieve the maximum power that the DUT will request.
	maxPwrReq := s.RequiredVar("usbc.MaxPwrReqMW")
	maxPwr, err := strconv.ParseFloat(maxPwrReq, 32)
	if err != nil {
		s.Fatalf("Max Power Request (mW) %q was not a valid number", maxPwrReq)
	}
	s.Logf("Assuming DUT will request a max of %s mW", maxPwrReq)

	// Read in the standard fluffy deployment config.
	s.Log("Reading standard config file")
	cfgfile, err := ioutil.ReadFile(s.DataPath("fluffy_interop_std_config.json"))
	if err != nil {
		s.Fatal("Failed to read standard config file: ", err)
	}

	var cfg []fluffyPort
	if err := json.Unmarshal(cfgfile, &cfg); err != nil {
		s.Fatal("Error decoding JSON file: ", err)
	}

	dut := s.DUT()

	// Setup a servo host connected to fluffy.
	s.Logf("Setting up connection to servod at %s", scfg)
	pxy, err := servo.NewProxy(c, scfg, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servod: ", err)
	}
	defer pxy.Close(c)

	fluffy := pxy.Servo()

	// For each charger, enable it, wait a bit and verify that the expected voltage is reached (with some margin).
	for p := range cfg {
		s.Logf("Connecting port %d (%s)", p, cfg[p].Device)
		// Call to servod to enable the port.
		if err := fluffy.SetActChgPort(c, strconv.Itoa(cfg[p].Port)); err != nil {
			s.Error("Could not set active charge port: ", err)
			continue
		}

		// Wait 1.5 seconds to allow the DUT to select the right PDO; this should be plenty of time.
		//
		// TODO(b/140571237) The way it stands is okay for a manual test. The more robust thing to do here
		// would be to wait until the CC line is idle (with some timeout).  However, in order to do that we
		// must have a way to sniff the CC lines.  Additionally, we could also have a servo flex connected to
		// the DUT, but that increases the setup required instead of simply plugging in the DUT.  In order for
		// this test to be run continuously in the future, this call should change to testing.Poll().
		testing.Sleep(c, 1500*time.Millisecond)

		// Verify that the voltage reached is as expected.
		voltageMV, err := fluffy.DUTVoltageMV(c)
		if err != nil {
			s.Error("Failed to read DUT voltage: ", err)
			continue
		}
		measuredVoltageMV, err := strconv.ParseFloat(voltageMV, 32)
		if err != nil {
			s.Error("Failed to convert string to float64: ", err)
			continue
		}

		// Limit the max power to either what the DUT can request or what the charger supports.
		var expectedPwrMW float64
		if maxPwr > cfg[p].MaxPwrMW {
			expectedPwrMW = cfg[p].MaxPwrMW
		} else {
			expectedPwrMW = maxPwr
		}

		// Select the correct voltage level depending upon the maximum power.  All PD compliant chargers should satisfy this.
		var expectedVoltageMV float64
		if expectedPwrMW > 45000 {
			expectedVoltageMV = 20000
		} else if expectedPwrMW > 27000 {
			expectedVoltageMV = 15000
		} else if expectedPwrMW > 15000 {
			expectedVoltageMV = 9000
		} else {
			expectedVoltageMV = 5000
		}

		// Set the min and max voltage per the USB PD spec.
		maxExpectedVoltageMV := expectedVoltageMV * 1.05
		minExpectedVoltageMV := (0.95 * expectedVoltageMV) - 750

		// Fluffy seems to read a bit high (b/140393065), move the entire range up by 300mV.
		maxExpectedVoltageMV += 300
		minExpectedVoltageMV += 300

		// Additionally, add a 200mV margin; this is wide but should still generally work.
		maxExpectedVoltageMV += 200
		minExpectedVoltageMV -= 200

		if (measuredVoltageMV > maxExpectedVoltageMV) || (measuredVoltageMV < minExpectedVoltageMV) {
			s.Errorf("Unexpected voltage with port %d (%s): got %.3fV, want %.3fV < x < %.3fV", p, cfg[p].Device, measuredVoltageMV/1000,
				minExpectedVoltageMV/1000, maxExpectedVoltageMV/1000)
		}
	}

	// Turn off the ports when finished testing.
	fluffy.SetActChgPort(c, "off")
}
