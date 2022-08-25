// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com", "tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
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

func getBoolPref(ctx context.Context, c *chrome.TestConn, prefName string) (bool, error) {
	var value struct {
		Value bool `json:"value"`
	}
	if err := c.Call(ctx, &value, "tast.promisify(chrome.settingsPrivate.getPref)", prefName); err != nil {
		return false, err
	}
	return value.Value, nil
}

func waitForBoolPrefValue(ctx context.Context, c *chrome.TestConn, prefName string, expectedValue bool, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		value, err := getBoolPref(ctx, c, prefName)
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

func waitForBoolPrefValueFromAshOrLacros(ctx context.Context, c *chrome.TestConn, isLacros bool, prefName string, expectedValue bool, timeout time.Duration) error {
	if !isLacros {
		return waitForBoolPrefValue(ctx, c, prefName, expectedValue, timeout)
	}
	l, err := lacros.Launch(ctx, c)
	if err != nil {
		return err
	}
	defer l.Close(ctx)

	tconn, err := l.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	return waitForBoolPrefValue(ctx, tconn, prefName, expectedValue, timeout)
}

func UnicornExtensions(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	isLacros := s.Param().(browser.Type) == browser.TypeLacros

	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}

	if err := waitForBoolPrefValueFromAshOrLacros(ctx, tconn, isLacros, "profile.managed.extensions_may_request_permissions", true, 4*time.Minute); err != nil {
		s.Fatal("Failed to wait for pref: ", err)
	}

	if err := familylink.NavigateExtensionApprovalFlow(ctx, cr, tconn, s.Param().(browser.Type), s.RequiredVar("family.parentEmail"), s.RequiredVar("family.parentPassword")); err != nil {
		s.Fatal("Failed to add extension: ", err)
	}
}
