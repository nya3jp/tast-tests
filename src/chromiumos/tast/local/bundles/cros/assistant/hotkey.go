// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Hotkey,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Assistant hotkey to toggle launcher",
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistant",
	})
}

func Hotkey(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.FixtData)
	cr := fixtData.Chrome
	accel := fixtData.Hotkey

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	assistantUI := nodewith.HasClass("AssistantDialogPlate")
	if err := action.Combine("Press hotkey and confirm that it toggles Assistant UI visibility",
		uiauto.New(tconn).WaitUntilGone(assistantUI),
		func(ctx context.Context) error { return assistant.ToggleUIWithHotkey(ctx, tconn, accel) },
		uiauto.New(tconn).WaitUntilExists(assistantUI),
		func(ctx context.Context) error { return assistant.ToggleUIWithHotkey(ctx, tconn, accel) },
		uiauto.New(tconn).WaitUntilGone(assistantUI),
	)(ctx); err != nil {
		s.Fatal("Failed to toggle Assistant UI with a hotkey: ", err)
	}
}
