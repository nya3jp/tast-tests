// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	pingTimeout      = 10 * time.Second
	speedtestTimeout = 1 * time.Minute
	googleDotComIPv6 = "ipv6test.google.com"
	googleDotComIPv4 = "ipv4.google.com"
)

func verifyCrostiniIPv4Ping(ctx context.Context, cmd func(context.Context, ...string) *testexec.Cmd) error {
	testing.ContextLog(ctx, "Verify IPv4 connectivity")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cmd(ctx, "/bin/ping", "-c1", "-w1", googleDotComIPv4).Run(); err != nil {
			return errors.Wrapf(err, "failed to ping %s in Crostini", googleDotComIPv4)
		}
		return nil
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		return errors.Wrapf(err, "failed to ping %s in Crostini", googleDotComIPv4)
	}
	return nil
}

func verifyCrostiniIPv6Ping(ctx context.Context, cmd func(context.Context, ...string) *testexec.Cmd) error {
	testing.ContextLog(ctx, "Verify IPv6 connectivity")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cmd(ctx, "/bin/ping6", "-c1", "-w1", googleDotComIPv6).Run(); err != nil {
			return errors.Wrapf(err, "failed to ping %s in Crostini", googleDotComIPv6)
		}
		return nil
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		return errors.Wrapf(err, "failed to ping %s in Crostini", googleDotComIPv6)
	}
	return nil
}

// VerifyCrostiniIPConnectivity verifies the ip connectivity from crostini via cellular interface.
func VerifyCrostiniIPConnectivity(ctx context.Context, cmd func(context.Context, ...string) *testexec.Cmd, ipv4, ipv6 bool) error {
	if ipv4 {
		if err := verifyCrostiniIPv4Ping(ctx, cmd); err != nil {
			return err
		}
	}
	if ipv6 {
		if err := verifyCrostiniIPv6Ping(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func verifyIPv4Ping(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, bindir string) error {
	testing.ContextLog(ctx, "Verify IPv4 connectivity")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cmd(ctx, filepath.Join(bindir, "ping"), "-c1", "-w1", googleDotComIPv4).Run(); err != nil {
			return errors.Wrapf(err, "failed to ping %s", googleDotComIPv4)
		}
		return nil
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		return errors.Wrapf(err, "failed to ping %s", googleDotComIPv4)
	}
	return nil
}

func verifyIPv6Ping(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, bindir string) error {
	testing.ContextLog(ctx, "Verify IPv6 connectivity")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cmd(ctx, filepath.Join(bindir, "ping6"), "-c1", "-w1", googleDotComIPv6).Run(); err != nil {
			return errors.Wrapf(err, "failed to ping %s", googleDotComIPv6)
		}
		return nil
	}, &testing.PollOptions{Timeout: pingTimeout}); err != nil {
		return errors.Wrapf(err, "failed to ping %s", googleDotComIPv6)
	}
	return nil
}

// VerifyIPConnectivity verifies the ip connectivity from Host/ARC via cellular interface.
func VerifyIPConnectivity(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, ipv4, ipv6 bool, bindir string) error {
	if ipv4 {
		if err := verifyIPv4Ping(ctx, cmd, bindir); err != nil {
			return err
		}
	}
	if ipv6 {
		if err := verifyIPv6Ping(ctx, cmd, bindir); err != nil {
			return err
		}
	}
	if !ipv4 && !ipv6 {
		return errors.New("no ip network found")
	}
	return nil
}

// RunHostIPSpeedTest runs speedtest on cellular interface and returns the download and upload speeds in bps
func RunHostIPSpeedTest(ctx context.Context, cmd func(context.Context, string, ...string) *testexec.Cmd, bindir string) (upload, download float64, err error) {
	testing.ContextLog(ctx, "Run IP Connectivity Speed Test")
	var uploadSpeed, downloadSpeed float64 = 0.0, 0.0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := cmd(ctx, filepath.Join(bindir, "speedtest-cli"), "--json").Output()
		if err != nil {
			return errors.Wrap(err, "failed speed test")
		}
		var data map[string]interface{}
		if err := json.Unmarshal(out, &data); err != nil {
			return errors.Wrap(err, "failed to unmarshal output")
		}
		downloadSpeed = data["download"].(float64)
		uploadSpeed = data["upload"].(float64)
		testing.ContextLogf(ctx, " Download %.2f bps", downloadSpeed)
		testing.ContextLogf(ctx, " Upload   %.2f bps", uploadSpeed)
		return nil
	}, &testing.PollOptions{Timeout: speedtestTimeout}); err != nil {
		return 0, 0, errors.Wrap(err, "failed speed test")
	}
	return uploadSpeed, downloadSpeed, nil
}
