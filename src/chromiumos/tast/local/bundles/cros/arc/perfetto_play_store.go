// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoPlayStore,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A test to detect jank when the Play Store launches",
		Contacts:     []string{"yukashu@chromium.org", "sstan@chromium.org", "brpol@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			Val: playStoreSearchAndLaunchTestParams{
				MaxOptinAttempts: 2,
			},
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name: "vm",
			Val: playStoreSearchAndLaunchTestParams{
				MaxOptinAttempts: 2,
			},
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Data:    []string{"config.pbtxt"},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func PerfettoPlayStore(ctx context.Context, s *testing.State) {

	// Give cleanup actions a minute to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Optin to Play Store.
	testing.ContextLog(ctx, "Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// The PlayStore only popup automatically on first optin of an account.
	// Launch it here in case it's not the first optin.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("failed to launch Play Store", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	// Overwrite tracing_on flag in kernel tracefs.
	if err := a.ForceEnableTrace(ctx); err != nil {
		s.Fatal("Error on enabling perfetto trace")
	}

	// Push the config from configTxtPath to ARC device, run the perfetto basing on config,
	// and pull the trace result from ARC device to traceResultPath. This will return after
	// perfetto finish tracing or get error during tracing.
	if err := a.RunPerfettoTrace(ctx, s.DataPath("config.pbtxt"), filepath.Join(s.OutDir(), "pulledtrace")); err != nil {
		s.Fatal("Error on run perfetto trace")
	}

}
