// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/enterpriseconnectors/fixtvals"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	// Note that for these fixtures the credentials are configured with the specific policy parameters through the admin dpanel.
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPDownloadAllowExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for downloads which allows immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			fixtvals.PolicyParams{
				AllowsImmediateDelivery: true,
				AllowsUnscannableFiles:  true,
				ScansEnabledForDownload: true,
				ScansEnabledForUpload:   false,
			},
			"enterpriseconnectors.username1",
			"enterpriseconnectors.password1",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			lacrosfixt.LacrosDeployedBinary,
			"enterpriseconnectors.username1",
			"enterpriseconnectors.password1",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPDownloadBlockExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for downloads which blocks immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			fixtvals.PolicyParams{
				AllowsImmediateDelivery: false,
				AllowsUnscannableFiles:  false,
				ScansEnabledForDownload: true,
				ScansEnabledForUpload:   false,
			},
			"enterpriseconnectors.username2",
			"enterpriseconnectors.password2",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			lacrosfixt.LacrosDeployedBinary,
			"enterpriseconnectors.username2",
			"enterpriseconnectors.password2",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPUploadAllowExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for uploads which allows immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			fixtvals.PolicyParams{
				AllowsImmediateDelivery: true,
				AllowsUnscannableFiles:  true,
				ScansEnabledForDownload: false,
				ScansEnabledForUpload:   true,
			},
			"enterpriseconnectors.username3",
			"enterpriseconnectors.password3",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			lacrosfixt.LacrosDeployedBinary,
			"enterpriseconnectors.username3",
			"enterpriseconnectors.password3",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosGaiaSignedInProdPolicyWPUploadBlockExtra",
		Desc: "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for uploads which blocks immediate file transfers, large and encrypted files",
		Contacts: []string{
			"sseckler@google.com",
			"webprotect-eng@google.com",
		},
		Impl: CreateFixture(
			fixtvals.PolicyParams{
				AllowsImmediateDelivery: false,
				AllowsUnscannableFiles:  false,
				ScansEnabledForDownload: false,
				ScansEnabledForUpload:   true,
			},
			"enterpriseconnectors.username4",
			"enterpriseconnectors.password4",
		),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			lacrosfixt.LacrosDeployedBinary,
			"enterpriseconnectors.username4",
			"enterpriseconnectors.password4",
		},
	})
}

func CreateFixture(policyParams fixtvals.PolicyParams, user, pw string) testing.FixtureImpl {
	return lacrosfixt.NewComposedFixture(
		lacrosfixt.Rootfs,
		func(v lacrosfixt.FixtValue, pv interface{}) interface{} {
			return &fixtvals.FixtValue{
				PolicyParams: policyParams,
				FixtValue:    v,
			}
		},
		func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			username := s.RequiredVar(user)
			password := s.RequiredVar(pw)
			return []chrome.Option{
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
				chrome.ProdPolicy(),
			}, nil
		},
	)
}
