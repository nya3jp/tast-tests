// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	pingTimeout      = 10 * time.Second
	googleDotComIPv6 = "ipv6.google.com"
	googleDotComIPv4 = "ipv4.google.com"
)

// VerifyCrostiniIPConnectivity verifies the ip connectivity from crostini via cellular interface.
func VerifyCrostiniIPConnectivity(ctx context.Context, cmd func(context.Context, ...string) *testexec.Cmd, ipType string) error {
	if ipType == "ipv4" || ipType == "ipv4v6" {
		testing.ContextLog(ctx, "Verify IPv4 connectivity")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := cmd(ctx, "/bin/ping", "-c1", "-w1", googleDotComIPv4).Run(); err != nil {
				return errors.Wrap(err, "failed ipv4 ping test in Crostini")
			}
			return nil
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrap(err, "failed ipv4 ping test in Crostini")
		}
	}
	if ipType == "ipv6" || ipType == "ipv4v6" {
		testing.ContextLog(ctx, "Verify IPv6 connectivity")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := cmd(ctx, "/bin/ping6", "-c1", "-w1", googleDotComIPv6).Run(); err != nil {
				return errors.Wrap(err, "failed ipv6 ping test in Crostini")
			}
			return nil
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrap(err, "failed ipv6 ping test in Crostini")
		}
	}
	return nil
}

// VerifyIPConnectivity verifies the ip connectivity from Host/ARC via cellular interface.
func VerifyIPConnectivity(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, ipType, bindir string) error {
	if ipType == "ipv4" || ipType == "ipv4v6" {
		testing.ContextLog(ctx, "Verify IPv4 connectivity")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := cmd(ctx, filepath.Join(bindir, "ping"), "-c1", "-w1", googleDotComIPv4).Run(); err != nil {
				return errors.Wrap(err, "failed ipv4 ping test")
			}
			return nil
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrap(err, "failed ipv4 ping test")
		}
	}
	if ipType == "ipv6" || ipType == "ipv4v6" {
		testing.ContextLog(ctx, "Verify IPv6 connectivity")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := cmd(ctx, filepath.Join(bindir, "ping6"), "-c1", "-w1", googleDotComIPv6).Run(); err != nil {
				return errors.Wrap(err, "failed ipv6 ping test")
			}
			return nil
		}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
			return errors.Wrap(err, "failed ipv6 ping test")
		}
	}
	return nil
}
