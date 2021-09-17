// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixtures holds fixtures for lacros tests.
package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

const speedometerWPRArchive = "speedometer.wprgo"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "speedometerWPR",
		Desc:            "Base fixture for speedometer with WPR",
		Contacts:        []string{"edcourtney@chromium.org", "hidehiko@chromium.org"},
		Impl:            wpr.NewFixture(speedometerWPRArchive, wpr.Replay),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{speedometerWPRArchive},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "speedometerWPRLacros",
		Desc:     "Composed fixture for speedometer with WPR",
		Contacts: []string{"edcourtney@chromium.org", "hidehiko@chromium.org"},
		Impl: wpr.NewLacrosFixture(launcher.External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Parent:          "speedometerWPR",
		Data:            []string{launcher.DataArtifact},
		Vars:            []string{launcher.LacrosDeployedBinary},
	})
}
