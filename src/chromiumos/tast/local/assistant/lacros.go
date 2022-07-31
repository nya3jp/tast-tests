// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	// List of fixtures with Lacros enabled for assistant testing.
	testing.AddFixture(&testing.Fixture{
		Name: "assistantLacrosBase",
		Desc: "Chrome session with Lacros enabled for assistant testing",
		Contacts: []string{
			"yawano@google.com",
			"hyungtaekim@chromium.org",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(
				VerboseLogging(),
				ashNoNudgesExtraArg(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantLacros",
		Desc: "Assistant is enabled for Lacros",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantLacrosBase",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantLacrosPerf",
		Desc: "Assistant fixture for running performance test with Lacros",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:         "assistantLacros",
		Impl:           newPerfFixture(),
		PreTestTimeout: perfFixturePreTestTimeout,
	})
}
