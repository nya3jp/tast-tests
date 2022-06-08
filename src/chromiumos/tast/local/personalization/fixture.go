// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithDarkLightMode",
		Desc: "Login with Personalization Hub and Dark Light mode enabled",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("DarkLightMode")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithGaiaLogin",
		Desc: "Login using Gaia account with Personalization Hub enabled",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("ambient.username"),
					Pass: s.RequiredVar("ambient.password"),
				}),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"ambient.username",
			"ambient.password",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithRgbKeyboard",
		Desc: "Login with Personalization Hub and RGB Keyboard enabled",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("RgbKeyboard")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "personalizationWithGooglePhotosWallpaper",
		Desc: "Login with Gaia account with Google Photos Wallpaper enabled",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		// Setting Google Photos wallpapers requires that Chrome be logged in with
		// a user from an account pool which has been preconditioned to have a
		// Google Photos library with specific photos/albums present. Note that sync
		// is disabled to prevent flakiness caused by wallpaper cross device sync.
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALoginPool(s.RequiredVar("wallpaper.googlePhotosAccountPool")),
				chrome.EnableFeatures("WallpaperGooglePhotosIntegration"),
				chrome.ExtraArgs("--disable-sync"),
			}, nil
		}),
		Vars: []string{
			"wallpaper.googlePhotosAccountPool",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}
