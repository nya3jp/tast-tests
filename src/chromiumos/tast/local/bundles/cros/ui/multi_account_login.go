// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultiAccountLogin,
		Desc: "Sequentially log in with a list of accounts to keep them active",
		Contacts: []string{
			"abergman@google.com",
			"chromeos-perf-reliability-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.MultiAccountLogin.accounts"},
		Timeout:      60 * time.Minute,
	})
}

func MultiAccountLogin(ctx context.Context, s *testing.State) {
	accounts := s.RequiredVar("ui.MultiAccountLogin.accounts")
	creds, err := parseCreds(accounts)
	if err != nil {
		s.Fatal("Error parsing account list: ", err)
	}
	s.Logf("Available accounts: %s", creds)

	for _, cred := range creds {
		gaia := chrome.GAIALogin(cred)

		cr, err := chrome.New(ctx, gaia)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		cr.Close(ctx)
	}
}

func parseCreds(creds string) ([]chrome.Creds, error) {
	var cs []chrome.Creds
	for i, line := range strings.Split(creds, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		ps := strings.SplitN(line, ":", 2)
		if len(ps) != 2 {
			return nil, errors.Errorf("failed to parse credential list: line %d", i+1)
		}
		cs = append(cs, chrome.Creds{
			User: ps[0],
			Pass: ps[1],
		})
	}
	return cs, nil
}
