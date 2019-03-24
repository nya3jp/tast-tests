// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
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

// A file which is to be copied to/from guest.
type fileMapping struct {
	hostPath, guestPath string
}

// A timestamp or an error. Useful when passing a timestamp via a channel with potential errors.
type timeOrErr struct {
	ts  time.Time
	err error
}

// A cleanup function, usually called in a deferred manner.
type cleanupFunc func() error

// CrostiniInputLatency measures input latency to Crostini container.
// In the container side, it launches a xterm running a python script to wait for a key stroke.
// However, host clock and guest clock is not synced so we can't simply subtract the timestamps
// of key press event on host and key received event on guest.
// Instead, we launch a socket server in the guest VM. When a key arrives at the guest, the
// socket server sends a response back to host. We then subtract the timestamp host receives
// the response by (RTT time)/2 as an estimation to key arrival time on guest.
func CrostiniInputLatency(ctx context.Context, s *testing.State) {
	// TODO(cylee): Consolidate container creation logic in a util function since it appears
	// in multiple files.
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

	// runCleanup runs c if non-nil and reports any error that it returns.
	runCleanup := func(c cleanupFunc, errString string) {
		if c != nil {
			if err = c(); err != nil {
				s.Errorf("%v: %v", errString, err)
			}
		}
	}

	// Bring up a socket server in container.
	const (
		socketServerFileName = "crostini_input_latency_server.py"
		socketServerLogName  = "socket_server_log"
	)
	socketServerFile := fileMapping{
		hostPath:  s.DataPath(socketServerFileName),
		guestPath: filepath.Join(perfutil.ContainerHomeDir, socketServerFileName),
	}
	socketServerLog := fileMapping{
		hostPath:  filepath.Join(s.OutDir(), socketServerLogName),
		guestPath: filepath.Join(perfutil.ContainerHomeDir, socketServerLogName),
	}
	serverCleanup, serverPort, err := startInputLatencyServer(ctx, cont, socketServerFile, socketServerLog, errFile)
	defer runCleanup(serverCleanup, "Failed cleaning up input latency server")
	if err != nil {
		s.Fatal("Failed to start input latency server: ", err)
	}

	// Set up a socket listener.
	containerIP, err := cont.GetIPv4Address(ctx)
	if err != nil {
		s.Fatal("Failed to get container IP address: ", err)
	}
	socketServerAddr := fmt.Sprintf("%v:%v", containerIP, serverPort)
	s.Log("Connecting to socket server ", socketServerAddr)

	guestSock, err := net.Dial("udp", socketServerAddr)
	if err != nil {
		s.Fatal("Failed to guestSockect to socket server: ", err)
	}
	defer guestSock.Close()

	// socketChan return the timestamp on host when it receives a response from guest socket
	// server.
	socketChan := make(chan timeOrErr)
	socketListenerCleanup := listenToSocketServer(ctx, guestSock, socketChan)
	defer runCleanup(socketListenerCleanup, "Failed cleaning up socket listener")
	if err != nil {
		s.Fatal("Failed to listen to socket server: ", err)
	}

	s.Log("Start measuring socket RTT")
	// Ping socket server and returns the RTT time.
	pingGuest := func() (time.Duration, error) {
		const waitSocketResponseTimeout = time.Second

		pingSendTime := time.Now()
		if _, err = guestSock.Write([]byte("ping")); err != nil {
			return 0, errors.Wrap(err, "failed pinging container")
		}

		select {
		case res := <-socketChan:
			if res.err != nil {
				return 0, res.err
			}
			return res.ts.Sub(pingSendTime), nil
		case <-time.After(waitSocketResponseTimeout):
			return 0, errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		case <-ctx.Done():
			return 0, errors.Wrap(ctx.Err(), "failed waiting for server pong")
		}
	}
	// Get rid of the initial ping since the initial ping time tends to be skewed.
	if _, err = pingGuest(); err != nil {
		s.Error("Initial guest ping failed: ", err)
	}

	// Calculate average RTT time.
	const numPings = 1000
	goodPings := 0
	var sumRTT time.Duration
	for i := 0; i < numPings; i++ {
		if ctx.Err() != nil {
			s.Fatal("Context error when measuring ping latency: ", ctx.Err())
		}
		if rtt, err := pingGuest(); err != nil {
			s.Error("Failed to ping socket server: ", err)
		} else {
			sumRTT += rtt
			goodPings++
		}
	}
	s.Logf("%v out of %v pings are successful", goodPings, numPings)
	if float64(goodPings) < 0.9*float64(numPings) {
		s.Errorf("Too many failed pings (%v/%v)", numPings-goodPings, numPings)
	}
	if goodPings == 0 {
		s.Fatal("All pings are failed")
	}
	avgRTT := sumRTT / time.Duration(goodPings)
	s.Log("Avg. RTT time: ", avgRTT.Round(time.Microsecond))

	keyWriter, err := input.Keyboard(ctx)
	if err != nil {
		s.Error("Failed to get keyboard event writer: ", err)
	}
	defer keyWriter.Close()

	// Listen to evtest before sending keys.
	// keyEventChan returns the timestamp of a key press event.
	keyEventChan := make(chan timeOrErr)

	// Set up evtest to monitor key events.
	evtestCleanup, err := startEvtest(ctx, keyWriter.Device(), keyEventChan)
	defer runCleanup(evtestCleanup, "Failed cleaning up evtest")
	if err != nil {
		s.Fatal("Failed to start evtest: ", err)
	}

	measureKeyLatency := func() (time.Duration, error) {
		// Sends a key to guest.
		err = keyWriter.Type(ctx, "\n")
		if err != nil {
			s.Error("Failed to inject a key event: ", err)
		}

		// Get the time when response is received from socket server.
		const waitSocketResponseTimeout = time.Second
		var keyReceivedTime time.Time
		select {
		case res := <-socketChan:
			if res.err != nil {
				return 0, res.err
			}
			keyReceivedTime = res.ts
		case <-time.After(waitSocketResponseTimeout):
			return 0, errors.Errorf("no container response in %v", waitSocketResponseTimeout)
		case <-ctx.Done():
			return 0, errors.Wrap(ctx.Err(), "failed reading socket response")
		}

		// Get key pressed timestamp.
		const waitKeyTimeout = time.Second
		var keyPressedTime time.Time
		select {
		case res := <-keyEventChan:
			if res.err != nil {
				return 0, err
			}
			keyPressedTime = res.ts
		case <-time.After(waitKeyTimeout):
			return 0, errors.Errorf("no key pressed in %v", waitKeyTimeout)
		case <-ctx.Done():
			return 0, errors.Wrap(ctx.Err(), "failed waiting for key press event")
		}

		// Estimate key received time on guest by subtracting (RTT time)/2.
		latency := keyReceivedTime.Add(-avgRTT / 2).Sub(keyPressedTime)

		return latency, nil
	}

	// Get rid of the initial measurement since it tends to be skewed.
	if _, err = measureKeyLatency(); err != nil {
		s.Error("Initial key latency measurement failed: ", err)
	}

	const numKeystrokes = 100
	goodKeystrokes := 0
	var sumKeyLatency time.Duration
	for i := 0; i < numKeystrokes; i++ {
		if ctx.Err() != nil {
			s.Fatal("Context error when measuring key latency: ", ctx.Err())
		}
		latency, err := measureKeyLatency()
		if err != nil {
			s.Error("Failed to measure key latency: ", err)
			continue
		}
		s.Logf("Key input latency (%v/%v): %v", i+1, numKeystrokes, latency.Round(time.Microsecond))
		sumKeyLatency += latency
		goodKeystrokes++
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
	s.Logf("%v out of %v keyboard latency measurements are successful", goodKeystrokes, numKeystrokes)
	if float64(goodKeystrokes) < 0.9*float64(numKeystrokes) {
		s.Errorf("Too many failed keyboard latency measurements (%v/%v)", numKeystrokes-goodKeystrokes, numKeystrokes)
	}
	if goodKeystrokes != 0 {
		avgKeyLatency := sumKeyLatency / time.Duration(goodKeystrokes)
		s.Log("Avg. key latency: ", avgKeyLatency.Round(time.Microsecond))
	}
}

// getTimeFromTimestamp expects a timestamp in string format like "1553789996.390980959" and
// returns an equivalent time.Time struct.
func getTimeFromTimestamp(ts string) (time.Time, error) {
	// time.ParseDuration turns the timestamp into a time.Duration. Add it to epoch time
	// to get a time.Time object.
	d, err := time.ParseDuration(ts + "s")
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to parse %q", ts)
	}
	return time.Unix(0, 0).Add(d), nil
}

// startEvtest starts a evtest process and report keyboard events via a channel.
// The cleanupFunc returned, if not nil, must be run unconditionally by the caller.
func startEvtest(ctx context.Context, dev string, eventChan chan<- timeOrErr) (cleanupFunc, error) {
	evtestCmd := testexec.CommandContext(ctx, "evtest", dev)
	evtestOut, err := evtestCmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create evtest stdout pipe")
	}
	if err := evtestCmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start evtest")
	}

	// Report errors from the monitoring goroutine. Closed when the goroutine is finished.
	evtestDone := make(chan error)
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
	// So we shouldn't send keys before seeing the line "Testing ... (interrupt to exit)",
	// otherwise the event would be missed.
	evtestReady := make(chan struct{})

	go func() {
		evtestScanner := bufio.NewScanner(evtestOut)
		for evtestScanner.Scan() {
			line := evtestScanner.Text()
			if strings.HasPrefix(line, "Testing ... (interrupt to exit)") {
				testing.ContextLog(ctx, "evtest begins monitoring key events")
				close(evtestReady)
				continue
			}

			// Waiting for key press event in the format like
			// Event: time 1553427907.227424, type 1 (EV_KEY), code 28 (KEY_ENTER), value 1
			enterPressedRegexp := regexp.MustCompile(
				`Event: time (\d*\.?\d+), type 1 \(EV_KEY\), code \d+ \(\S+\), value 1`)
			matched := enterPressedRegexp.FindStringSubmatch(line)
			if matched != nil {
				ts, err := getTimeFromTimestamp(matched[1])
				if err != nil {
					eventChan <- timeOrErr{err: errors.Wrap(err, "failed to get key press timestamp")}
				} else {
					eventChan <- timeOrErr{ts, nil}
				}
			}
		}
		if err := evtestScanner.Err(); err != nil {
			evtestDone <- errors.Wrap(err, "error reading evtest")
		} else {
			close(evtestDone)
		}
	}()

	cleanup := func() error {
		testing.ContextLog(ctx, "Shutting down evtest")
		if err := evtestCmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Error killing evtest: ", err)
		}
		if err := evtestCmd.Wait(); err != nil {
			status := evtestCmd.ProcessState.Sys().(syscall.WaitStatus)
			signaled := status.Signaled()
			signal := status.Signal()
			// Expect it to be killed.
			if !signaled || signal != syscall.SIGKILL {
				return errors.Wrap(err, "evtest not finished by expected SIGKILL")
			}
		}
		testing.ContextLog(ctx, "evtest process terminated")
		// Block waiting for evtest goroutine to be finished.
		testing.ContextLog(ctx, "Waiting for evtest monitoring goroutine to finish")
		select {
		case err := <-evtestDone:
			if err != nil {
				return errors.Wrap(err, "evtest reading error")
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "failed waiting for evtest monitoring goroutine to finish")
		}
		testing.ContextLog(ctx, "evtest monitoring goroutine finished")
		return nil
	}

	testing.ContextLog(ctx, "Waiting for evtest to start monitoring key events")
	const waitEvtestReadyTimeout = 10 * time.Second
	select {
	case <-evtestReady:
		testing.ContextLog(ctx, "evtest ready, continue testing")
	case <-time.After(waitEvtestReadyTimeout):
		return cleanup, errors.Errorf("evtest failed to monitor key events in %v", waitEvtestReadyTimeout)
	case <-ctx.Done():
		return cleanup, errors.Wrap(ctx.Err(), "failed waiting for evtest readiness")
	}

	return cleanup, nil
}

// startInputLatencyServer starts a socket server in container and returns the listening port.
// The cleanupFunc returned, if not nil, must be run unconditionally by the caller.
func startInputLatencyServer(ctx context.Context, cont *vm.Container, socketServerFile, socketServerLog fileMapping,
	errFile io.Writer) (cleanup cleanupFunc, port uint16, err error) {
	if err := cont.PushFile(ctx, socketServerFile.hostPath, socketServerFile.guestPath); err != nil {
		return nil, 0, errors.Wrapf(err, "failed to push %v to container", socketServerFile.hostPath)
	}

	// The dynamic socket server port is communicated via a file. Remove the file before
	// launching the server. When it's accessible we know the server is ready.
	portFile := filepath.Join(perfutil.ContainerHomeDir, "crostini_socket_server_port")
	_, err = perfutil.RunCmd(ctx, cont.Command(ctx, "rm", "-f", portFile), errFile)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "failed to remove stale socket server port file %v", portFile)
	}

	socketServerArgs := []string{"xterm", "-e", fmt.Sprintf("/usr/bin/python %v >%v 2>&1", socketServerFile.guestPath, socketServerLog.guestPath)}
	socketServerCmd := cont.Command(ctx, socketServerArgs...)
	if err := socketServerCmd.Start(); err != nil {
		return nil, 0, errors.Wrap(err, "failed to start socket server in container")
	}
	cleanup = func() error {
		if err := socketServerCmd.Wait(); err != nil {
			return errors.Wrap(err, "socket server exited with error")
		}
		err := cont.GetFile(ctx, socketServerLog.guestPath, socketServerLog.hostPath)
		if err != nil {
			return errors.Wrap(err, "failed to copy socket server log")
		}
		testing.ContextLog(ctx, "Socket server exited successfully and log copied to ", socketServerLog.hostPath)
		return nil
	}

	var portString string
	// Waiting for server to be ready.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		getPortCmd := cont.Command(ctx, "cat", portFile)
		out, err := perfutil.RunCmd(ctx, getPortCmd, errFile)
		if err != nil {
			return err
		}
		portString = string(out)
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second})
	if err != nil {
		return cleanup, 0, errors.Wrap(err, "failed to get socket server port")
	}

	// Expect an uint16, but the return value of ParseUInt is always an uint64.
	port64, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return cleanup, 0, errors.Wrapf(err, "failed to parse port nunmber %q", portString)
	}
	return cleanup, uint16(port64), nil
}

// listenToSocketServer connects to the socket server in container and reports response time via
// a channel. The cleanupFunc returned must be run unconditionally by the caller.
func listenToSocketServer(ctx context.Context, conn net.Conn, ch chan<- timeOrErr) cleanupFunc {
	readDone := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				ch <- timeOrErr{err: errors.Wrap(err, "reading socket error")}
			}
			// On exit, host sends an "exit" command when cleanup. Server sends
			// an "exit" back when received it.
			if n == 4 && bytes.Equal(buf[:n], []byte("exit")) {
				break
			}
			ch <- timeOrErr{time.Now(), nil}
		}
		close(readDone)
	}()
	return func() error {
		testing.ContextLog(ctx, "Shutting down socket server")
		if _, err := conn.Write([]byte("exit")); err != nil {
			return errors.Wrap(err, "failed to terminate socket server")
		}
		testing.ContextLog(ctx, "Waiting for socket listener goroutine to finish")
		select {
		case <-readDone:
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "failed waiting for socket-listening goroutine to finish")
		}
		testing.ContextLog(ctx, "Socket listener goroutine finsished")
		return nil
	}
}
