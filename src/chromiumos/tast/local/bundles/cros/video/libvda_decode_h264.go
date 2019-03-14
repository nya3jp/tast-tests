// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
		Data:         []string{"test-25fps.h264"},
	})
}

func LibvdaDecodeH264(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-arcvm"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	args := []string{
		"--test_video_file=test-25fps.h264",
	}
	const testExec = "/usr/local/libexec/libvda_tests/libvda_unittests"

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(s.OutDir(), fmt.Sprintf("output_libvda_%d.txt", time.Now().Unix())))
	if err != nil {
		s.Fatalf("Failed to create logfile ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, testExec, args...)
	cmd.Stdout = f
	cmd.Stderr = f

	testing.ContextLogf(ctx, "Executing %s", shutil.EscapeSlice(cmd.Args))
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to run %v: %v", testExec, err)
	}
}
