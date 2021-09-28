// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPolicyLoggedIn",
		Desc:     "Fixture for a running FakeDMS with lacros",
		Contacts: []string{"mohamedaomar@google.com", "wtlee@chromium.org", "chromeos-commercial-remote-management@google.com"},
		Impl: launcher.NewComposedFixture(launcher.External, func(v launcher.FixtValue, pv interface{}) interface{} {
			return &struct {
				fakedms.HasFakeDMS
				launcher.FixtValue
			}{
				pv.(fakedms.HasFakeDMS),
				v,
			}
		}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
			if !ok {
				return nil, errors.New("parent is not a FakeDMS fixture")
			}
			opts := []chrome.Option{chrome.DMSPolicy(fdms.URL),
				chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"})}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "fakeDMS",
		Data:            []string{launcher.DataArtifact},
		Vars:            []string{launcher.LacrosDeployedBinary},
	})
}
