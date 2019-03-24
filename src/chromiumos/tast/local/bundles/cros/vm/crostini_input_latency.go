// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/vm/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniInputLatency,
		Desc:         "Tests Crostini input latency",
		Contacts:     []string{"cylee@chromium.org", "cros-containers-dev@google.com"},
		Data:         []string{"crostini_input_latency_server.py"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

// getTimeFromTimestamp expects a timestamp in string format like "1553789996.390980959" and
// returns an equivalent time.Time struct.
func getTimeFromTimestamp(ts string) (t time.Time, err error) {
	// time.ParseDuration turns the timestamp into a time.Duration. Add it to epoch time
	// to get a time.Time object.
	d, err := time.ParseDuration(ts + "s")
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to parse %q", ts)
	}
	return time.Unix(0, 0).Add(d), nil
}

// CrostiniInputLatency measures input latency to Crostini container.
// In the container side, it launches a xterm running a python script to wait for a key stroke.
// However, host clock and guest clock is not synced so we can't simply subtract the timestamps
// of key press event on host and key received event on guest.
// Instead, we launch a socket server in the guest VM. When a key arrives at the guest, the socket server
// sends a response back to host. We then subtract the timestamp host receives the response by (RTT time)/2
// as an estimation of key arrival time on guest.
func CrostiniInputLatency(ctx context.Context, s *testing.State) {
	// TODO(cylee): Consolidate container creation logic in a util function since it appears in multiple files.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	perfValues := perf.NewValues()
	defer perfValues.Save(s.OutDir())

	// Prepare error log file.
	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()

	// TOOD(cylee): Install it in container image.
	s.Log("Installing xterm")
	if _, err := perfutil.RunCmd(ctx, cont.Command(ctx, "sudo", "apt-get", "-y", "install", "xterm"), errFile); err != nil {
		s.Fatal("Failed to install xterm: ", err)
	}

	// Listen to evtest before sending keys.
	// Find keyboard device first.
	// TODO(cylee): Remove duplicated logic after module "input" supports needed features.
	findKeyboardDevice := func(out string) string {
		for _, device := range strings.Split(strings.TrimSpace(out), "\n\n") {
			if strings.Contains(device, "EV=120013") {
				handlersRegexp := regexp.MustCompile(`H: Handlers=.*\b(event\d+)\b`)
				matched := handlersRegexp.FindStringSubmatch(device)
				if matched != nil {
					return fmt.Sprintf("/dev/input/%s", matched[1])
				}
			}
		}
		return ""
	}
	const procInputDevicesFile = "/proc/bus/input/devices"
	listDevCmd := testexec.CommandContext(ctx, "cat", procInputDevicesFile)
	out, err := perfutil.RunCmd(ctx, listDevCmd, errFile)
	if err != nil {
		s.Fatal("Failed to list keyboard devivce: ", err)
	}
	keyboardDevice := findKeyboardDevice(string(out))
	if keyboardDevice == "" {
		perfutil.WriteError(ctx, errFile, procInputDevicesFile, out)
		s.Fatal("Can't find keyboard device")
	}
	s.Log("Found keyboard devices: ", keyboardDevice)

	// Setup evtest to monitor key events.
	evtestCmd := testexec.CommandContext(ctx, "evtest", keyboardDevice)
	evtestOut, err := evtestCmd.StdoutPipe()
	if err != nil {
		s.Error("Error getting evtest output: ", err)
	}
	if err := evtestCmd.Start(); err != nil {
		s.Fatal("Error starting evtest: ", err)
	}
	defer func() {
		if err := evtestCmd.Kill(); err != nil {
			s.Error("Error killing evtest: ", err)
		}
	}()
	keyEventChan := make(chan string)
	// evtest seems only starts monitoring key events after it prints out lines like
	//
	// Input driver version is 1.0.1
	// Input device ID: bus 0x11 vendor 0x1 product 0x1 version 0xab83
	// Input device name: "AT Translated Set 2 keyboard"
	// Supported events:
	//   Event type 0 (EV_SYN)
	//   Event type 1 (EV_KEY)
	// ...
	// Testing ... (interrupt to exit)
	//
	// So we shouldn't send keys before seeing the line "Testing ... (interrupt to exit)", otherwise
	// the event would be missed.
	evtestReady := make(chan int)
	go func() {
		evtestScanner := bufio.NewScanner(evtestOut)
		for evtestScanner.Scan() {
			line := evtestScanner.Text()
			if strings.HasPrefix(line, "Testing ... (interrupt to exit)") {
				s.Log("Evtest begins monitoring key events")
				evtestReady <- 1
				continue
			}

			// Waiting for key press event in the format like
			// Event: time 1553427907.227424, type 1 (EV_KEY), code 28 (KEY_ENTER), value 1
			enterPressedRegexp := regexp.MustCompile(
				`Event: time (\d*\.?\d+), type 1 \(EV_KEY\), code \d+ \(\S+\), value 1`)
			matched := enterPressedRegexp.FindStringSubmatch(line)
			if matched != nil {
				keyEventChan <- matched[1]
			}
		}
		if err := evtestScanner.Err(); err != nil {
			s.Error("Error reading evtest: ", err)
		}
		s.Log("stopped monitoring evest")
	}()

	s.Log("Waiting for evtest start monitoring key events")
	waitEvtestReadyTimeout := 3 * time.Second
	select {
	case <-evtestReady:
		s.Log("Evtest ready, continue testing")
	case <-time.After(waitEvtestReadyTimeout):
		s.Fatal("Evtest failed to monitor key events in v", waitEvtestReadyTimeout)
	case <-ctx.Done():
		s.Fatal("Context deadline exceeds: ", ctx.Err())
	}

	// Bring up a socket server in container.
	const containerHomeDir = "/home/testuser"
	const socketServerFileName = "crostini_input_latency_server.py"
	socketServerFilePath := filepath.Join(containerHomeDir, socketServerFileName)
	if err := cont.PushFile(ctx, s.DataPath(socketServerFileName), socketServerFilePath); err != nil {
		s.Fatalf("Failed to push %v to container: %v", socketServerFileName, err)
	}

	// The dynamic socket server port is communicated via a file. Remove the file before launching
	// the server. When it's accessible we know the server is ready.
	const portFile = "crostini_socket_server_port"
	_, err = perfutil.RunCmd(ctx, cont.Command(ctx, "rm", "-f", portFile), errFile)
	if err != nil {
		s.Error("Failed to remove stale socket server port file: ", err)
	}

	const socketServerLogName = "socket_server_log"
	socketServerLogPath := filepath.Join(containerHomeDir, socketServerLogName)
	socketServerArgs := []string{"xterm", "-e", fmt.Sprintf("/usr/bin/python %v >%v 2>&1", socketServerFilePath, socketServerLogPath)}
	socketServerCmd := cont.Command(ctx, socketServerArgs...)
	if err := socketServerCmd.Start(); err != nil {
		s.Error("Failed to start socket server in container: ", err)
	}
	defer func() {
		serverLogOutPath := filepath.Join(s.OutDir(), socketServerLogName)
		err := cont.GetFile(ctx, socketServerLogPath, serverLogOutPath)
		if err != nil {
			s.Error("Failed to copy socket server log: ", err)
		}
	}()

	// Setup a socket listener.
	containerIP, err := cont.GetIPv4Address(ctx)
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	s.Log("Container IP: ", containerIP)

	var socketServerPort string
	err = testing.Poll(ctx, func(ctx context.Context) error {
		getPortCmd := cont.Command(ctx, "cat", filepath.Join(containerHomeDir, portFile))
		out, err := perfutil.RunCmd(ctx, getPortCmd, errFile)
		if err != nil {
			return err
		}
		socketServerPort = string(out)
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second})
	if err != nil {
		s.Fatal("Failed to get socket server port: ", err)
	}
	s.Log("Socket server port: ", socketServerPort)

	var conn net.Conn
	conn, err = net.Dial("udp", containerIP+":"+socketServerPort)
	if err != nil {
		s.Fatal("Failed to connect to socket server: ", err)
	}
	defer func() {
		if _, err := conn.Write([]byte("exit")); err != nil {
			s.Error("Failed to terminate socket server: ", err)
		}
	}()

	socketChan := make(chan time.Time)
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err = conn.Read(buf)
			if err != nil {
				s.Error("Error reading socket: ", err)
			}
			socketChan <- time.Now()
		}
	}()

	// Ping socket server and returns the RTT time.
	pingGuest := func() (time.Duration, error) {
		waitSocketResponseTimeout := time.Second

		pingSendTime := time.Now()
		_, err = conn.Write([]byte("ping"))
		if err != nil {
			return 0, errors.Wrap(err, "failed to ping container")
		}

		select {
		case responseTime := <-socketChan:
			return responseTime.Sub(pingSendTime), nil
		case <-time.After(waitSocketResponseTimeout):
			return 0, errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	// Get rid of the initial ping since the initial ping time tends to be skewed.
	_, err = pingGuest()
	if err != nil {
		s.Error("Initial guest ping failed: ", err)
	}

	// Calculate average RTT time.
	var sumRTT time.Duration
	const numPing = 1000
	for i := 0; i < numPing; i++ {
		rtt, err := pingGuest()
		if err != nil {
			s.Error("Failed to ping socket server: ", err)
			continue
		}

		sumRTT += rtt
	}
	avgRTT := sumRTT / numPing
	s.Log("Avg. RTT time: ", avgRTT)

	keyWriter, err := input.Keyboard(ctx)
	if err != nil {
		s.Error("Failed to get keyboard event writer: ", err)
	}
	defer keyWriter.Close()

	measureKeyLatency := func() (time.Duration, error) {
		// Sends a key to guest.
		err = keyWriter.Type(ctx, "\n")
		if err != nil {
			s.Error("Failed to inject a key event: ", err)
		}

		// Get the time when a response is received from socket server.
		waitSocketResponseTimeout := time.Second
		var keyReceivedTime time.Time
		select {
		case keyReceivedTime = <-socketChan:
		case <-time.After(waitSocketResponseTimeout):
			return 0, errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		// Get key pressed timestamp.
		waitKeyTimeout := time.Second
		var keyPressedTimestamp string
		select {
		case keyPressedTimestamp = <-keyEventChan:
		case <-time.After(waitKeyTimeout):
			return 0, errors.Errorf("no key pressed in %v", waitKeyTimeout)
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		keyPressedTime, err := getTimeFromTimestamp(keyPressedTimestamp)
		if err != nil {
			s.Error("Failed to get key press timestamp: ", err)
		}

		// Estimate key received time on guest by subtracting (RTT time)/2.
		latency := keyReceivedTime.Add(-avgRTT / 2).Sub(keyPressedTime)

		return latency, nil
	}

	// Get rid of the initial measurement since it tends to be skewed.
	_, err = measureKeyLatency()
	if err != nil {
		s.Error("Initial key latency measurement failed: ", err)
	}

	const numMeasureKeyLatency = 100
	var sumKeyLatency time.Duration
	for i := 0; i < numMeasureKeyLatency; i++ {
		latency, err := measureKeyLatency()
		if err != nil {
			s.Error("Failed to measure key latency: ", err)
			continue
		}
		s.Logf("Key input latency (%v/%v): %v", i+1, numMeasureKeyLatency, latency)
		sumKeyLatency += latency
		perfValues.Append(
			perf.Metric{
				Name:      "crostini_input_latency",
				Variant:   "key_latency",
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			},
			perfutil.ToTimeUnit(time.Millisecond, latency)...)
	}
	avgKeyLatency := sumKeyLatency / numMeasureKeyLatency
	s.Log("Avg. key latency: ", avgKeyLatency)
}
