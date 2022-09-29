// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornExtensions,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Unicorn user can add extension with parent permission",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "galenemco@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// This test has a long timeout because syncing settings can occasionally
		// take a long time.
		Timeout: 5 * time.Minute,
		VarDeps: []string{
			"family.parentEmail",
			"family.parentPassword",
		},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "familyLinkUnicornLogin",
		}, {
			Name:    "lacros",
			Val:     browser.TypeLacros,
			Fixture: "familyLinkUnicornLoginWithLacros",
		}},
	})
}

func boolPref(ctx context.Context, tconn *chrome.TestConn, prefName string) (bool, error) {
	var value struct {
		Value bool `json:"value"`
	}
	if err := tconn.Call(ctx, &value, "tast.promisify(chrome.settingsPrivate.getPref)", prefName); err != nil {
		return false, err
	}
	return value.Value, nil
}

func waitForBoolPrefValue(ctx context.Context, tconn *chrome.TestConn, prefName string, expectedValue bool, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		value, err := boolPref(ctx, tconn, prefName)
		if err != nil {
			return err
		}
		if value != expectedValue {
			return errors.Errorf("%q is not the right value", prefName)
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: timeout}); err != nil {
		return err
	}
	return nil
}

func waitForBoolPrefValueFromAshOrLacros(ctx context.Context, tconn *chrome.TestConn, bt browser.Type, prefName string, expectedValue bool, timeout time.Duration) error {
	// TODO(b/244515056): Move this function to a shared location.
	if bt == browser.TypeAsh {
		return waitForBoolPrefValue(ctx, tconn, prefName, expectedValue, timeout)
	}

	// Launch Lacros so that we can sync the preference and poll its status.
	l, err := lacros.Launch(ctx, tconn)
	if err != nil {
		return err
	}
	// Ensure we close Lacros before we return.
	defer l.Close(ctx)

	ltconn, err := l.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	return waitForBoolPrefValue(ctx, ltconn, prefName, expectedValue, timeout)
}

func UnicornExtensions(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}

	if err := waitForBoolPrefValueFromAshOrLacros(ctx, tconn, s.Param().(browser.Type), "profile.managed.extensions_may_request_permissions", true, 4*time.Minute); err != nil {
		s.Fatal("Failed to wait for pref: ", err)
	}

	if err := familylink.NavigateExtensionApprovalFlow(ctx, cr, tconn, s.Param().(browser.Type), s.RequiredVar("family.parentEmail"), s.RequiredVar("family.parentPassword")); err != nil {
		s.Fatal("Failed to add extension: ", err)
	}
}
