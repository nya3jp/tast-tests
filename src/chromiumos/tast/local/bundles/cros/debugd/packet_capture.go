// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"io"
	"os"

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
		Desc: "Verifies network packet capture works as intended",
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
			name:      "Unallowed by policy",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: false},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: false,
		},
		{
			name:      "Successful device-based capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: true,
		},
		{
			name:          "Empty arguments",
			policyVal:     &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options:       map[string]dbus.Variant{},
			expectSuccess: false,
		},
		{
			name:      "Wrong device",
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
			ofn := "packet_capture_test.pcap"
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

			r, w, err := os.Pipe()
			if err != nil {
				s.Fatal("Failed to create status pipe: ", err)
			}

			stat := make(chan string)
			go func() {
				var buf bytes.Buffer
				io.Copy(&buf, r)
				stat <- buf.String()
			}()

			handle, err := dbg.PacketCaptureStart(ctx, of, w, param.options)
			testing.ContextLog(ctx, "Packet capture D-Bus call is made")

			if !param.expectSuccess && err == nil {
				// If there is no error returned when failure is expected, check the
				// status output of the call. dbugd's packet capture tool runs an
				// executable for packet capture operation and if there's an error
				// during execution, it doesn't return error but it writes on status
				// pipe. Status output will be non-empty in case of a failure.
				w.Close()
				status := <-stat
				close(stat)
				if status == "" {
					s.Error("PacketCaptureStart succeeded when it's expected to fail")
				}
			}

			if param.expectSuccess {
				if err != nil || handle == "" {
					s.Error("PacketCaptureStart DBus call failed to start packet capture process: ", err)
				}

				testing.ContextLog(ctx, "Performing simple network operation. This will take a few seconds")
				// Perform a network operation to capture the packets.
				_, err := testexec.CommandContext(ctx, "ping", "-c", "15", "www.google.com").Output()
				if err != nil {
					s.Error("Ping command failed: ", err)
				}

				// Stop packet capture.
				if err := dbg.PacketCaptureStop(ctx, handle); err != nil {
					s.Error("PacketCaptureStop DBus call failed: ", err)
				}
				testing.ContextLog(ctx, "Packet capture is stopped")

				// Check the output file size.
				fi, err := os.Stat(ofn)
				if err != nil {
					s.Error("Can't get output file status information: ", err)
				}
				if fi.Size() == 0 {
					s.Error("Output file is empty. Couldn't capture any packets")
				}
			}
		})
	}
}
