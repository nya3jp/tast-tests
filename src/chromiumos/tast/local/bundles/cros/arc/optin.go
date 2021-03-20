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
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Optin,
		Desc: "A functional test that verifies OptIn flow",
		Contacts: []string{
			"arc-core@google.com",
			"mhasank@chromium.org",
			"khmel@chromium.org", // author.
		},
		Attr: []string{"group:mainline", "group:arc-functional"},
		Vars: []string{"ui.gaiaPoolDefault"}, // TODO(crbug.com/1183238): add VarDeps when supported.
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Params: []testing.Param{{
			Val:               3,
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabilized.
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "unstable",
			Val:               1,
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               3,
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabilized.
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "vm_unstable",
			Val:               1,
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 16 * time.Minute,
	})
}

// Optin tests optin flow.
func Optin(ctx context.Context, s *testing.State) {
	cr := setupChrome(ctx, s)
	defer cr.Close(ctx)

	s.Log("Performing optin")

	maxAttempts := s.Param().(int)
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

// writeLog writes the log to test output directory.
func writeLog(ctx context.Context, fileName string, data []byte) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Failed to get out dir")
		return
	}

	logPath := filepath.Join(dir, fileName)
	err := ioutil.WriteFile(logPath, data, 0644)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to save %q: %v", fileName, err)
		return
	}
	testing.ContextLog(ctx, "Saved ", fileName)
}

// dumpLogCat saves logcat to test output directory.
func dumpLogCat(ctx context.Context, attempt int) {
	cmd := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c", "/system/bin/logcat -d")
	log, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Failed to pull logcat: ", err)
	}

	fileName := fmt.Sprintf("logcat_%d.txt", attempt)
	writeLog(ctx, fileName, log)
}

// optinWithRetry retries optin on failure up to maxAttempts times.
func optinWithRetry(ctx context.Context, s *testing.State, cr *chrome.Chrome, maxAttempts int) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	attempts := 1
	for {
		err := optin.Perform(ctx, cr, tconn)
		if err == nil {
			break
		}

		dumpLogCat(ctx, attempts)

		if attempts >= maxAttempts {
			s.Fatal("Failed to optin: ", err)
		}

		s.Log("Retrying optin, previous attempt failed: ", err)
		attempts = attempts + 1

		// Opt out.
		if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to optout: ", err)
		}
	}
}
