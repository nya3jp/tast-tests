// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

type preoptTestParam struct {
	// Template to search dex2oat runs in logcat
	template string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Preopt,
		Desc: "Verifies that ARC++ is fully pre-optimized and there is no pre-opt happening during the boot",
		Contacts: []string{
			"khmel@chromium.org", // author.
			"arc-performance@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			Val: preoptTestParam{
				template: `/system/bin/dex2oat .*--dex-file=(.+?) --`,
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
			Val: preoptTestParam{
				template: `DexInv: --- BEGIN \'(.+?)\' ---`,
			},
		}},
		Timeout: 4 * time.Minute,
	})
}

func Preopt(ctx context.Context, s *testing.State) {
	param := s.Param().(preoptTestParam)

	if err := performBootAndWaitForIdle(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to boot ARC: ", err)
	}

	// Read what ARC logs from the beginning. If we start logcat now, it would likely miss many entries.
	logcatPath := filepath.Join(s.OutDir(), "logcat.txt")

	dump, err := ioutil.ReadFile(logcatPath)
	if err != nil {
		s.Fatal("Failed to read logcat: ", err)
	}

	m := regexp.MustCompile(param.template).FindAllStringSubmatch(string(dump), -1)
	for _, match := range m {
		res := match[1]
		if !strings.HasPrefix(res, "/data/") {
			s.Errorf("Found unpreoptimized system resource %q", res)
		}
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
