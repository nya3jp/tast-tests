// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hibernate

import (
	"bufio"
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// hibernationDeadline is an approximate time required to hibernate the machine.
	// The deadline is quite generous, as the time depends on a machine performance
	// and RAM-to-disk image size.
	hibernationDeadline = 60 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HibermanDryRun,
		Desc: "Checks the hiberman logs to ensure correct hibernate flow",
		Contacts: []string{
			"chromeos-hibernate@google.com", // Test owners
			"tast-users@chromium.org",       // Backup mailing list
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// HibermanDryRun executes the hibernate dry-run and checks the log correctness.
func HibermanDryRun(ctx context.Context, s *testing.State) {
	s.Log("Start streamLogs()")
	logCmd, logCh, err := streamLogs(ctx)
	if err != nil {
		s.Fatal("Failed to start streamLogs(): ", err)
	}
	defer logCmd.Wait()
	defer logCmd.Kill()

	cmd := []string{"hiberman", "hibernate", "--test-keys", "--dry-run"}
	_, err = testexec.CommandContext(ctx, cmd[0], cmd[1:]...).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to execute %v: %v", cmd, err)
	}

	// Expected order of logs from correctly functioning hibernate dry-run.
	// TODO(jakubm): Request from evgreen@ in crrev/c/3992310 is to migrate
	// from using human readable logs to telemetry metrics.
	var hibermanHibernateLogs = []string{
		"Beginning hibernate",
		"Mounting hibervol",
		"Wrote hibernate image in",
		"Setting hibernate cookie at",
	}
	for _, expMsg := range hibermanHibernateLogs {
		s.Logf("Watching for %q in logs", expMsg)
		ctx, cancel := context.WithTimeout(ctx, hibernationDeadline)
		defer cancel()
		detectLog(ctx, s, logCh, expMsg)
	}
}

func detectLog(ctx context.Context, s *testing.State, ch <-chan string, expMsg string) bool {
	for {
		select {
		case msg := <-ch:
			if strings.Contains(msg, expMsg) {
				return true
			}
		case <-ctx.Done():
			s.Fatalf("Didn't see %q in channel: %v", expMsg, ctx.Err())
		}
	}
	return false
}

func streamLogs(ctx context.Context) (*testexec.Cmd, <-chan string, error) {
	// Start a process that writes messages to stdout as they're logged.
	cmd := testexec.CommandContext(ctx, "tail", "-f", "/var/log/messages")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch hiberman logs")
	}

	// Start a goroutine that just passes lines from dmesg to a channel.
	ch := make(chan string)
	go func() {
		defer close(ch)

		// Writes msg to ch and returns true if more messages should be written.
		writeMsg := func(msg string) bool {
			// To avoid blocking forever on a write to ch if nobody's reading from
			// it, we use a non-blocking write. If the channel isn't writable, sleep
			// briefly and then check if the context's deadline has been reached.
			for {
				if ctx.Err() != nil {
					return false
				}

				select {
				case ch <- msg:
					return true
				default:
					testing.Sleep(ctx, 10*time.Millisecond)
				}
			}
		}

		// The Scan method will return false once the dmesg process is killed.
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if !writeMsg(sc.Text()) {
				break
			}
		}
		// Don't bother checking sc.Err(). The test will already fail if the expected
		// message isn't seen.
	}()

	return cmd, ch, nil
}
