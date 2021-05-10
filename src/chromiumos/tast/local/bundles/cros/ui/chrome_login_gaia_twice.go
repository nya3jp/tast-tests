// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeLoginGAIATwice,
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
		Timeout: 2*chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeLoginGAIATwice(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	cr.Close(ctx)

	cr, err = chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	cr.Close(ctx)
}
