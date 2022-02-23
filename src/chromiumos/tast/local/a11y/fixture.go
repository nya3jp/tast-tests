// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

var liveCaptionOpts = []chrome.Option{
	chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
	chrome.EnableFeatures("OnDeviceSpeechRecognition"),
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInForLiveCaption",
		Desc:     "Logged into a user session for Live Caption",
		Contacts: []string{"hyungtaekim@chromium.org", "chrome-knowledge-eng@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return liveCaptionOpts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosLoggedInForLiveCaption",
		Desc:     "Logged into a user session for Live Caption with Lacros Primary",
		Contacts: []string{"hyungtaekim@chromium.org", "chrome-knowledge-eng@google.com"},
		Impl: lacrosfixt.NewFixture(lacrosfixt.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts := append(liveCaptionOpts,
				chrome.EnableFeatures("LacrosPrimary"),
				chrome.ExtraArgs("--disable-lacros-keep-alive", "--disable-login-lacros-opening"))
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{lacrosfixt.LacrosDeployedBinary},
	})
}
