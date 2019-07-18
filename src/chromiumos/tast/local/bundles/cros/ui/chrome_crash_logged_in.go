// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os/exec"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while logged in",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func ChromeCrashLoggedIn(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.CrashNormalMode(),
		chrome.ExtraArgs("--enable-crash-reporter-for-testing"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	testing.ContextLog(ctx, "Running metric_client to set up consent")
	err = exec.Command("/usr/bin/metrics_client", "-C").Run()
	if err != nil {
		s.Fatal("Error setting metrics consent: ", err)
	}

	files, err := chromecrash.KillAndGetCrashFiles(ctx)
	if err != nil {
		s.Fatal("Couldn't kill Chrome or get files: ", err)
	}

	if err = chromecrash.FindCrashFilesIn(chromecrash.CryptohomeCrashPattern, files); err != nil {
		s.Error("Crash files weren't written to cryptohome: ", err)
	}
}
