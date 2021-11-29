// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Optin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A functional test that verifies OptIn flow",
		Contacts: []string{
			"arc-core@google.com",
			"mhasank@chromium.org",
			"khmel@chromium.org", // author.
		},
		Attr:    []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

// Optin tests optin flow.
func Optin(ctx context.Context, s *testing.State) {
	const (
		// If a single variant is flaky, please promote this to test params and increase the
		// attempts only for that specific variant instead of updating the constant for all.
		// See crrev.com/c/2979454 for an example.
		maxAttempts = 1
	)

	gaiaLogin := chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))

	cr, err := setupChrome(ctx, gaiaLogin)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Performing optin")

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin: ", err)
	}
}

// setupChrome starts chrome with pooled GAIA account and ARC enabled.
func setupChrome(ctx context.Context, gaiaLogin chrome.Option) (*chrome.Chrome, error) {
	cr, err := chrome.New(ctx,
		gaiaLogin,
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return nil, err
	}
	return cr, nil
}
