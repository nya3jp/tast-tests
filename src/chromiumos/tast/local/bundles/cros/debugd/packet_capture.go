// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/policy"
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
		// The size in MiBs of the network operation the packets will be captured from.
		captureSize int
		// Output file size limit in MiBs. Zero if there's no size limit.
		// Use MiBs to be compatible with packet capture DBus inputs and make tests more readable.
		maxFileSize int64
	}{
		{
			name:      "unallowed_by_policy",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: false},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: false,
			captureSize:   5,
			maxFileSize:   0,
		},
		{
			name:      "successful_device_based_capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: true,
			captureSize:   5,
			maxFileSize:   0,
		},
		{
			name:          "empty_arguments",
			policyVal:     &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options:       map[string]dbus.Variant{},
			expectSuccess: false,
			captureSize:   5,
			maxFileSize:   0,
		},
		{
			name:      "wrong_device",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("fake_device"))},
			expectSuccess: false,
			captureSize:   5,
			maxFileSize:   0,
		},
		{
			name:      "high_volume_capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess: true,
			captureSize:   500,
			maxFileSize:   0,
		},
		{
			name:      "max_size_option",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device":   (dbus.MakeVariant("lo")),
				"max_size": (dbus.MakeVariant(5))},
			expectSuccess: true,
			captureSize:   15,
			maxFileSize:   5,
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
			defer func(ctx context.Context) {
				if err := os.Remove(of.Name()); err != nil {
					s.Error("Failed to remove the output file : ", err)
				}
			}(ctx)

			readPipe, writePipe, err := os.Pipe()
			if err != nil {
				s.Fatal("Failed to create status pipe: ", err)
			}

			handle, err := dbg.PacketCaptureStart(ctx, of, writePipe, param.options)

			if err == nil && !param.expectSuccess {
				// If there is no error returned when failure is expected, check the
				// status output of the call. dbugd's packet capture tool runs an
				// executable for packet capture operation and if there's an error
				// during execution, it doesn't return error but it writes on status
				// pipe. Status output will be non-empty in case of a failure.
				writePipe.Close()
				var buf bytes.Buffer
				io.Copy(&buf, readPipe)
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

			// Notification ID of the packet capture notification. It is hard-coded in Chrome as
			// DebugdNotificationHandler::kPacketCaptureNotificationId.
			notificationID := "debugd-packetcapture"

			// A notification must be shown after packet capture starts successfully.
			s.Log("Checking if packet capture notification is visible")
			if _, err = ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)); err != nil {
				s.Error("Packet capture notification is not visible")
			}

			// Perform network operation to capture packets.
			s.Log("Performing simple network operation. This will take a few seconds")
			var wg sync.WaitGroup
			wg.Add(2)
			port := "localhost:12121"
			go listener(port, &wg, s)
			go dialer(port, param.captureSize, &wg, s)
			wg.Wait()

			// Stop packet capture.
			if err = dbg.PacketCaptureStop(ctx, handle); err != nil {
				s.Error("PacketCaptureStop DBus call failed: ", err)
			}

			// Notification must be gone after the packet capture is stopped.
			s.Log("Checking if packet capture notification is gone")
			if ash.WaitUntilNotificationGone(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)) != nil {
				s.Error("Notification isn't gone after stopping packet capture")
			}

			// Check the output file size.
			fi, err := os.Stat(of.Name())
			if err != nil {
				s.Fatal("Can't get output file status information: ", err)
			}
			fs := fi.Size()
			if fs == 0 {
				s.Error("Output file is empty. Couldn't capture any packets")
				return
			}
			// Convert the file size into MiBs.
			fs = fs / (1024 * 1024)
			if param.maxFileSize != 0 && fs > param.maxFileSize {
				s.Error("Output file exceeded size limit")
			}
		})
	}
}

// listener listens to the tcp connection on given port.
func listener(port string, wg *sync.WaitGroup, s *testing.State) {
	defer wg.Done()

	lstnr, err := net.Listen("tcp", port)
	if err != nil {
		s.Error("Can't listen to tcp port: ", err)
	}
	defer lstnr.Close()

	conn, err := lstnr.Accept()
	if err != nil {
		s.Error("Can't create connection to listen localhost port: ", err)
	}
	defer conn.Close()

	// Read the data in the socket.
	var buf bytes.Buffer
	_, err = io.Copy(&buf, conn)
	if err != nil {
		s.Error("Can't read data from localhost tcp port: ", err)
	}
}

// dialer sends <size> MiBs of data to given tcp port to imitate network operation on device.
func dialer(port string, size int, wg *sync.WaitGroup, s *testing.State) {
	defer wg.Done()
	var dlr net.Dialer
	conn, err := dlr.Dial("tcp", port)
	if err != nil {
		s.Fatal("Can't dial localhost tcp port: ", err)
		return
	}
	defer conn.Close()
	mb := 1024 * size
	for i := 0; i < mb; i++ {
		if _, err := conn.Write(make([]byte, 1024)); err != nil {
			s.Error("Can't write to localhost tcp port: ", err)
		}
	}
}
