// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Kexec,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that platform supports kexec",
		Contacts:     []string{"ribalda@google.com", "chromeos-kdump@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pstore", "reboot"},
		Timeout:      3 * time.Minute,
	})
}

func Kexec(ctx context.Context, s *testing.State) {
	const KexecTest = " KexecTest"

	d := s.DUT()

	cmdline, err := d.Conn().CommandContext(ctx, "cat", "/proc/cmdline").CombinedOutput()
	if err != nil {
		s.Fatal("Failed to read cmdline")
	}

	// If the system has been booted via kexec, reboot the machine
	cmdlineRegexp := regexp.MustCompile(KexecTest)
	if cmdlineRegexp.Match(cmdline) {
		s.Log("DUT already booted via kexec, rebooting it")
		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}

	// Load the same kernel, and kexec()
	if output, err := d.Conn().CommandContext(ctx, "kexec", "-s", "-l", "/boot/vmlinuz", "--command-line="+string(cmdline)+KexecTest).CombinedOutput(); err != nil {
		s.Log(string(output))
		s.Fatal("Failed load kexec kernel: ", err)
	}
	d.Conn().CommandContext(ctx, "kexec", "-e").Run()

	// Reconnect to dut
	s.Log("Waiting for DUT to become unreachable")
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	// Validate ramoops
	if err := d.GetFile(ctx, "/sys/fs/pstore/console-ramoops-0", filepath.Join(s.OutDir(), "ramoops")); err != nil {
		s.Fatal("Failed to save ramoops")
	}

	f, err := ioutil.ReadFile(filepath.Join(s.OutDir(), "ramoops"))
	if err != nil {
		s.Fatal("Unable to read ramoops file")
	}

	kexecRegexp := regexp.MustCompile("kexec_core: Starting new kernel")
	if !kexecRegexp.Match(f) {
		s.Fatal("Boot was not triggered by kexec, checking ramoops")
	}

	// Validate cmdline
	cmdline, err = d.Conn().CommandContext(ctx, "cat", "/proc/cmdline").CombinedOutput()
	if err != nil {
		s.Fatal("Failed to read kexec cmdline")
	}

	if !cmdlineRegexp.Match(cmdline) {
		s.Fatal("Boot was not triggered by kexec, checking cmdline", cmdline)
	}
}
