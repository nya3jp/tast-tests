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
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabalized.
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(optinUnstableModels...)),
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(optinUnstableModels...)),
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"informational"}, // TODO(b/177341225): remove after stabalized.
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

func Optin(ctx context.Context, s *testing.State) {
	const (
		retryCount = 2
	)

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Performing optin")

	attempts := 0
	for {
		err := optin.Perform(ctx, cr, tconn)
		if err == nil {
			break
		}
		attempts = attempts + 1
		if attempts > retryCount {
			s.Fatal("Failed to optin. No more retries left: ", err)
		}
		s.Log("Retrying optin, previous attempt failed: ", err)

		// Opt out.
		if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
			s.Log("Failed to optout: ", err)
		}
	}

}
