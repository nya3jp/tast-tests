// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndroidAndWeb,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test assistant to open Android app over web app",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantWithArc",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AndroidAndWeb(ctx context.Context, s *testing.State) {
	const (
		QueryOpenGoogleNews = "Open Google News"
	)

	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome
	a := fixtData.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if _, err := assistant.SendTextQuery(ctx, tconn, QueryOpenGoogleNews); err != nil {
		s.Fatal("Failed to send Assistant text query: ", err)
	}

	if err := assistant.WaitForGoogleNewsWebActivation(ctx, tconn); err != nil {
		s.Fatal("Failed to wait Google News Web gets active: ", err)
	}

	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to close all windows: ", err)
	}

	if err := assistant.InstallTestApkAndWaitReady(ctx, tconn, a); err != nil {
		s.Fatal("Failed to install a test apk: ", err)
	}

	if _, err := assistant.SendTextQuery(ctx, tconn, QueryOpenGoogleNews); err != nil {
		s.Fatal("Failed to send Assistant text query: ", err)
	}

	if err := assistant.WaitForGoogleNewsAppActivation(ctx, tconn); err != nil {
		s.Fatal("Failed to wait Google News Android gets active: ", err)
	}
}
