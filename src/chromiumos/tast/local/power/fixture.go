// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	chromeQuickMetricsCollectionArg := "--external-metrics-collection-interval=1"

	// Sets version to "unknown" to ensure smart dim uses builtin model.
	chromeSmartDimBuiltinModelArg := "--enable-features=SmartDimExperimentalComponent:smart_dim_experimental_version/unknown"

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeFastHistograms",
		Desc:     "Logged into a user session and enabled quick metrics collection for fast histogram validation",
		Contacts: []string{"alanlxl@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeQuickMetricsCollectionArg),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeFastHistogramsAndBuiltinSmartDimModel",
		Desc:     "Similar to chromeFastHistograms, plus force chrome to use builtin smart dim models",
		Contacts: []string{"alanlxl@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeQuickMetricsCollectionArg),
				chrome.ExtraArgs(chromeSmartDimBuiltinModelArg),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosFastHistogramsAndBuiltinSmartDimModel",
		Desc:     "Similar to chromeFastHistogramsAndBuiltinSmartDimModel but on lacros",
		Contacts: []string{"alanlxl@google.com"},
		Impl: lacrosfixt.NewFixture(lacros.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(chromeQuickMetricsCollectionArg),
				chrome.ExtraArgs(chromeSmartDimBuiltinModelArg),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{lacrosfixt.LacrosDeployedBinary},
	})

}
