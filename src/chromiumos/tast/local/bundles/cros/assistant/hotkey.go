// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hotkey,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Search+A Assistant hotkey to toggle launcher. Search+A hotkey is disabled if a device has a dedicated assistant key",
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "assistant_key",
				Val:               assistant.AccelAssistantKey,
				ExtraHardwareDeps: hwdep.D(hwdep.AssistantKey()),
			},
			{
				Name:              "search_plus_a",
				Val:               assistant.AccelSearchPlusA,
				ExtraHardwareDeps: hwdep.D(hwdep.NoAssistantKey()),
			},
		},
	})
}

func Hotkey(ctx context.Context, s *testing.State) {
	accel := s.Param().(assistant.Accelerator)
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer func() {
		if err := assistant.Cleanup(ctx, s.HasError, cr, tconn); err != nil {
			s.Fatal("Failed to disable Assistant: ", err)
		}
	}()

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to confirm that launcher is closed before a test: ", err)
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		s.Fatal("Failed to toggle Assistant UI with hotkey: ", err)
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Half); err != nil {
		s.Fatal("Failed to confirm that launcher got opened with hotkey: ", err)
	}

	if err := assistant.ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		s.Fatal("Failed to toggle Assistant UI with hotkey: ", err)
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to confirm that launcher got closed with hotkey: ", err)
	}
}
