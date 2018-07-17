// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ReconnectToDUT,
		Desc: "Demonstrates connecting to and disconnecting from DUT",
		Attr: []string{"informational"},
	})
}

func ReconnectToDUT(s *testing.State) {
	ctx := s.Context()
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}
	if !d.Connected(ctx) {
		s.Error("Not initially connected to DUT")
	}

	s.Log("Disconnecting from DUT")
	if err := d.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect from DUT: ", err)
	}
	if d.Connected(ctx) {
		s.Error("Still connected after disconnecting")
	}

	s.Log("Connecting to DUT")
	if err := d.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT: ", err)
	}
	if !d.Connected(ctx) {
		s.Error("Not connected after connecting")
	}

	// Leave the DUT in a disconnected state.
	// The connection should automatically be reestablished before the next test is run.
	s.Log("Disconnecting from DUT again")
	if err := d.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect from DUT: ", err)
	}
}
