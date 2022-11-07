// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides util functions for hibernate tast.
package utils

import (
	"bufio"
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DetectLogs reads the string channel until the all string match or context expires.
func DetectLogs(ctx context.Context, s *testing.State, ch <-chan string, expMsgs []string) error {
	if len(expMsgs) == 0 {
		return nil
	}
	msgCount := 0
	for {
		select {
		case msg := <-ch:
			if strings.Contains(msg, expMsgs[msgCount]) {
				msgCount = msgCount + 1
			}
			if msgCount == len(expMsgs) {
				return nil
			}
		case <-ctx.Done():
			return errors.Errorf("didn't see %v in channel: %v", expMsgs[msgCount:], ctx.Err())
		}
	}
	return nil
}

// StreamLogs tails the /var/log/messages and wrtires them to the channel.
func StreamLogs(ctx context.Context) (*testexec.Cmd, <-chan string, error) {
	// Start a process that writes messages to stdout as they're logged.
	cmd := testexec.CommandContext(ctx, "tail", "-f", "/var/log/messages")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch /var/log/messages")
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
