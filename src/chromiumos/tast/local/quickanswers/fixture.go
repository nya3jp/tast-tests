// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickanswers

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "quickAnswersLoggedInFixture",
		Desc: "Chrome session logged in with OTA for Quick answres testing",
		Contacts: []string{
			"updowndota@google.com",
			"assitive-eng@google.com",
		},
		Vars: []string{"quickanswers.username", "quickanswers.password"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("quickanswers.username"),
					Pass: s.RequiredVar("quickanswers.password"),
				}),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "quickAnswersLoggedInFixtureLacros",
		Desc: "Lacros Chrome session logged in with OTA for Quick answres testing",
		Contacts: []string{
			"updowndota@google.com",
			"assitive-eng@google.com",
		},
		Vars: []string{"quickanswers.username", "quickanswers.password"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts := []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("quickanswers.username"),
					Pass: s.RequiredVar("quickanswers.password"),
				}),
			}
			return lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}
