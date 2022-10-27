// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PacketCapture,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies network packet capture works and can be controlled by policy",
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
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
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
		// The size of the network operation the packets will be captured from.
		captureSizeMiBs int
		// Output file size limit. Zero if there's no size limit.
		// Use MiBs to be compatible with packet capture DBus inputs and make tests more readable.
		maxFileSizeMiBs int64
		// The number of packet capture operations that will be initiated by the test.
		numberOfCaptures int
		// shouldTerminate will be true if the test needs to stop the packet capture operations explicitly.
		// It'll be set to false if the packet capture operation will terminate itself due to an error or reaching file size limit.
		shouldTerminate bool
	}{
		{
			name:      "unallowed_by_policy",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: false},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess:    false,
			captureSizeMiBs:  5,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  false,
		},
		{
			name:      "successful_device_based_capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess:    true,
			captureSizeMiBs:  5,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  true,
		},
		{
			name:             "empty_arguments",
			policyVal:        &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options:          map[string]dbus.Variant{},
			expectSuccess:    false,
			captureSizeMiBs:  5,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  false,
		},
		{
			name:      "wrong_device",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("fake_device"))},
			expectSuccess:    false,
			captureSizeMiBs:  5,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  false,
		},
		{
			name:      "high_volume_capture",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess:    true,
			captureSizeMiBs:  500,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  true,
		},
		{
			name:      "max_size_option",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device":   (dbus.MakeVariant("lo")),
				"max_size": (dbus.MakeVariant(5))},
			expectSuccess:    true,
			captureSizeMiBs:  15,
			maxFileSizeMiBs:  5,
			numberOfCaptures: 1,
			shouldTerminate:  false,
		},
		{
			name:      "multiple_captures",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"device": (dbus.MakeVariant("lo"))},
			expectSuccess:    true,
			captureSizeMiBs:  5,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 5,
			shouldTerminate:  true,
		},
		{
			name:      "frequency_based",
			policyVal: &policy.DeviceDebugPacketCaptureAllowed{Val: true},
			options: map[string]dbus.Variant{
				"frequency": (dbus.MakeVariant("5220"))},
			expectSuccess:    true,
			captureSizeMiBs:  15,
			maxFileSizeMiBs:  0,
			numberOfCaptures: 1,
			shouldTerminate:  true,
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

			type packetCaptureDetails struct {
				// Process handle of packet capture process.
				handle string
				// The name of the file that packet capture output will be written into.
				outputFile string
				// Indicates the packet capture process is running and not stopped yet.
				isCapturing bool
			}

			testCaptures := make([]packetCaptureDetails, param.numberOfCaptures)

			// Stop the packet capture operations if they haven't yet when the test ends.
			defer func() {
				for i := 0; i < param.numberOfCaptures; i++ {
					if !testCaptures[i].isCapturing {
						continue
					}

					// Stop packet capture.
					if err := dbg.PacketCaptureStop(ctx, testCaptures[i].handle); err != nil {
						s.Error("PacketCaptureStop DBus call failed: ", err)
					}
				}
			}()

			// Start packet captures for testing.
			for i := 0; i < param.numberOfCaptures; i++ {
				// Create output file for packet capture operation.
				of, err := ioutil.TempFile("", fmt.Sprintf("test%d.*.pcap", i))
				if err != nil {
					s.Fatal("Failed to create output file: ", err)
				}
				if err = os.Chmod(of.Name(), 0666); err != nil {
					s.Fatal("Failed to make output file writable for packet capture: ", err)
				}

				// Remove the created output file after the test ends.
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

				testing.ContextLog(ctx, "Making packet capture D-Bus call")
				handle, err := dbg.PacketCaptureStart(ctx, of, writePipe, param.options)

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

				testCaptures[i] = packetCaptureDetails{handle: handle, outputFile: of.Name(), isCapturing: true}
			}

			// Notification ID of the packet capture notification. It is hard-coded in Chrome as
			// DebugdNotificationHandler::kPacketCaptureNotificationId.
			notificationID := "debugd-packetcapture"

			// A notification must be shown after packet captures start successfully.
			s.Log("Checking if packet capture notification is visible")
			if _, err = ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)); err != nil {
				s.Error("Packet capture notification is not visible")
			}

			// Perform network operation to capture packets.
			s.Log("Performing simple network operation. This will take a few seconds")
			sendNetworkDataToLocalhost(param.captureSizeMiBs, s)

			// Stop packet capture.
			if param.shouldTerminate {
				for i := 0; i < param.numberOfCaptures; i++ {
					// The notification should stay visible even after stopping some of the packet captures.
					if _, err = ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(notificationID)); err != nil {
						s.Error("Packet capture notification is not visible")
					}
					if err = dbg.PacketCaptureStop(ctx, testCaptures[i].handle); err != nil {
						s.Error("PacketCaptureStop DBus call failed: ", err)
					} else {
						testCaptures[i].isCapturing = false
					}
				}
			}

			// Notification must be gone after all packet captures are stopped.
			s.Log("Checking if packet capture notification is gone")
			if ash.WaitUntilNotificationGone(ctx, tconn, 10*time.Second, ash.WaitIDContains(notificationID)) != nil {
				s.Error("Notification isn't gone after stopping packet capture")
			}

			// Check output file sizes.
			for i := 0; i < param.numberOfCaptures; i++ {
				fi, err := os.Stat(testCaptures[i].outputFile)
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
				if param.maxFileSizeMiBs != 0 && fs > param.maxFileSizeMiBs {
					s.Error("Output file exceeded size limit")
				}
			}
		})
	}
}

// sendNetworkDataToLocalhost sends and reads <sizeMiBs> of network data to localhost.
func sendNetworkDataToLocalhost(sizeMiBs int, s *testing.State) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Listen from an available port in localhost.
	lstnr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		s.Error("Can't listen to tcp port: ", err)
	}
	defer lstnr.Close()

	go listener(lstnr, int64(sizeMiBs), &wg, s)
	go dialer(lstnr.Addr().String(), sizeMiBs, &wg, s)
	wg.Wait()
}

// listener listens to the tcp connection on given Listener and reads the data.
func listener(lstnr net.Listener, sizeMiBs int64, wg *sync.WaitGroup, s *testing.State) {
	defer wg.Done()

	conn, err := lstnr.Accept()
	if err != nil {
		s.Error("Can't create connection to listen localhost port: ", err)
	}
	defer conn.Close()

	// Read the data in the socket.
	n, err := io.Copy(ioutil.Discard, conn)
	if err != nil {
		s.Error("Can't read data from localhost tcp port: ", err)
		return
	}
	// Compare the number of bytes read with the data that's sent in tcp port.
	if n != (sizeMiBs * 1024 * 1024) {
		s.Error("Couldn't read all the data from the tcp port")
	}
}

// dialer sends <sizeMiBs> of data to given tcp address to imitate network operation on device.
func dialer(address string, sizeMiBs int, wg *sync.WaitGroup, s *testing.State) {
	defer wg.Done()
	var dlr net.Dialer
	conn, err := dlr.Dial("tcp", address)
	if err != nil {
		s.Fatal("Can't dial localhost tcp address: ", err)
		return
	}
	defer conn.Close()
	chunk1KiB := make([]byte, 1024)
	mb := 1024 * sizeMiBs
	for i := 0; i < mb; i++ {
		if _, err := conn.Write(chunk1KiB); err != nil {
			s.Error("Can't write to localhost tcp address: ", err)
		}
	}
}
