// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeGaiaAPI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks credentials passing API works",
		Contacts: []string{
			"rsorokin@google.com",
			"cros-oac@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		VarDeps: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "sandbox",
			Val:  true,
		}},
	})
}

func ChromeGaiaAPI(ctx context.Context, s *testing.State) {
	useSandboxGaia := s.Param().(bool)
	options := []chrome.Option{chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")), chrome.DeferLogin()}
	if useSandboxGaia {
		options = append(options, chrome.UseSandboxGaia())
	}
	cr, err := chrome.New(
		ctx,
		options...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer cr.Close(ctx)
	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Could not get Chrome log reader: ", err)
	}
	defer logReader.Close()

	if err = cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	for {
		entry, err := logReader.Read()
		if err == io.EOF {
			break
		}
		if strings.Contains(entry.Content, "SamlHandler.onAPICall_") && entry.Severity == "ERROR" {
			s.Fatal("Found error in the Chrome log: ", entry.Content)
		}
	}
}
