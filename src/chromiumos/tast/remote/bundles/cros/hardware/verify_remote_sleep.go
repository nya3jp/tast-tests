// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// all clocks from linux.die.net/man/2/clock_gettime are supported at the
// time of writing this comment. This array should be kept in sync with
// the `clocks` array in helpers/local/hardware.VerifyRemoteSleep.timersignal.c,
// which implements the remote side of this test.
var allowableRemoteClocks = []string{
	"CLOCK_REALTIME",
	"CLOCK_REALTIME_COARSE",
	"CLOCK_MONOTONIC",
	"CLOCK_MONOTONIC_COARSE",
	"CLOCK_MONOTONIC_RAW",
	"CLOCK_BOOTTIME",
	"CLOCK_PROCESS_CPUTIME_ID",
	"CLOCK_THREAD_CPUTIME_ID",
}

const (
	sleepDuration   = 300 * time.Second
	sleepIterations = 10
	// this value was selected empirically. The usual variance in the results is
	// well below 10ms and should be consistent across different setups - there's
	// no network or other sources of noise in the test. Heavy load may cause this
	// to fail sporadically* (albeit I've never managed to make it happen), but if
	// the test fails repeatably, something is amiss.
	// *Some theoretical flakiness is necessarily introduced simply because of the
	// fact that this test requires communication between DUT and the testing machine.
	sleepTolerance = 20 * time.Millisecond
	// set to false if you want to run the test to completion even if it fails
	failEagerly = true
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyRemoteSleep,
		Desc:         "Verifies that sleeps on DUT are as long as they should be",
		Contacts:     []string{"semihalf@google.com"},
		Attr:         []string{},
		Timeout:      sleepDuration*sleepIterations + time.Minute,
		SoftwareDeps: []string{},
		Vars:         []string{"hardware.VerifyRemoteSleep.clock"},
	})
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func doRemoteSleep(ctx context.Context, s *testing.State, remoteClock string) *ssh.Cmd {
	const exe = "/usr/local/libexec/tast/helpers/local/cros/hardware.VerifyRemoteSleep.timersignal"
	sleepArg := strconv.FormatInt(int64(sleepDuration.Milliseconds()), 10)
	itersArg := strconv.FormatInt(sleepIterations, 10)
	fileArg := "/dev/ttyS0"
	testCommand := fmt.Sprintf("sleep 1; %s %s %s %s %s", exe, sleepArg, itersArg, remoteClock, fileArg)

	dut := s.DUT()
	cmd := dut.Conn().Command("sh", "-c", testCommand)
	err := cmd.Start(ctx)

	if err != nil {
		s.Fatal("Couldn't start the remote sleep: ", err)
	}

	return cmd
}

func serialReadUntilPing(s *testing.State, reader *bufio.Reader) {
	const suffix = "ping"

	/* ignore lines that aren't ".*ping\s*" */
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			s.Fatal("Couldn't read line: ", err)
		}

		trimmed := strings.TrimSpace(line)
		if !strings.HasSuffix(trimmed, suffix) {
			s.Log("Unexpected line: ", line)
		} else {
			break
		}
	}
}

func measureRemoteSleep(ctx context.Context, s *testing.State) []time.Duration {
	var durations []time.Duration

	srv, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Couldn't get default servo: ", err)
	}

	pty, err := srv.GetString(ctx, "cpu_uart_pty")
	if err != nil {
		s.Fatal("Couldn't get cpu_uart_pty from servo: ", err)
	}
	s.Log("Detected cpu_uart_pty at ", pty)

	ptyFile, err := os.Open(pty)
	if err != nil {
		s.Fatalf("Couldn't open %v: %v", pty, err)
	}
	defer ptyFile.Close()

	reader := bufio.NewReader(ptyFile)

	serialReadUntilPing(s, reader)

	for i := 0; i < sleepIterations; i++ {
		start := time.Now()

		serialReadUntilPing(s, reader)

		elapsed := time.Since(start)
		durations = append(durations, elapsed)
		measuredMs := float32(elapsed) / float32(time.Millisecond)

		lowerBound := sleepDuration - sleepTolerance
		upperBound := sleepDuration + sleepTolerance
		lowerMs := lowerBound.Milliseconds()
		upperMs := upperBound.Milliseconds()

		s.Logf("Measured: %vms", measuredMs)

		if elapsed < lowerBound || elapsed > upperBound {
			if failEagerly {
				s.Fatalf("[ERR] %vms not in range [%v, %v]ms", measuredMs, lowerMs, upperMs)
			} else {
				s.Errorf("[ERR] %vms not in range [%v, %v]ms", measuredMs, lowerMs, upperMs)
			}
		}
	}

	return durations
}

func VerifyRemoteSleep(ctx context.Context, s *testing.State) {
	remoteClockArg, ok := s.Var("hardware.VerifyRemoteSleep.clock")
	if !ok {
		s.Fatal("Variable hardware.VerifyRemoteSleep.clock not supplied. Consider passing one of the following values: ", allowableRemoteClocks)
	} else if !stringInSlice(remoteClockArg, allowableRemoteClocks) {
		s.Fatal("Invalid variable hardware.VerifyRemoteSleep.clock. Consider passing one of the following values: ", allowableRemoteClocks)
	}

	cmd := doRemoteSleep(ctx, s, remoteClockArg)
	defer cmd.Wait(ctx)
	defer cmd.Abort()

	measureRemoteSleep(ctx, s)
}
