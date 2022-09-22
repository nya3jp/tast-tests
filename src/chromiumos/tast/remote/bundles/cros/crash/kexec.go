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
		Timeout:      10 * time.Minute,
	})
}

func Kexec(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"

	d := s.DUT()

	if err := d.Conn().CommandContext(ctx, "reboot").Run(); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	s.Log("Waiting for DUT to become unreachable")

	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	if err := d.GetFile(ctx, "/sys/fs/pstore/console-ramoops-0", filepath.Join(s.OutDir(), "ramoops")); err != nil {
		s.Fatal("Failed to save ramoops")
	}

	rebootRegexp := regexp.MustCompile("reboot: Restarting system")
	f, err := ioutil.ReadFile(filepath.Join(s.OutDir(), "ramoops"))
	if err != nil {
		s.Fatal("Unable to read ramoops file")
	}

	if !rebootRegexp.Match(f) {
		s.Fatal("Boot was not triggered by reboot")
	}

}
