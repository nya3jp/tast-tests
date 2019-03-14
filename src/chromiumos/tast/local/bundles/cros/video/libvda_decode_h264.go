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
		Func:     LibvdaDecodeH264,
		Desc:     "Checks H264 video decoding using libvda's mojo connection to GAVDA is working",
		Contacts: []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"android", "chrome_internal", "chrome_login", caps.HWDecodeH264},
		Data:         []string{"test-25fps.h264"},
	})
}

func LibvdaDecodeH264(ctx context.Context, s *testing.State) {
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

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(s.OutDir(), "output_libvda_h264.txt"))
	if err != nil {
		s.Fatalf("Failed to create logfile: ", err)
	}
	defer f.Close()

	const testExec = "/usr/local/libexec/libvda_tests/libvda_unittests"
	cmd := testexec.CommandContext(ctx, testExec, "--test_video_file="+s.DataPath("test-25fps.h264"))
	cmd.Stdout = f
	cmd.Stderr = f

	s.Log("Executing ", shutil.EscapeSlice(cmd.Args))
	if err := cmd.Run(); err != nil {
		s.Fatalf("Failed to run %v: %v", testExec, err)
	}
}
