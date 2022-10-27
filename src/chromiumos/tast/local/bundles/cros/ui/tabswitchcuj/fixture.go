// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tabswitchcuj

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "tabSwitchCUJWPR",
		Desc: "Base fixture for TabSwitchCUJ with WPR",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl:            wpr.NewFixture(WPRArchiveName, wpr.Replay),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{WPRArchiveName},
	})

	testing.AddFixture(&testing.Fixture{
		Name: "tabSwitchCUJWPRAsh",
		Desc: "Composed fixture for TabSwitchCUJ with WPR",
		Contacts: []string{
			"amusbach@chromium.org",
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return s.ParentValue().(wpr.FixtValue).FOpt()(ctx, s)
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "tabSwitchCUJWPR",
	})

	testing.AddFixture(&testing.Fixture{
		Name: "tabSwitchCUJWPRLacros",
		Desc: "Composed fixture for TabSwitchCUJ with WPR",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts, err := s.ParentValue().(wpr.FixtValue).FOpt()(ctx, s)
			if err != nil {
				return nil, err
			}
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "tabSwitchCUJWPR",
	})
}
