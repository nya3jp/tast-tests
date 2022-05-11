// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

func init() {
	// lacros uses rootfs lacros, which is the recommend way to use lacros
	// in Tast tests, unless you have a specific use case for using lacros from
	// another source.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacros",
		Desc:     "Lacros Chrome from a pre-built image",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig().Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosAudio is the same as lacros but has some special flags for audio
	// tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosAudio",
		Desc:     "Lacros Chrome from a pre-built image with camera/microphone permissions",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(
				chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--autoplay-policy=no-user-gesture-required"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosWith100FakeApps is the same as lacros but
	// creates 100 fake apps that are shown in the ash-chrome launcher.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWith100FakeApps",
		Desc:     "Lacros Chrome from a pre-built image with 100 fake apps installed",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig().Opts()
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosForceComposition is the same as lacros but
	// forces composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosForceComposition",
		Desc:     "Lacros Chrome from a pre-built image with composition forced on",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(chrome.ExtraArgs("--enable-hardware-overlays=\"\""))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosForceDelegation is the same as lacros but
	// forces delegated composition.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosForceDelegated",
		Desc:     "Lacros Chrome from a pre-built image with delegated compositing forced on",
		Contacts: []string{"petermcneeley@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(
				chrome.LacrosExtraArgs("--enable-gpu-memory-buffer-compositor-resources"),
				chrome.LacrosEnableFeatures("DelegatedCompositing"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosOmaha is a fixture to enable Lacros by feature flag in Chrome.
	// This does not require downloading a binary from Google Storage before the test.
	// It will use the currently available fishfood release of Lacros from Omaha.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosOmaha",
		Desc:     "Lacros Chrome from omaha",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(Selection(lacros.Omaha)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosPrimary is a fixture to bring up Lacros as a primary browser from the rootfs partition by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPrimary",
		Desc:     "Lacros Chrome from rootfs as a primary browser",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(Mode(lacros.LacrosPrimary)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosPrimaryDisableSync is a fixture to bring up Lacros as a primary browser from the rootfs partition by default and disables app sync.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPrimaryDisableSync",
		Desc:     "Lacros Chrome from rootfs as a primary browser and disables app sync",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(Mode(lacros.LacrosPrimary), ChromeOptions(
				chrome.ExtraArgs("--disable-sync"),
			)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosUIKeepAlive is similar to lacros but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line, and this test assuming that Lacros will be keep alive
	// in the background even if the browser is turned off.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosKeepAlive",
		Desc:     "Lacros Chrome from a pre-built image using the UI and the Lacros chrome will stay alive even when the browser terminated",
		Contacts: []string{"mxcai@chromium.org", "hidehiko@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(KeepAlive(true), Mode(lacros.LacrosPrimary)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosVariation is similar to lacros but should be used
	// by variation smoke tests that will launch lacros with variation service enabled,
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosVariationEnabled",
		Desc:     "Lacros with variation service enabled",
		Contacts: []string{"yjt@google.com", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(Mode(lacros.LacrosPrimary), ChromeOptions(
				chrome.LacrosExtraArgs("--fake-variations-channel=beta"),
				chrome.LacrosExtraArgs("--variations-server-url=https://clients4.google.com/chrome-variations/seed"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}
