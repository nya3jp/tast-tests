// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome/ash"
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
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
			of, err := ioutil.TempFile("", "test.pcap")
			if err != nil {
				s.Fatal("Failed to create output file: ", err)
			}
			if err = os.Chmod(of.Name(), 0666); err != nil {
				s.Fatal("Failed to make output file writable for packet capture: ", err)
			}

			// Remove the created output file aftert the test ends.
			defer func() {
				if err := os.Remove(of.Name()); err != nil {
					s.Error("Failed to remove the output file : ", err)
				}
			}()

			readPipe, writePipe, err := os.Pipe()
			if err != nil {
				s.Fatal("Failed to create status pipe: ", err)
			}
			defer func() {
				writePipe.Close()
				readPipe.Close()
			}()

			isCapturing := true
			testing.ContextLog(ctx, "Making packet capture D-Bus call")
			handle, err := dbg.PacketCaptureStart(ctx, of, writePipe, param.options)
			defer func() {
				if !isCapturing {
					return
				}

				// Stop packet capture.
				if err := dbg.PacketCaptureStop(ctx, handle); err != nil {
					s.Error("PacketCaptureStop DBus call failed: ", err)
				}
			}()

			if err == nil && !param.expectSuccess {
				// If there is no error returned when failure is expected, check the
				// status output of the call. dbugd's packet capture tool runs an
				// executable for packet capture operation and if there's an error
				// during execution, it doesn't return error but it writes on status
				// pipe. Status output will be non-empty in case of a failure.
				writePipe.Close()
				readPipe.SetDeadline(time.Now().Add(15 * time.Second))

				testing.ContextLog(ctx, "Reading error from packet capture")
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, readPipe); err != nil {
					s.Fatal("Failed to read error: ", err)
				}
				if buf.String() == "" {
					s.Error("PacketCaptureStart succeeded when it's expected to fail")
				}
				return
			} else if err != nil && !param.expectSuccess {
				// An error is expected so test result is successful.
				return
			} else if err != nil && param.expectSuccess {
				s.Fatal("PacketCaptureStart DBus call failed to start packet capture process: ", err)
			}

			testing.ContextLog(ctx, "Performing simple network operation. This will take a few seconds")
			// Perform a network operation to capture the packets.
			err = testexec.CommandContext(ctx, "ping", "-c", "15", "www.google.com").Run()
			if err != nil {
				s.Error("Ping command failed: ", err)
			}

			// Notification ID of the packet capture notification. It is hard-coded in Chrome as
			// DebugdNotificationHandler::kPacketCaptureNotificationId.
			notificationID := "debugd-packetcapture"

			// A notification must be shown after packet capture starts successfully.
			testing.ContextLog(ctx, "Checking if packet capture notification is visible")
			if _, err = ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)); err != nil {
				s.Error("Packet capture notification is not visible")
			}

			testing.ContextLog(ctx, "Stopping packet capture")
			// Stop packet capture.
			if err := dbg.PacketCaptureStop(ctx, handle); err != nil {
				s.Error("PacketCaptureStop DBus call failed: ", err)
			} else {
				isCapturing = false
			}

			// Notification must be gone after the packet capture is stopped.
			testing.ContextLog(ctx, "Checking if packet capture notification is gone")
			if ash.WaitUntilNotificationGone(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)) != nil {
				s.Error("Notification isn't gone after stopping packet capture")
			}

			// Check the output file size.
			fi, err := os.Stat(of.Name())
			if err != nil {
				s.Fatal("Can't get output file status information: ", err)
			}
			if fi.Size() == 0 {
				s.Error("Output file is empty. Couldn't capture any packets")
			}
		})
	}
}
