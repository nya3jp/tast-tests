// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"bufio"
	"chromiumos/tast/errors"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"

	// Register the fixtures to remote bundle.
	_ "chromiumos/tast/remote/bundles/cros/meta/fixture"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsDebugger,
		Desc:     "Verifies that Tast can run with a debugger attached",
		Contacts: []string{"msta@google.com", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// Though the debugger should work on all x86 boards, testing it with a VM
		// and a single board should be sufficient, since it's not a hardware
		// dependent feature.
		HardwareDeps: hwdep.D(hwdep.Model("betty", "eve")),
		Timeout:      8 * time.Minute,
	})
}

type state string

const (
	notStarted      state = "Not Started"
	waitForDebugger       = "Waiting for debugger"
	connected             = "Connected"
)
const debuggerWaitString = "Waiting for debugger on port "

// A port chosen relatively arbitrarily, but explicitly not the ports suggested
// in the developer guide so you can easily attach the debugger to this test.
const debuggerPort = 2351

func RunTestsDebugger(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		name string
		test string
		kind string
		// By terminating tast early, we leave an instance of the debugger running.
		// This can cause an issue for running a second test, so it's worth testing.
		earlyExit bool
	}{
		{
			name:      "LocalEarlyExit",
			test:      "meta.LocalPass",
			kind:      "local",
			earlyExit: true,
		}, {
			name:      "Local",
			test:      "meta.LocalPass",
			kind:      "local",
			earlyExit: false,
		}, {
			name:      "RemoteEarlyExit",
			test:      "meta.RemotePass",
			kind:      "remote",
			earlyExit: true,
		}, {
			name:      "Remote",
			test:      "meta.RemotePass",
			kind:      "remote",
			earlyExit: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			flags := []string{fmt.Sprintf("-attachdebugger=%s:%d", tc.kind, debuggerPort)}
			cmd, err := tastrun.NewCommand(ctx, s, "run", flags, []string{tc.test})
			if err != nil {
				s.Fatal("Failed to generate tast command: ", err)
			}

			var wg sync.WaitGroup
			defer wg.Wait()

			handlePipeLines := func(pipeName string, pipeGetter func() (io.ReadCloser, error), lineHandler func(string)) {
				f, err := pipeGetter()
				if err != nil {
					s.Fatalf("Failed to get %s pipe for debugger command: %+v", pipeName, err)
				}

				scanner := bufio.NewScanner(f)
				wg.Add(1)
				go func() {
					defer wg.Done()
					for scanner.Scan() {
						line := scanner.Text()
						s.Logf("%s: %q", pipeName, line)
						lineHandler(line)
					}
				}()
			}

			currentState := notStarted

			handlePipeLines("Tast stdout", cmd.StdoutPipe, func(line string) {
				if idx := strings.Index(line, debuggerWaitString); idx != -1 {
					portString := line[idx+len(debuggerWaitString):]
					port, err := strconv.Atoi(portString)
					if err != nil || port != debuggerPort {
						s.Fatalf("Waiting for debugger on incorrect port: got %q, want %q", portString, debuggerPort)
					}
					currentState = waitForDebugger
				} else if currentState == waitForDebugger {
					// It could be that the debugger's connected, but we haven't registered it yet.
					testing.Sleep(ctx, time.Second)
					if currentState == waitForDebugger {
						s.Fatal("Tast claimed that it would wait for the debugger, but didn't wait")
					}
				}
			})
			handlePipeLines("Tast stderr", cmd.StderrPipe, func(string) {})

			s.Log("Running ", strings.Join(cmd.Args, " "))
			if err := cmd.Start(); err != nil {
				s.Fatal("Failed to start command: ", err)
			}
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
			}()

			waitForTransition := func(expected state, timeout time.Duration) error {
				return testing.Poll(ctx, func(ctx context.Context) error {
					if currentState != expected {
						return errors.Errorf("Waiting for currentState to become %s, currently %s", expected, currentState)
					}
					return nil
				}, &testing.PollOptions{Timeout: timeout, Interval: time.Millisecond * 100})
			}

			if err := waitForTransition(waitForDebugger, time.Minute); err != nil {
				s.Fatal("Tast never started waiting for debugger: ", err)
			}

			s.Log("Sleeping for 5 seconds to see if tast attempts to start the test without the debugger")
			testing.Sleep(ctx, time.Second*5)

			if tc.earlyExit {
				// Send a control-C to simulate the most likely way for the process to
				// end early.
				if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
					s.Fatal("Failed to kill process: ", err)
				}
				// Ignore the cmd.Wait error, since it's expected, and instead verify
				// that it failed with the conditions below.
				cmd.Wait()
				if !cmd.ProcessState.Exited() {
					s.Fatal("Process didn't end after being killed")
				} else if cmd.ProcessState.Success() {
					s.Fatal("Killed process should not have exited successfully")
				}
				return
			}
			s.Log("Starting up debugger")

			debuggerCmd := exec.CommandContext(ctx, "dlv", "connect", fmt.Sprintf("localhost:%d", debuggerPort))
			debuggerCmd.Stdin = strings.NewReader("continue\n")
			// The debugger doesn't output to stdout until it connects successfully.
			handlePipeLines("Debugger stdout", debuggerCmd.StdoutPipe, func(string) { currentState = connected })
			handlePipeLines("Debugger stderr", debuggerCmd.StderrPipe, func(line string) {
				if strings.Contains(line, "has exited with status") && !strings.HasSuffix(line, "has exited with status 0") {
					s.Fatalf("Expected process to exit with status 0, actually got: %q", line)
				}
			})
			if err := debuggerCmd.Start(); err != nil {
				s.Fatal("Failed to start the debugger: ", err)
			}
			defer func() {
				debuggerCmd.Process.Kill()
				debuggerCmd.Wait()
			}()

			if err := waitForTransition(connected, time.Second*10); err != nil {
				s.Fatal("Debugger never connected to tast: ", err)
			}

			if err := debuggerCmd.Wait(); err != nil {
				s.Fatal("Debugger failed to close: ", err)
			}

			if err := cmd.Wait(); err != nil {
				s.Fatal("Failed to complete command: ", err)
			}
		})
	}
}
