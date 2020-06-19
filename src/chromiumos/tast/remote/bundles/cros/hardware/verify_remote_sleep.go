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

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

const (
	sleepDuration   = 300 * time.Second
	sleepIterations = 10
	sleepTolerance  = 20 * time.Millisecond
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyRemoteSleep,
		Desc:         "Verifies that sleeps on DUT are as long as they should be",
		Contacts:     []string{"semihalf@google.com"},
		Attr:         []string{},
		Timeout:      sleepDuration*sleepIterations + time.Minute,
		SoftwareDeps: []string{},
	})
}

func doRemoteSleep(ctx context.Context, s *testing.State) {
	sleepArg := strconv.FormatInt(int64(sleepDuration.Milliseconds()), 10)
	itersArg := strconv.FormatInt(sleepIterations, 10)
	fileArg := "/dev/ttyS0"
	testCommand := fmt.Sprintf("sleep 1; remote_sleep_test %s %s %s", sleepArg, itersArg, fileArg)

	dut := s.DUT()
	cmd := dut.Conn().Command("sh", "-c", testCommand)
	err := cmd.Start(ctx)

	if err != nil {
		s.Fatal("Couldn't start the remote sleep: ", err)
	}
}

func serialReadUntilPing(s *testing.State, reader *bufio.Reader) {
	suffix := "ping"

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

	reader := bufio.NewReader(ptyFile)

	serialReadUntilPing(s, reader)

	for i := 0; i < sleepIterations; i++ {
		start := time.Now()

		serialReadUntilPing(s, reader)

		elapsed := time.Since(start)
		durations = append(durations, elapsed)
		measuredMs := float32(elapsed) / float32(time.Millisecond)
		s.Logf("Measured: %vms", measuredMs)
	}

	return durations
}

func VerifyRemoteSleep(ctx context.Context, s *testing.State) {
	doRemoteSleep(ctx, s)

	measured := measureRemoteSleep(ctx, s)
	lowerBound := sleepDuration - sleepTolerance
	upperBound := sleepDuration + sleepTolerance
	failed := false

	for _, dur := range measured {
		if dur < lowerBound || dur > upperBound {
			failed = true
			durMs := dur.Milliseconds()
			lowerMs := lowerBound.Milliseconds()
			upperMs := upperBound.Milliseconds()
			s.Logf("[ERR] %vms not in range [%v, %v]ms", durMs, lowerMs, upperMs)
		}
	}

	if failed {
		s.Fatalf("Some measured sleeps were shorter than %vms", sleepDuration.Milliseconds())
	}
}
