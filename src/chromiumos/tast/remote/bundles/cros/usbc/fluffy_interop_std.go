// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"time"

	"chromiumos/tast/remote/servo"
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
		Vars:     []string{"usbc.MaxPwrReqMW"},
		Attr:     []string{"disabled", "informational"},
	})
}

func FluffyInteropStd(c context.Context, s *testing.State) {

	// Retrieve the maximum power that the DUT will request.
	maxPwrReq := s.RequiredVar("usbc.MaxPwrReqMW")
	s.Logf("Assuming DUT will request a max of %s mW", maxPwrReq)

	// Setup a servo host connected to fluffy.  This connects to the servod running on localhost:9999
	s.Log("Setting up connection to servod at localhost:9999")
	fluffy, err := servo.Default(c)
	if err != nil {
		s.Error("Failed to connect to servod: ", err)
	}

	// Read in the standard fluffy deployment config.
	s.Log("Reading standard config file")
	cfgfile, err := ioutil.ReadFile(s.DataPath("fluffy_interop_std_config.json"))
	if err != nil {
		s.Fatal("Failed to read standard config file: ", err)
	}

	var cfg []fluffyPort
	err = json.Unmarshal(cfgfile, &cfg)
	if err != nil {
		s.Fatal("Error decoding JSON file: ", err)
	}

	// For each charger, enable it, wait a bit and verify that the expected voltage is reached (with some margin).
	for p := range cfg {
		s.Logf("Connecting port %d (%s)", p, cfg[p].Device)
		// Call to servod to enable the port.
		_, err = fluffy.SetActChgPort(c, strconv.Itoa(cfg[p].Port))
		if err != nil {
			s.Error("Could not set active charge port: ", err)
		}

		// Wait 1.5 seconds to allow the DUT to select the right PDO; this should be plenty of time.
		testing.Sleep(c, 1500*time.Millisecond)

		// Verify that the voltage reached is as expected.
		voltageMV, err := fluffy.DutVoltageMV(c)
		if err != nil {
			s.Error("Failed to read DUT voltage: ", err)
		}
		measuredVoltageMV, err := strconv.ParseFloat(voltageMV, 32)
		if err != nil {
			s.Error("Failed to convert string to float64: ", err)
		}

		// Limit the max power to either what the DUT can request or what the charger supports.
		var maxPwr, expectedVoltageMV float64
		maxPwr, err = strconv.ParseFloat(maxPwrReq, 32)
		if err != nil {
			s.Error("Failed to convert string to float64: ", err)
		}
		if maxPwr > cfg[p].MaxPwrMW {
			maxPwr = cfg[p].MaxPwrMW
		}

		// Select the correct voltage level depending upon the maximum power.  All PD compliant chargers should satisfy this.
		if maxPwr > float64(45000) {
			expectedVoltageMV = float64(20000)
		} else if maxPwr > float64(27000) {
			expectedVoltageMV = float64(15000)
		} else if maxPwr > float64(15000) {
			expectedVoltageMV = float64(9000)
		} else {
			expectedVoltageMV = float64(5000)
		}

		// Set the min and max voltage per the USB PD spec.
		maxExpectedVoltageMV := expectedVoltageMV * 1.05
		minExpectedVoltageMV := (0.95 * expectedVoltageMV) - float64(750)

		// Fluffy seems to read a bit high (b/140393065), move the entire range up by 300mV.
		maxExpectedVoltageMV += float64(300)
		minExpectedVoltageMV += float64(300)

		// Additionally, add a 200mV margin; this is wide but should still generally work.
		maxExpectedVoltageMV += float64(200)
		minExpectedVoltageMV -= float64(200)

		if (measuredVoltageMV > maxExpectedVoltageMV) || (measuredVoltageMV < minExpectedVoltageMV) {
			s.Errorf("FAIL! [Port %d: %s] (Measured %.3fV. Should be %.3fV < x < %.3fV)", p, cfg[p].Device, measuredVoltageMV/float64(1000),
				minExpectedVoltageMV/float64(1000), maxExpectedVoltageMV/float64(1000))
		} else {
			s.Log("PASS")
		}
	}

	// Turn off the ports when finished testing.
	_, err = fluffy.SetActChgPort(c, "off")
	if err != nil {
		s.Log("Failed to turn off all ports: ", err)
	}
}
