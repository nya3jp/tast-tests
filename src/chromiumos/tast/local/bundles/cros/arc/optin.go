// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// TODO(b/177341225): Stabilize optin test.
var optinUnstableModels = []string{
	"kled",
	"helios",
	"pantheon",
	"drawcia",
	"veyron_tiger",
	"volteer2",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Optin,
		Desc: "A functional test that verifies OptIn flow",
		Contacts: []string{
			"arc-core@google.com",
			"mhasank@chromium.org",
			"khmel@chromium.org", // author.
		},
		Attr: []string{"group:mainline"},
		Vars: []string{"ui.gaiaPoolDefault"}, // TODO(mhasank): add VarDeps when supported.
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Params: []testing.Param{{
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabilized.
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(optinUnstableModels...)),
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(optinUnstableModels...)),
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabilized.
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(optinUnstableModels...)),
		}, {
			Name:              "vm_unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(optinUnstableModels...)),
		}},
		Timeout: 16 * time.Minute,
	})
}

// Optin tests optin flow.
func Optin(ctx context.Context, s *testing.State) {
	cr := setupChrome(ctx, s)
	defer cr.Close(ctx)

	s.Log("Performing optin")

	const maxAttempts = 3 // TODO(b/177341225): remove after stabilized.
	optinWithRetry(ctx, s, cr, maxAttempts)
}

// setupChrome starts chrome with pooled GAIA account and ARC enabled.
func setupChrome(ctx context.Context, s *testing.State) *chrome.Chrome {
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	return cr
}

func dumpLogCat(ctx context.Context, s *testing.State, attempt int) {
	cmd := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "/system/bin/logcat -d")
	log, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Log("Failed to pull logcat: ", err)
	}

	fileName := fmt.Sprintf("logcat_%d.txt", attempt)
	logcatPath := filepath.Join(s.OutDir(), fileName)
	err = ioutil.WriteFile(logcatPath, log, 0644)
	if err != nil {
		s.Log("Failed to save logcat: ", err)
		return
	}
	s.Logf("Logcat saved to: %q", logcatPath)
}

// optinWithRetry retries optin on failure up to maxAttempts times.
func optinWithRetry(ctx context.Context, s *testing.State, cr *chrome.Chrome, maxAttempts int) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	attempts := 0
	for {
		err := optin.Perform(ctx, cr, tconn)
		if err == nil {
			break
		}
		attempts = attempts + 1
		dumpLogCat(ctx, s, attempts)
		if attempts >= maxAttempts {

			s.Fatal("Failed to optin. No more retries left: ", err)
		}
		s.Log("Retrying optin, previous attempt failed: ", err)

		// Opt out.
		if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to optout: ", err)
		}
	}
}
