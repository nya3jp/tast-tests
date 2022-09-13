// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

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
		Attr:         []string{"group:mainline"},
	})
}

// PstoreConsoleRamoops confirms that pstore is properly saving the previous kernel log.
func PstoreConsoleRamoops(ctx context.Context, s *testing.State) {
	d := s.DUT()

	checkForPstoreConsoleRamoops := func(ctx context.Context, destName string) bool {
		ramoopsDir := filepath.Join(s.OutDir(), destName)
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

	if checkForPstoreConsoleRamoops(ctx, "before-reboot") {
		// The ramoops file already exists so this kernel under test
		// has a working ramoops implementation that has kept the
		// kernel log from the previous boot. No need to reboot.
		return
	}

	// Userspace must have deleted the file or this kernel is from cold boot.
	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	if checkForPstoreConsoleRamoops(ctx, "after-reboot") {
		return
	}

	s.Error("Couldn't find any console-ramoops file")

	// Save eventlog for failure analysis
	if err := d.GetFile(ctx, "/var/log/eventlog.txt",
		filepath.Join(s.OutDir(), "eventlog.txt")); err != nil {
		s.Log("Failed to save eventlog")
	}

	// Save coreboot log for failure analysis
	out, err := d.Conn().CommandContext(ctx, "cbmem", "-1").Output()
	if err != nil {
		s.Log("Failed to execute cbmem -1")
	} else {
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "bios_log.txt"), out, 0666); err != nil {
			s.Log("Failed to write bios log")
		}
	}
}
