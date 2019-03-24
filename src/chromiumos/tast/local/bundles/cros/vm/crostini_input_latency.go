// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/vm/perfutil"
	"chromiumos/tast/local/chrome"
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
		Data:         []string{"keyboard_enter", "crostini_input_latency_server.py"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func getTimeFromTimestamp(s string) (t time.Time, err error) {
	ts, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "Could not parse float64 from "+s)
	}
	return time.Unix(0, int64(ts*float64(time.Second))), nil
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// CrostiniInputLatency measures input latency to Crostini container.
// In the container side, it launches a xterm running a python script to wait for a key stroke.
// However, host clock and guest clock is not synced so we can't simply subtract the timestamp of key press event on
// host and key received timestamp on guest.
// Instead, we first try to calibrate the clock by sending pings to a socket server in container. The socket server always
// returns the local (guest) timestamp. After getting the timestamp, we estimate the clock difference between host and guest
// taking socket RTT time into consideration.
// After that, we emulate a key press and let the socket server returns key received time to host. Then host can
// calculate input latency using a calibrated key received time.
//
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

	s.Log("Installing xterm")
	if _, err := perfutil.RunCmd(ctx, cont.Command(ctx, "sudo", "apt-get", "-y", "install", "xterm"), errFile); err != nil {
		s.Fatal("Failed to install xterm: ", err)
	}

	// Listen to evtest before sending keys.
	// Find keyboard device first.
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
	// We need to make sure the evtest gorutine starts before sending a key, otherwise the key event may
	// be missed.
	var evtestReady sync.WaitGroup
	evtestReady.Add(1)
	go func() {
		evtestScaner := bufio.NewScanner(evtestOut)
		for evtestScaner.Scan() {
			line := evtestScaner.Text()
			if strings.HasPrefix(line, "Testing ... (interrupt to exit)") {
				s.Log("Evtest begins monitoring key events")
				evtestReady.Done()
				continue
			}

			// Waiting for key press event in the format like
			// Event: time 1553427907.227424, type 1 (EV_KEY), code 28 (KEY_ENTER), value 1
			enterPressedRegexp := regexp.MustCompile(
				`Event: time (\d*\.?\d+), type 1 \(EV_KEY\), code 28 \(KEY_ENTER\), value 1`)
			matched := enterPressedRegexp.FindStringSubmatch(line)
			if matched != nil {
				keyEventChan <- matched[1]
			}
		}
		if err := evtestScaner.Err(); err != nil {
			s.Error("Error reading evtest: ", err)
		}
		s.Log("stopped monitoring evest")
	}()
	s.Log("Waiting for evtest goroutine")
	evtestReady.Wait()
	s.Log("Evtest goroutine ready, continue testing")

	// Bring up a socket server in container.
	const containerHomeDir = "/home/testuser"
	const socketServerFileName = "crostini_input_latency_server.py"
	socketServerFilePath := filepath.Join(containerHomeDir, socketServerFileName)
	if err := cont.PushFile(ctx, s.DataPath(socketServerFileName), socketServerFilePath); err != nil {
		s.Fatalf("Failed to push %v to container: %v", socketServerFileName, err)
	}
	const socketServerLogName = "socket_server_log"
	socketServerLogPath := filepath.Join(containerHomeDir, socketServerLogName)
	socketServerArgs := []string{"xterm", "-e", fmt.Sprintf("python %v >%v 2>&1", socketServerFilePath, socketServerLogPath)}
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

	// Need some time for the server to be ready, otherwise would get a "connection refused" error when
	// trying to connect to it.
	if err := Sleep(ctx, time.Second); err != nil {
		s.Fatal("Context deadline exceeded: ", err)
	}

	// Setup a socket listener.
	containerIP, err := cont.GetIPv4Address(ctx)
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	s.Log("Container IP: ", containerIP)
	const containerPort = "12346"
	conn, err := net.Dial("tcp", containerIP+":"+containerPort)
	if err != nil {
		s.Fatal("Failed to connect to socket server: ", err)
	}
	defer func() {
		if _, err := conn.Write([]byte("exit")); err != nil {
			s.Error("Failed to terminate socket server: ", err)
		}
	}()
	socketChan := make(chan string)
	go func() {
		socketScanner := bufio.NewScanner(conn)
		for socketScanner.Scan() {
			res := socketScanner.Text()
			s.Log("received ", res)
			socketChan <- res
		}
		if err := socketScanner.Err(); err != nil {
			s.Error("Error reading socket: ", err)
		}
		s.Log("stopped listen to socket server")
	}()

	// Ping socket server to get current timestamp in guest.
	getGuestTime := func() (ts string, err error) {
		waitSocketResponseTimeout := time.Second
		_, err = conn.Write([]byte("ping"))
		if err != nil {
			return "", errors.Wrap(err, "failed to ping container")
		}
		select {
		case ts := <-socketChan:
			return ts, nil
		case <-time.After(waitSocketResponseTimeout):
			return "", errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		}
	}
	const numClockSync = 3
	var sumRTT, sumTimeDelta time.Duration
	for i := 0; i < numClockSync; i++ {
		s.Logf("Sending a ping to server (%v/%v)", i+1, numClockSync)

		pingBegin := time.Now()
		ts, err := getGuestTime()
		if err != nil {
			s.Error("Failed to ping socket server: ", err)
			continue
		}
		rtt := time.Since(pingBegin)

		// Try to estimate the host time when guest receives the ping by adding half of RTT time.
		estimatedHostTime := pingBegin.Add(rtt / 2)
		guestTime, err := getTimeFromTimestamp(ts)
		if err != nil {
			s.Error("Failed to parse guest tiemstamp: ", err)
		}
		timeDelta := estimatedHostTime.Sub(guestTime)
		s.Logf("RTT time: %v, time delta (host - guest): %v", rtt, timeDelta)
		sumRTT += rtt
		sumTimeDelta += timeDelta
	}
	avgRTT := sumRTT / numClockSync
	avgTimeDelta := sumTimeDelta / numClockSync
	s.Log("Avg. RTT time: ", avgRTT)
	s.Log("Avg. time delta (host - guest): ", avgTimeDelta)

	measureKeyLatency := func() (time.Duration, error) {
		// Adds some random cool down time.
		if err := Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Context deadline exceeded: ", err)
		}

		s.Log("Sending an enter key to container")
		sendKeyCmd := testexec.CommandContext(ctx, "evemu-play", keyboardDevice)
		keyboardFile, err := os.Open(s.DataPath("keyboard_enter"))
		if err != nil {
			return 0, errors.Wrap(err, "unable to open keybaord playback file")
		}
		defer keyboardFile.Close()
		sendKeyCmd.Stdin = keyboardFile
		_, err = perfutil.RunCmd(ctx, sendKeyCmd, errFile)
		if err != nil {
			return 0, errors.Wrap(err, "failed to send key event")
		}

		// Get key received time from socket server.
		waitSocketResponseTimeout := time.Second
		var keyReceivedTime time.Time
		select {
		case ts := <-socketChan:
			keyReceivedTime, err = getTimeFromTimestamp(ts)
			if err != nil {
				return 0, errors.Wrap(err, "failed to get key received time")
			}
		case <-time.After(waitSocketResponseTimeout):
			return 0, errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		}

		// Get key pressed timestamp.
		waitKeyTimeout := time.Second
		var keyPressedTime time.Time
		select {
		case keyPressedTimestamp := <-keyEventChan:
			keyPressedTime, err = getTimeFromTimestamp(keyPressedTimestamp)
			if err != nil {
				return 0, errors.Wrap(err, "failed to get key pressed time")
			}
		case <-time.After(waitKeyTimeout):
			return 0, errors.Errorf("no key pressed in %v", waitKeyTimeout)
		}
		latency := keyReceivedTime.Add(avgTimeDelta).Sub(keyPressedTime)
		s.Logf("keyPressedTime: %v, keyReceivedTime: %v, latency: %v", keyPressedTime, keyReceivedTime, latency)
		return latency, nil
	}
	const numMeasureKeyLatency = 5
	var sumKeyLatency time.Duration
	for i := 0; i < numMeasureKeyLatency; i++ {
		s.Logf("Measuring key latency (%v/%v)", i+1, numMeasureKeyLatency)
		latency, err := measureKeyLatency()
		if err != nil {
			s.Error("Failed to measure key latency: ", err)
			continue
		}
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
