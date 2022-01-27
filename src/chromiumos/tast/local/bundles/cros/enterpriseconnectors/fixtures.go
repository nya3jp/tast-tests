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

type Useless struct{}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosGaiaSignedInProdPolicyWPDownloadAllowExtra",
		Desc:     "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for downloads which allows immediate file transfers, large and encrypted files",
		Contacts: []string{"sseckler@google.com"},
		Impl: lacrosfixt.NewFixture(
			lacrosfixt.Rootfs,
			func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				username := s.RequiredVar("enterpriseconnectors.username1")
				password := s.RequiredVar("enterpriseconnectors.password1")
				return []chrome.Option{
					chrome.ExtraArgs("--disable-lacros-keep-alive"),
					chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
					chrome.ProdPolicy(),
				}, nil
			},
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
		Name:     "lacrosGaiaSignedInProdPolicyWPDownloadBlockExtra",
		Desc:     "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for downloads which blocks immediate file transfers, large and encrypted files",
		Contacts: []string{"sseckler@google.com"},
		Impl: lacrosfixt.NewFixture(
			lacrosfixt.Rootfs,
			func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				username := s.RequiredVar("enterpriseconnectors.username2")
				password := s.RequiredVar("enterpriseconnectors.password2")
				return []chrome.Option{
					chrome.ExtraArgs("--disable-lacros-keep-alive"),
					chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
					chrome.ProdPolicy(),
				}, nil
			},
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
		Name:     "lacrosGaiaSignedInProdPolicyWPUploadAllowExtra",
		Desc:     "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for uploads which allows immediate file transfers, large and encrypted files",
		Contacts: []string{"sseckler@google.com"},
		Impl: lacrosfixt.NewFixture(
			lacrosfixt.Rootfs,
			func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				username := s.RequiredVar("enterpriseconnectors.username3")
				password := s.RequiredVar("enterpriseconnectors.password3")
				return []chrome.Option{
					chrome.ExtraArgs("--disable-lacros-keep-alive"),
					chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
					chrome.ProdPolicy(),
				}, nil
			},
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
		Name:     "lacrosGaiaSignedInProdPolicyWPUploadBlockExtra",
		Desc:     "Fixture that allows usage of Lacros, with a gaia login with production policy and enabled WebProtect scanning for uploads which blocks immediate file transfers, large and encrypted files",
		Contacts: []string{"sseckler@google.com"},
		Impl: lacrosfixt.NewFixture(
			lacrosfixt.Rootfs,
			func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				username := s.RequiredVar("enterpriseconnectors.username4")
				password := s.RequiredVar("enterpriseconnectors.password4")
				return []chrome.Option{
					chrome.ExtraArgs("--disable-lacros-keep-alive"),
					chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
					chrome.ProdPolicy(),
				}, nil
			},
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
