// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"strings"
	"syscall"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const kmsvncReadyMessage = "Listening for VNC connections"

// Kmsvnc wraps a kmsvnc process used in tests.
type Kmsvnc struct {
	cmd *testexec.Cmd
}

// NewKmsvnc launches kmsvnc as a subprocess and returns a handle.
// Blocks until kmsvnc is ready to accept connections, so it's safe to connect to kmsvnc once this function returns.
func NewKmsvnc(ctx context.Context) (*Kmsvnc, error) {
	cmd := testexec.CommandContext(ctx, "kmsvnc")

	// Create a pipe for stderr which we'll be listening later.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Launch a separate goroutine to listen stderr and print as logs.
	// Also detects when kmsvnc is ready to accept connections.
	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			t := scanner.Text()
			testing.ContextLog(ctx, "kmsvnc: ", t)
			if ready != nil && strings.Contains(t, kmsvncReadyMessage) {
				close(ready)
				ready = nil
			}
		}
	}()

	// Block until kmsvnc is ready, or context timeout.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ready:
		return &Kmsvnc{cmd}, nil
	}
}

// Stop terminates the kmsvnc process gracefully.
func (k *Kmsvnc) Stop() error {
	// In case this fails, the watchdog routine created by cmd.Start() will kill it when the context expires.
	if err := k.cmd.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return k.cmd.Wait()
}
