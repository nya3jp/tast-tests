// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/testing"
	//	"chromiumos/tast/remote/bundles/cros/meta/fixture"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalRemoteFixture,
		Desc:     "Tests local tests can depend on remote fixtures",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
		Fixture:  "metaRemote",
	})
}

func LocalRemoteFixture(ctx context.Context, s *testing.State) {
	// Getting fixture value from remote fixture causes panic in the fixture when using the
	// DUT library: Error at usercode.go:28: Panic: DUT unavailable (running non-remote?)
	// When this line is commented out it works fine. This is not expected to work yet due
	// to https://b.corp.google.com/issues/207607742. However it is supposed to return nil
	// Instead it actually errors out.
	//phoneIP := s.FixtValue().(*fixture.FixtData).PhoneIP

	// Read IP Address of Android from a text file written in the fixture.
	ip, err := testexec.CommandContext(ctx, "cat", "/tmp/ipaddress.txt").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to read Android ip address")
	}
	phoneIP := string(ip)
	s.Logf("Phone IP is: %s", phoneIP)

	// Startup adb on the DUT with the correct permissions to bypass manual interaction with phone.
	if err := localadb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server")
	}

	// Connect to the adb-over-tcp DUT that was setup in the fixture.
	adbDevice, err := adb.Connect(ctx, phoneIP, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to connect to adb over wifi: ", err)
	}
	s.Log("Connected to remote Android device")

	// Wait for the Android device to be ready for use.
	if err := adbDevice.WaitForState(ctx, adb.StateDevice, 30*time.Second); err != nil {
		s.Fatal("Wait for state failed: ", err)
	}
	s.Log("Android device is authorized for use")

	// List adb devices info
	devices, err := adb.Devices(ctx)
	if err != nil {
		s.Fatal("Failed to get adb devices: ", err)
	}
	for _, d := range devices {
		state, err := d.State(ctx)
		if err != nil {
			s.Logf("Failed to get State for %s", d.Serial)
			continue
		}
		s.Logf("Android device serial: %s, state: %s", d.Serial, state)
	}

	// Check that we can use the adbDevice as we would the USB one. Get GMSCore version.
	gmsVersion, err := adbDevice.GMSCoreVersion(ctx)
	if err != nil {
		s.Fatal("Failed to get GMS Core version: ", err)
	}
	s.Logf("GMSCore version: %d", gmsVersion)
}
