// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsRenewRefreshTokens,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Obtain new refresh tokens for pooled DriveFS GAIA logins",
		Contacts: []string{
			"austinct@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Timeout: 10 * time.Minute,
		Vars: []string{
			"drivefs.clientCredentials",
			"drivefs.accountPool",
		},
	})
}

func DrivefsRenewRefreshTokens(ctx context.Context, s *testing.State) {
	accounts := strings.Split(s.RequiredVar("drivefs.accountPool"), "\n")
	var newRefreshString strings.Builder

	for i, line := range accounts {
		account := strings.TrimSpace(line)
		if len(account) == 0 || strings.HasPrefix(account, "#") {
			continue
		}
		accountCredentials := strings.SplitN(account, ":", 2)
		if len(accountCredentials) != 2 {
			s.Errorf("Failed to extract account credentials on line %d", i+1)
			continue
		}
		username := accountCredentials[0]
		s.Run(ctx, username, func(ctx context.Context, s *testing.State) {
			cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: accountCredentials[1]}), chrome.ARCDisabled())
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(ctx)

			refreshToken, err := drivefs.RenewRefreshTokenForAccount(ctx, cr, s.RequiredVar("drivefs.clientCredentials"))
			if err != nil {
				s.Fatalf("Failed to get new refresh token for %q: %v", username, err)
			}

			fmt.Fprintf(&newRefreshString, "%s:%s\n", cr.NormalizedUser(), refreshToken)

			s.Logf("Obtained new refresh token for account %q", cr.NormalizedUser())
		})
	}

	s.Logf("Obtained all new refresh tokens, copy paste the following line into drivefs.yaml: %q", newRefreshString.String())
}
