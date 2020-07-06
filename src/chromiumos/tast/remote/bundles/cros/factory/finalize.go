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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Finalize,
		Desc:         "Test finalize process in factory toolkit",
		Contacts:     []string{"menghuan@chromium.org", "chromeos-factory-eng@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      8 * time.Minute,
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
	})
}

func Finalize(fullCtx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(fullCtx, time.Minute)
	defer cancel()
	defer cleanup(fullCtx, s)

	d := s.DUT()

	s.Log("Reboot to setup the same environment as factory")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	// Wait system daemons up
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		out, err := d.Command("initctl", "status", "system-services").Output(ctx)
		// keep retrying when the log file is not created.
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

	// "gooftool" of "factory-mini" package has been already installed on test image.
	if err := d.Command("gooftool", "wipe_in_place", "--test_umount").Run(ctx); err != nil {
		s.Fatal("Failed to run wiping of finalize: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		out, err := d.Command("cat", "/tmp/wipe_init.log").Output(ctx)
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
