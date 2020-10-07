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

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Preopt,
		Desc: "A functional test that verifies ARC is fully pre-optimized and there is no pre-opt happening during the boot",
		Contacts: []string{
			"arc-performance@google.com",
			"khmel@chromium.org", // author.
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func Preopt(ctx context.Context, s *testing.State) {
	if err := performBootAndWaitForIdle(ctx, s); err != nil {
		s.Fatal("Failed to boot ARC: ", err)
	}

	// Read what ARC starts from the begining. If we start logcat now, it would likely miss many entries.
	logcatPath := filepath.Join(s.OutDir(), "logcat.txt")

	dump, err := ioutil.ReadFile(logcatPath)
	if err != nil {
		s.Fatal("Failed to read logcat: ", err)
	}

	templates := []string{
		`DexInv: --- BEGIN \'(.+)\' ---`,
		`dex2oat : /system/bin/dex2oat --dex-file=(.+) --output-vdex`}
	for _, template := range templates {
		m := regexp.MustCompile(template).FindAllStringSubmatch(string(dump), -1)
		for _, match := range m {
			res := match[1]
			if strings.HasPrefix(res, "/system/") ||
				strings.HasPrefix(res, "/vendor/") ||
				strings.HasPrefix(res, "/apex/") {
				s.Errorf("Found unpreoptimized system resource %q", res)
			} else {
				s.Logf("Found unpreoptimized non-system resource %q", res)
			}
		}
	}
}

func performBootAndWaitForIdle(ctx context.Context, s *testing.State) error {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return err
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while booting ARC: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return err
	}
	defer a.Close()

	s.Log("Wating for CPU idle")
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}

	return nil
}
