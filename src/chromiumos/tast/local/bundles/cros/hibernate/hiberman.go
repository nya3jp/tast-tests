// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hibernate

import (
	"bufio"
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HibermanHibernateDryRun,
		Desc: "Checks the hiberman logs to ensure correct hibernate flow",
		Contacts: []string{
			"chromeos-hibernate@google.com", // Test owners
			"tast-users@chromium.org",       // Backup mailing list
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// HibermanHibernateDryRun executes the hibernate dry-run and checks the log correctness.
func HibermanHibernateDryRun(ctx context.Context, s *testing.State) {
	// Steps for the test:
	//  1. Start streamLogs channel
	//  2. Start "hiberman hibernate --test-keys --dry-run"
	//  3. Check for following logs
	//    * Beginning hibernate
	//    * Activating hibervol / Mounting hibervol
	//    * Wrote hibernate image in
	//    * Setting hibernate cookie at
}

func streamLogs(ctx context.Context) (*testexec.Cmd, <-chan string, error) {
	// Start a process that writes messages to stdout as they're logged.
	cmd := testexec.CommandContext(ctx, "tail -f /var/log/messages | grep -ai hiberman")
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
