// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/factory/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Finalize,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test finalize process in factory toolkit",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		SoftwareDeps: append([]string{"reboot", "factory_flow"}, fixture.EnsureToolkitSoftwareDeps...),
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
		Fixture:      fixture.EnsureToolkit,
		// Skip "nyan_kitty" due to slow reboot speed.
		// TODO(b/203313828): Skip on dedede due to CQ failures
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty"), hwdep.SkipOnPlatform("dedede")),
	})
}

func Finalize(fullCtx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(fullCtx, time.Minute)
	defer cancel()

	d := s.DUT()

	// Wait system daemons up
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		out, err := d.Conn().CommandContext(ctx, "initctl", "status", "system-services").Output()
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "fail to access initctl"))
		}

		if strings.Contains(string(out), "start/running") {
			return nil
		}

		return errors.New("status: " + string(out))
	}, nil); err != nil {
		s.Fatal("Failed to wait system daemons up after reboot: ", err)
	}

	s.Log("Start wiping and umount")
	defer cleanup(fullCtx, s)
	if err := d.Conn().CommandContext(ctx, "gooftool", "wipe_in_place", "--test_umount").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to run wiping of finalize: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		out, err := d.Conn().CommandContext(ctx, "cat", "/tmp/wipe_init.log").Output()
		// keep retrying when the log file is not created.
		if err != nil {
			return errors.Wrap(err, "fail to access log")
		}

		if strings.Contains(string(out), "wipe_init failed") {
			return testing.PollBreak(errors.New("wipe_init failed"))
		}

		if strings.Contains(string(out), "GOOFTOOL command 'wipe_init' SUCCESS") {
			return nil
		}

		return errors.New("wipe have not finished yet")
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait wiping finished: ", err)
	}
}

func cleanup(ctx context.Context, s *testing.State) {
	d := s.DUT()

	s.Log("Backing up the logs")
	if err := d.GetFile(ctx, "/tmp/wipe_init.log", filepath.Join(s.OutDir(), "wipe_init.log")); err != nil {
		s.Error("Dump wipe_init.log fail: ", err)
	}
	if err := d.GetFile(ctx, "/old_root/tmp/wipe_in_tmpfs.log", filepath.Join(s.OutDir(), "wipe_in_tmpfs.log")); err != nil {
		s.Log("Dump wipe_in_tmpfs.log (after pivot root) fail: ", err)

		// Re-try the path before pivot root
		if err := d.GetFile(ctx, "/tmp/wipe_in_tmpfs.log", filepath.Join(s.OutDir(), "wipe_in_tmpfs.log")); err != nil {
			s.Error("Dump wipe_in_tmpfs.log (before pivot root) fail: ", err)
		}
	}

	s.Log("Cleaning up")
	// Reboot to recover umounted partitiions.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// /var/log may be umount during the test, backup the log after cleanup.
	s.Log("Backing up the logs under /var/log")
	if err := d.GetFile(ctx, "/var/log/upstart.log", filepath.Join(s.OutDir(), "upstart.log")); err != nil {
		s.Error("Dump upstart.log fail: ", err)
	}
}
