// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

type testInfo struct {
	username string // username for Chrome login
	password string // password to login
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Enrollment,
		Desc:         "Check that DUT can enroll to alpha dmserver",
		Contacts:     []string{"rzakarian@google.com" /*, "cros-reporting-team@google.com"*/},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      8 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testInfo{
					username: "enterprise.Enrollment.user_name",
					password: "enterprise.Enrollment.password",
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{
			"enterprise.Enrollment.user_name",
			"enterprise.Enrollment.password",
		},
	})
}

func Enrollment(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 10 * time.Second // time reserved for cleanup.
	)
	param := s.Param().(testInfo)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	dmServerURL := param.dmserver

	// Log-in to Chrome
	cr, err := chrome.New(
		ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: username, Pass: password}),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(dmServerURL),
		chrome.ExtraArgs("--login-manager"),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)
	defer clearEnrollment(ctx, s)
}

func clearEnrollment(ctx context.Context, s *testing.State) error {
	cmdRunner := hwsec.NewCmdRunner()
	helper, err := hwsec.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	return nil
}
