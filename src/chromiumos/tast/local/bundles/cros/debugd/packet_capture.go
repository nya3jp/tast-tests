// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PacketCapture,
		Desc: "Verifies network packet capture works and can be controlled by policy",
		Contacts: []string{
			"iremuguz@google.com", // Test author
			"chromeos-commercial-networking@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

func PacketCapture(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	dbg, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// policyVal is the policy value.
		policyVal *policy.DeviceDebugPacketCaptureAllowed
		// options to run packet capture with.
		options       map[string]dbus.Variant
		expectSuccess bool
	}{
		{
			name:      "unallowed_by_policy",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: false},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: false,
		},
		{
			name:      "successful_device_based_capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: true,
		},
		{
			name:          "empty_arguments",
			policyVal:     &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options:       map[string]dbus.Variant{},
			expectSuccess: false,
		},
		{
			name:      "wrong_device",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("fake_device"))},
			expectSuccess: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policyVal}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Create output file for packet capture operation.
			ofn := path.Join(s.OutDir(), "packet_capture_test.pcap")
			of, err := os.OpenFile(ofn, os.O_RDWR|os.O_CREATE, 0666)
			if err != nil {
				s.Fatal("Failed to create output file: ", err)
			}

			// Close and remove the created output file aftert the test ends.
			defer func(ctx context.Context, fn string, f *os.File) {
				f.Close()
				if err := os.Remove(fn); err != nil {
					s.Error("Failed to remove the output file : ", err)
				}
				os.Remove(fn)
			}(ctx, ofn, of)

			readStat, writeStat, err := os.Pipe()
			if err != nil {
				s.Fatal("Failed to create status pipe: ", err)
			}

			testing.ContextLog(ctx, "Making packet capture D-Bus call")
			handle, err := dbg.PacketCaptureStart(ctx, of, writeStat, param.options)

			if err == nil && !param.expectSuccess {
				// If there is no error returned when failure is expected, check the
				// status output of the call. dbugd's packet capture tool runs an
				// executable for packet capture operation and if there's an error
				// during execution, it doesn't return error but it writes on status
				// pipe. Status output will be non-empty in case of a failure.
				writeStat.Close()
				var buf bytes.Buffer
				io.Copy(&buf, readStat)
				if buf.String() == "" {
					s.Error("PacketCaptureStart succeeded when it's expected to fail")
				}
				return
			} else if err != nil && !param.expectSuccess {
				// An error is expected so test result is successful.
				return
			} else if err != nil && param.expectSuccess {
				s.Error("PacketCaptureStart DBus call failed to start packet capture process: ", err)
			}

			testing.ContextLog(ctx, "Performing simple network operation. This will take a few seconds")
			// Perform a network operation to capture the packets.
			_, err = testexec.CommandContext(ctx, "ping", "-c", "15", "www.google.com").Output()
			if err != nil {
				s.Error("Ping command failed: ", err)
			}

			testing.ContextLog(ctx, "Stopping packet capture")
			// Stop packet capture.
			if err := dbg.PacketCaptureStop(ctx, handle); err != nil {
				s.Error("PacketCaptureStop DBus call failed: ", err)
			}

			// Check the output file size.
			fi, err := os.Stat(ofn)
			if err != nil {
				s.Error("Can't get output file status information: ", err)
			}
			if fi.Size() == 0 {
				s.Error("Output file is empty. Couldn't capture any packets")
			}

		})
	}
}
