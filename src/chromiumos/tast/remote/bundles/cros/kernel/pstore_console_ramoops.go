// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
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
		Attr:         []string{"group:mainline", "informational"},
	})
}

// PstoreConsoleRamoops Writes a blurb to kernel logs, reboots, and checks to
// make sure console-ramoops has that message. This confirms that pstore is
// properly saving the previous kernel log.
func PstoreConsoleRamoops(ctx context.Context, s *testing.State) {
	const testKey = "tast is rebooting for PstoreConsoleRamoops"

	d := s.DUT()

	if err := linuxssh.WriteFile(ctx, d.Conn(), "/dev/kmsg", []byte(testKey), 0644); err != nil {
		s.Fatal("Failed to write message to /dev/kmsg on the DUT: ", err)
	}

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	ramoopsDir := filepath.Join(s.OutDir(), "console-ramoops")
	if err := linuxssh.GetFile(ctx, d.Conn(), "/sys/fs/pstore/", ramoopsDir, linuxssh.PreserveSymlinks); err != nil {
		s.Fatal("Failed to copy ramoops dir after reboot on the DUT: ", err)
	}

	files, err := ioutil.ReadDir(ramoopsDir)
	if err != nil {
		s.Fatal("Failed to list ramoops directory: ", err)
	}

	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "console-ramoops") {
			continue
		}

		f, err := ioutil.ReadFile(filepath.Join(ramoopsDir, file.Name()))
		if err != nil {
			s.Fatal("Failed to read ramoops file: ", err)
		}

		goodSigRegexp := regexp.MustCompile(testKey)
		if !goodSigRegexp.Match(f) {
			s.Error("Couldn't find reboot signature in ramoops file ", file.Name())
		}

		return
	}

	s.Error("Couldn't find reboot signature in any console-ramoops file")
}
