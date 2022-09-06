// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IgtChamelium,
		Desc: "Verifies IGT Chamelium test binaries run successfully",
		Contacts: []string{
			"chromeos-gfx-display@google.com",
			"markyacoub@google.com",
		},
		SoftwareDeps: []string{"drm_atomic", "igt", "no_qemu"},
		Attr:         []string{"group:graphics", "graphics_chameleon_igt"},
		Fixture:      "chromeGraphicsIgt",
		Params: []testing.Param{{
			Name: "kms_chamelium",
			Val: graphics.IgtTest{
				Exe: "kms_chamelium",
			},
			Timeout:   15 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}},
	})
}

func IgtChamelium(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(graphics.IgtTest)
	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.Exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	testPath := filepath.Join("chamelium", testOpt.Exe)
	isExitErr, exitErr, err := graphics.IgtExecuteTests(ctx, testPath, f)

	isError, outputLog := graphics.IgtProcessResults(testOpt.Exe, f, isExitErr, exitErr, err)

	if isError {
		s.Error(outputLog)
	} else {
		s.Log(outputLog)
	}
}
