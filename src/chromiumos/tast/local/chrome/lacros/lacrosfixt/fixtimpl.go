// Copyright 2021 The ChromiumOS Authors
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

	// lacrosPerf is the same as lacros, but has some options specific for perf tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPerf",
		Desc:     "Lacros Chrome from a pre-built image, for perf tests",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			// Powerd is restarts because fwupd starts, which breaks power tests.
			// Disable the FirmwareUpdaterApp feature.
			return NewConfig(ChromeOptions(chrome.DisableFeatures("FirmwareUpdaterApp"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosForceComposition is the same as lacros but
	// forces composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPerfForceComposition",
		Desc:     "Lacros Chrome from a pre-built image with composition forced on",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
				chrome.DisableFeatures("FirmwareUpdaterApp"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosForceNonDelegation is the same as lacros but
	// forces delegated composition off as well as hw overlays.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPerfForceNonDelegated",
		Desc:     "Lacros Chrome from a pre-built image with both delegated compositing and hw overlays forced off",
		Contacts: []string{"petermcneeley@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
				chrome.LacrosDisableFeatures("DelegatedCompositing"),
				chrome.DisableFeatures("FirmwareUpdaterApp"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
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

	// lacrosWith100FakeApps is the same as the "lacros" fixture but
	// creates 100 fake apps for lacros that are shown in the OS launcher.
	// TODO(crbug.com/1309565): Remove this fixture if no longer used in any tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWith100FakeApps",
		Desc:     "Lacros Chrome from a pre-built image with 100 fake apps installed for lacros",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig().Opts()
		}),
		Parent:          "install100LacrosApps",
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
	// This is DEPRECATED. Use the "lacros" fixture instead.
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

	// lacrosOnly is a fixture to bring up Lacros as the only browser from the rootfs partition by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosOnly",
		Desc:     "Lacros Chrome from rootfs as the only browser",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(Mode(lacros.LacrosOnly)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosDisableSync is a fixture to bring up Lacros as the only browser from the rootfs partition by default, with disabled app sync.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosDisableSync",
		Desc:     "Lacros Chrome from rootfs as the only browser, with disabled app sync",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(chrome.ExtraArgs("--disable-sync"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosResourcesFileSharing is the same as lacros but has some special flags
	// for resources file sharing tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosResourcesFileSharing",
		Desc:     "Lacros Chrome with resources file sharing feature",
		Contacts: []string{"elkurin@chromium.org", "hidehiko@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(
				chrome.ExtraArgs("--enable-features=LacrosResourcesFileSharing"),
			)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosKeepAlive is the same as lacros but has KeepAlive enabled, i.e.
	// Lacros keeps running in the background even when the browser is closed.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosKeepAlive",
		Desc:     "Lacros Chrome with KeepAlive enabled",
		Contacts: []string{"mxcai@chromium.org", "hidehiko@chromium.org"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(KeepAlive(true)).Opts()
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
		Vars:     []string{"fakeVariationsChannel"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			channel := "beta"
			if val, ok := s.Var("fakeVariationsChannel"); ok {
				s.Log("Setting fake-variations-channel to ", val)
				channel = val
			}
			return NewConfig(ChromeOptions(
				chrome.LacrosExtraArgs("--fake-variations-channel="+channel),
				chrome.LacrosExtraArgs("--variations-server-url=https://clients4.google.com/chrome-variations/seed"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosOsFeedback is is similar to lacros but should be used
	// by tests that will launch lacros with OsFeedback enabled.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosOsFeedback",
		Desc:     "Lacros Chrome from a pre-built image with OsFeedback enabled",
		Contacts: []string{"wangdanny@google.com", "cros-feedback-app@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfig(ChromeOptions(chrome.ExtraArgs("--enable-features=OsFeedback"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}
