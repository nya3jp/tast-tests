// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	// Note that for these fixtures the credentials are configured with the specific policy parameters through the admin dpanel.
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPEnabledAllowExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning which allows immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			"enterpriseconnectors.username1",
			"enterpriseconnectors.password1",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"enterpriseconnectors.username1",
			"enterpriseconnectors.password1",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPEnabledBlockExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning which blocks immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			"enterpriseconnectors.username2",
			"enterpriseconnectors.password2",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"enterpriseconnectors.username2",
			"enterpriseconnectors.password2",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPDisabled",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and disabled WebProtect scanning",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			"enterpriseconnectors.username3",
			"enterpriseconnectors.password3",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"enterpriseconnectors.username3",
			"enterpriseconnectors.password3",
		},
	})
}

func CreateFixture(user, pw string) testing.FixtureImpl {
	return chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		username := s.RequiredVar(user)
		password := s.RequiredVar(pw)
		return lacrosfixt.NewConfig(
			lacrosfixt.ChromeOptions(
				chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
				chrome.ProdPolicy(),
			),
		).Opts()
	})
}
