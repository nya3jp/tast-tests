// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture contains fixtures meta tests use.
package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "metaRemote",
		Desc:     "Fixture for testing Tast's remote fixture support",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
		Impl:     &metaRemoteFixt{},
		Vars:     []string{"meta.metaRemote.SetUpError", "meta.metaRemote.TearDownError", "androidSerial", "phoneHost"},
		// Else fixture times out during setup
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type metaRemoteFixt struct{}

// FixtData is data returned from fixture to the test
type FixtData struct {
	PhoneIP string
}

func (*metaRemoteFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("SetUp metaRemote")
	if x, ok := s.Var("meta.metaRemote.SetUpError"); ok {
		s.Error(x)
	}

	// Read runtime vars. Global vars didnt work: https://b.corp.google.com/issues/217953452
	var androidSerial string
	if val, ok := s.Var("androidSerial"); ok {
		s.Log(val)
		androidSerial = val
	} else {
		s.Fatal("Failed to read Android serial")
	}
	var phoneHost string
	if val, ok := s.Var("phoneHost"); ok {
		s.Log(val)
		phoneHost = val
	} else {
		s.Fatal("Failed to read labstation hostname")
	}

	// setup connection to phone host by reusing ssh info from dut
	// I tried to put the details in ssh config but couldn't get that to work
	// passing localhost:1234 to Tast and to the runtime arg works fine
	d1 := s.DUT()
	s.Log("Connecting to Phone Host")
	d2, err := d1.ConnectPhoneHost(ctx, phoneHost)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}

	// Since the phones are connected to a separate phone host (currently a labstation, in the future a linux box) we have to ssh to the phone host to issue the commands for setting up touchless adb access and setting up adb-over-tcp. This means we cannot use the existing adb libraries in Tast :-/
	s.Logf("Setting up adb on %s with the correct permissions for touchless control", androidSerial)
	adbHome := "/tmp/adb_home/"
	arcKey := "/tmp/adb_home/arc.adb_key"
	if err := d2.CommandContext(ctx, "mkdir", "-p", adbHome).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create adb home: ", err)
	}
	if err := d2.CommandContext(ctx, "chmod", "0755", adbHome).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to chmod 0755 adb home: ", err)
	}
	if err := d2.CommandContext(ctx, "touch", arcKey).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to create arc++ key file: ", err)
	}
	if err := d2.CommandContext(ctx, "chmod", "0600", arcKey).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to cmhod arc++ key file: ", err)
	}
	privateKey := fmt.Sprintf("echo -n %q > %s", adb.ARCPrivateKey(), arcKey)
	if err := d2.CommandContext(ctx, "sh", "-c", privateKey).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to write ARC++ adb key to a file: ", err)
	}

	// Restart the adb-server now that the permissions are setup.
	s.Log("Restarting adb with the correct permssions")
	if err := d2.CommandContext(ctx, "adb", "kill-server").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to kill any running adb server: ", err)
	}

	// Pass the ADB_VENDOR_KEY are to the start-server command so no UI permission prompt is shown.
	cmdStrs := []string{
		"ADB_VENDOR_KEYS=/tmp/adb_home/arc.adb_key",
		"adb start-server",
	}
	if err := d2.CommandContext(ctx, "sh", "-c", strings.Join(cmdStrs, " ")).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to start adb with correct permissions: ", err)
	}

	// Print the existing devices attached to the phone host.
	out, err := d2.CommandContext(ctx, "adb", "devices").Output()
	if err != nil {
		s.Fatal("Failed to run adb devices")
	}
	s.Log(string(out))

	// Get the IP Address of the Android phone we want to setup adb-over-tcp with.
	out, err = d2.CommandContext(ctx, "adb", "-s", androidSerial, "shell", "ip", "route", "|", "awk", "'{print $9}'").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get Android phones IP Address")
	}
	phoneIPAddress := fmt.Sprintf("%s:%d", strings.TrimSpace(string(out)), 5555)
	s.Logf("Android IP Address is: %s", phoneIPAddress)

	// Enable adb-over-tcp on the Android device with the serial number we are using this test run.
	s.Log("Setting up adb-over-tcp on the Android device")
	if err = d2.CommandContext(ctx, "adb", "-s", androidSerial, "tcpip", "5555").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable adb-over-tcp on Android phone")
	}

	// Pass the IP Address from the fixture to the local test
	// Currently FixtValue() doesn't work: https://b.corp.google.com/issues/207607742
	// So write a text file to the local DUT and we will read it in the local test.
	phoneIPCmd := fmt.Sprintf("echo -n %q > %s", phoneIPAddress, "/tmp/ipaddress.txt")
	if err = d1.Conn().CommandContext(ctx, "sh", "-c", phoneIPCmd).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run: ", err)
	}

	// Save IP Address to fixture data for retrieval by local test.
	// Currently broken: https://b.corp.google.com/issues/207607742
	return &FixtData{
		PhoneIP: phoneIPAddress,
	}
}

func (*metaRemoteFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("TearDown metaRemote")
	if x, ok := s.Var("meta.metaRemote.TearDownError"); ok {
		s.Error(x)
	}
}

func (*metaRemoteFixt) Reset(ctx context.Context) error                        { return nil }
func (*metaRemoteFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*metaRemoteFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
