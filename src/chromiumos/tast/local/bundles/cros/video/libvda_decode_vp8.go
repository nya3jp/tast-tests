// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LibvdaDecodeVP8,
		Desc:         "Checks VP8 video decoding using libvda's Mojo connection to GAVDA is working",
		Contacts:     []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.HWDecodeVP8},
		Data:         []string{"test-25fps.vp8"},
	})
}

func LibvdaDecodeVP8(ctx context.Context, s *testing.State) {
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// This flag enables the libvda D-Bus service, and should work even on ARC++ devices.
		"--enable-arcvm",
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Create the output file that the test log is dumped to.
	f, err := os.Create(filepath.Join(s.OutDir(), "output_libvda_vp8.txt"))
	if err != nil {
		s.Fatalf("Failed to create logfile: ", err)
	}
	defer f.Close()

	const testExec = "/usr/local/libexec/libvda_tests/libvda_unittest"
	cmd := testexec.CommandContext(ctx, testExec, "--test_video_file="+s.DataPath("test-25fps.vp8"))
	cmd.Stdout = f
	cmd.Stderr = f

	s.Log("Executing ", shutil.EscapeSlice(cmd.Args))
	// TODO(alexlau): Consider using the helper function to get failed test cases if necessary.
	if err := cmd.Run(); err != nil {
		s.Fatalf("Failed to run %v: %v", testExec, err)
	}
}
