// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strconv"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

type fluffyPort struct {
	Port int
	Device string
	Expected_voltage_mv int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     FluffyInteropStd,
		Desc:     "Uses fluffy with the standard charger configuration to check that expected voltages are reached.",
		Contacts: []string{"aaboagye@chromium.org"},
		Attr:     []string{"disabled"},
	})
}

func FluffyInteropStd(ctx context.Context, s *testing.State) {

	// Need to setup a servo host connected to fluffy.  This connects to the servod running on localhost:9999
	s.Log("Setting up connection to servod at localhost:9999")
	fluffy, err := servo.Default(ctx)
	if err != nil {
		s.Error(err)
	}
	s.Log("%v", fluffy)

	// Read in the standard fluffy deployment config.
	s.Log("Reading standard config file")
	cfgfile, err := ioutil.ReadFile("./data/fluffy_interop_std_config.json")
	if (err != nil) {
		s.Error("Failed to read config file. ", err)
	}

	// var cfg interface{}
	var cfg []fluffyPort
	err = json.Unmarshal(cfgfile, &cfg)
	if err != nil {
		s.Error("Error decoding JSON file: ", err)
	}
	s.Log("Got config:\n", cfg)
	// err = fluffy.ActChgPort(ctx, "6")
	// if (err != nil) {
		// s.Error(err)
	// }
	// _, err = fluffy.PowerNormalPress(ctx)

	for port := range cfg {
		s.Logf("port[%d] #%d", port, cfg[port].Device)
		var val string = strconv.Itoa(cfg[port].Port)
		_, err = fluffy.ActChgPort(ctx, val)
		if (err != nil) {
			s.Error(err)
		}
	}

	// ports := cfg.(map[string]interface{})
	// s.Log("ports: ", ports)
	// p := ports["ports"]
	// s.Log("p: ", p)
	// q := p.([]map[string]interface{})
	// for m, i := range .([]map[string] {
	// for m, i := range q {
	// 	m := p[i]
	// 	s.Logf("i: (%d)", i, m)
	// }

	// For each charger, enable it, wait a bit and verify that the expected voltage is reached (with some margin).

	//Call to servod to enable the port.
}
