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
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(optinUnstableModels...)),
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(optinUnstableModels...)),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(optinUnstableModels...)),
		}, {
			Name:              "vm_unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(optinUnstableModels...)),
		}},
		Timeout: 4 * time.Minute,
	})
}

func Optin(ctx context.Context, s *testing.State) {
	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.AuthPool(s.RequiredVar("ui.gaiaPoolDefault")),
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
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Error("Failed to optin: ", err)
	}
}
