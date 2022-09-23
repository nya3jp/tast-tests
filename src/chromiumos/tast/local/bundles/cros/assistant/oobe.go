// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"strings"

	"chromiumos/tast/local/assistant"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Oobe,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Oobe flow for a gaia which has used Assistant and whose voice match is ready",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"yawano@google.com", "assistive-eng@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "assistantOobeUsedVmReady",
	})
}

func Oobe(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(*assistant.OobeFixtData)
	oobeCtx := fixtData.OobeCtx

	// Gaia used for this fixture has used Assistant before. It goes to related info screen.
	if err := assistant.GoThroughOobeScreen(ctx, &assistant.AssistantScreenRelatedInfoAgree, &oobeCtx); err != nil {
		s.Fatal("Failed to go through related info Assistant screen with agree option: ", err)
	}

	if err := assistant.GoThroughOobeScreen(ctx, &assistant.AssistantScreenHotwordAgree, &oobeCtx); err != nil {
		s.Fatal("Failed to go through hotword Assistant screen with agree option: ", err)
	}

	if err := assistant.GoThroughOobeScreen(ctx, &assistant.AssistantScreenHotwordReady, &oobeCtx); err != nil {
		s.Fatal("Failed to go through hotword ready sceeen: ", err)
	}

	if err := assistant.GoThroughOobeScreen(ctx, &assistant.ThemeSelectionScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through theme selection screen: ", err)
	}

	if err := assistant.GoThroughOobeScreen(ctx, &assistant.OobeCompleteScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through oobe complete screen: ", err)
	}

	tconn, err := oobeCtx.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a test API connection: ", err)
	}

	// Confirm that Assistant works.
	//
	// We use an equation whose answer is 3 digit numbers:
	// - A small number (e.g. 1+1=2) can appear in random part of a response HTML (e.g. CSS).
	// - Assistant inserts a space for 4 or more digit numbers (e.g. 12 345). Avoid it as they
	//   might change the behavior in the future.
	queryStatus, err := assistant.SendTextQuery(ctx, tconn, "123 + 456 =")
	if err != nil {
		s.Fatal("Failed to send a query: ", err)
	}

	// 579 = 123 + 456
	if !strings.Contains(queryStatus.QueryResponse.HTML, "579") {
		s.Fatal("A response does not contain an expected response: 579")
	}
}
