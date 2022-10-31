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

func init() {
	testing.AddTest(&testing.Test{
		Func: HibermanDryRun,
		Desc: "Checks the hiberman logs to ensure correct hibernate flow",
		Contacts: []string{
			"chromeos-hibernate@google.com", // Test owners
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// HibermanDryRun executes the hibernate dry-run and checks the log correctness.
func HibermanDryRun(ctx context.Context, s *testing.State) {
	const (
		// hibernationDeadline is an approximate time required to hibernate the machine.
		// The deadline is quite generous, as the time depends on a machine performance
		// and RAM-to-disk image size.
		hibernationDeadline = 60 * time.Second
	)

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
	// TODO(b/237084693): Request from evgreen@ in crrev/c/3992310 is to migrate
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
		writeMsg := func(ctx context.Context, msg string) bool {
			select {
			case ch <- msg:
				return true
			case ctx.Done():
				return false
			}
		}

		// The Scan method will return false once the dmesg process is killed.
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if !writeMsg(ctx, sc.Text()) {
				break
			}
		}
		// Don't bother checking sc.Err(). The test will already fail if the expected
		// message isn't seen.
	}()

	return cmd, ch, nil
}
