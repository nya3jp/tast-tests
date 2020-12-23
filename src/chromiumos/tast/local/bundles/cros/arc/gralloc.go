// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gralloc,
		Desc:         "Test ARC++ gralloc implementation",
		Contacts:     []string{"stevensd@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func Gralloc(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 5*time.Second)
	defer cancel()

	tempDir, err := a.TempDir(ctx)
	if err != nil {
		s.Fatal("Failed to create tempdir: ", err)
	}
	defer func() {
		if err := a.RemoveAll(fullCtx, tempDir); err != nil {
			s.Error("Failed to clean up tempdir: ", err)
		}
	}()

	ranTest := false
	for _, arch := range []string{"amd64", "arm", "x86"} {
		// The test binary is part of media-libs/arc-cros-gralloc. To update the test
		// binary, build and deploy that package.
		const testName = "gralloctest"
		const crosBinDir = "/usr/local/bin/"
		archTest := testName + "_" + arch

		if err := a.PushFile(ctx, path.Join(crosBinDir, archTest), tempDir); err != nil {
			s.Logf("Skipping %s", arch)
			continue
		}
		s.Logf("Running %s", archTest)

		output, testErr := a.Command(ctx, path.Join(tempDir, archTest), "all").CombinedOutput(testexec.DumpLogOnError)

		if err := ioutil.WriteFile(path.Join(s.OutDir(), archTest), output, 0644); err != nil {
			s.Log(string(output))
			s.Logf("Failed to write %s logs: %v", archTest, err)
		}

		if testErr != nil {
			s.Fatalf("%s failed: %v", archTest, testErr)
		} else {
			s.Logf("%s completed successfully", archTest)
		}
		ranTest = true
	}

	if !ranTest {
		s.Fatal("Failed to run any tests")
	}
}
