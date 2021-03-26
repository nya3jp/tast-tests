// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
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

		if err := optin.DumpLogCat(ctx, strconv.Itoa(attempts)); err != nil {
			s.Logf("WARNING: Failed to dump logcat: %s", err)
		}

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
