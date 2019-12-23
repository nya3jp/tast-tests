// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SharedFontFiles,
		Desc:     "Checks that the hostOS font files are shared with the guestOS and they are accessible",
		Contacts: []string{"matterchen@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "download",
			Pre:     crostini.StartedByDownload(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func SharedFontFiles(ctx context.Context, s *testing.State) {
	// Check for two things
	// 1. The /mnt/chromeos/fonts dir exists
	// 2. The fonts within /mnt/chromeos/fonts are part of the fontconfig path
	pre := s.PreValue().(crostini.PreData)
	cont := pre.Container

	const sharedFonts = "/mnt/chromeos/fonts"
	s.Log("1. Verifying mounted fonts dir exists")

	cmd := cont.Command(ctx, "ls", sharedFonts)
	var outBytes []byte
	var err error
	if outBytes, err = cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list fonts directory : ", err)
	}
	outString := string(outBytes)
	if len(outString) == 0 {
		s.Fatal("Fonts directory is empty")
	}

	s.Log("2. Verifying one of the available fonts comes from mounted fonts dir")
	cmd = cont.Command(ctx, "fc-list")
	if outBytes, err = cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to call fc-list : ", err)
	}
	outString = string(outBytes)
	if !strings.Contains(outString, sharedFonts) {
		s.Fatal("Host fonts not part of font-config path")
	}
}
