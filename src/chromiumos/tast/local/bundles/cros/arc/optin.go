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

type optinTestParam struct {
	username    string
	password    string
	maxAttempts int
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
		Attr: []string{"group:mainline", "group:arc-functional"},
		VarDeps: []string{"ui.gaiaPoolDefault",
			"arc.Optin.managed_username",
			"arc.Optin.managed_password"},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"play_store",
		},
		Params: []testing.Param{{
			Val: optinTestParam{
				maxAttempts: 1,
			},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			Val: optinTestParam{
				maxAttempts: 1,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name: "managed",
			Val: optinTestParam{
				username:    "arc.Optin.managed_username",
				password:    "arc.Optin.managed_password",
				maxAttempts: 1,
			},
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "managed_vm",
			Val: optinTestParam{
				username:    "arc.Optin.managed_username",
				password:    "arc.Optin.managed_password",
				maxAttempts: 1,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}

// Optin tests optin flow.
func Optin(ctx context.Context, s *testing.State) {
	param := s.Param().(optinTestParam)
	var gaiaLogin chrome.Option
	if param.username != "" {
		gaiaLogin = chrome.GAIALogin(chrome.Creds{User: s.RequiredVar(param.username), Pass: s.RequiredVar(param.password)})
	} else {
		gaiaLogin = chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))
	}

	cr, err := setupChrome(ctx, gaiaLogin)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Performing optin")

	if err := optin.PerformWithRetry(ctx, cr, param.maxAttempts); err != nil {
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
