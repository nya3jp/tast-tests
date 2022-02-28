// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Preopt,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that ARC++ is fully pre-optimized and there is no pre-opt happening during the boot",
		Contacts: []string{
			"khmel@chromium.org", // author.
			"arc-performance@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func Preopt(ctx context.Context, s *testing.State) {
	if err := performBootAndWaitForIdle(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to boot ARC: ", err)
	}

	if err := arc.CheckNoDex2Oat(s.OutDir()); err != nil {
		s.Fatal("Failed to verify dex2oat was not running: ", err)
	}
}

func performBootAndWaitForIdle(ctx context.Context, outDir string) error {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome browser process")
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, outDir)
	if err != nil {
		return errors.Wrap(err, "failed to connect to ARC")
	}
	defer a.Close(ctx)

	// Wait for CPU is idle once dex2oat is heavy operation and idle CPU would
	// indicate that heavy boot operations are done.
	testing.ContextLog(ctx, "Wating for CPU idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait CPU is idle")
	}

	return nil
}
