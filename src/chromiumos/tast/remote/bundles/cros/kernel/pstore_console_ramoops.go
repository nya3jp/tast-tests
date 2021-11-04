// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PstoreConsoleRamoops,
		Desc: "Fails if console-ramoops isn't maintained across a warm reboot",
		Contacts: []string{
			"swboyd@chromium.org",
			"chromeos-kernel-test@google.com",
		},
		SoftwareDeps: []string{"pstore", "reboot"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// PstoreConsoleRamoops confirms that pstore is properly saving the previous kernel log.
func PstoreConsoleRamoops(ctx context.Context, s *testing.State) {
	CheckForPstoreConsoleRamoops := func(ctx context.Context, d *dut.DUT, s *testing.State) bool {
		ramoopsDir := filepath.Join(s.OutDir(), "console-ramoops")
		if err := linuxssh.GetFile(ctx, d.Conn(), "/sys/fs/pstore/", ramoopsDir, linuxssh.PreserveSymlinks); err != nil {
			s.Fatal("Failed to copy ramoops dir after reboot on the DUT: ", err)
		}

		files, err := ioutil.ReadDir(ramoopsDir)
		if err != nil {
			s.Fatal("Failed to list ramoops directory: ", err)
		}

		for _, file := range files {
			if strings.HasPrefix(file.Name(), "console-ramoops") && file.Size() > 0 {
				return true
			}
		}

		return false
	}

	d := s.DUT()

	if CheckForPstoreConsoleRamoops(ctx, d, s) {
		return
	}

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	if CheckForPstoreConsoleRamoops(ctx, d, s) {
		return
	}

	s.Error("Couldn't find any console-ramoops file")
}
