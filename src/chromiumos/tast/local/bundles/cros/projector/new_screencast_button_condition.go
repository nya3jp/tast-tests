// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NewScreencastButtonCondition,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks conditions where the new screencast button is disabled or enabled",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		// The soda software dep excludes VMs because we want
		// to verify that SODA is installed on non-VM devices.
		// Don't use ondevice_speech because that would make
		// this test a tautology.
		SoftwareDeps: []string{"chrome", "soda"},
		Timeout:      5 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func NewScreencastButtonCondition(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	if err := projector.DismissOnboardingDialog(ctx, tconn); err != nil {
		s.Fatal("Failed to close the onboarding dialog: ", err)
	}

	if err := audio.WaitForDevice(ctx, audio.InputStream); err != nil {
		s.Log("Microphone is unavailable, verifying new screencast button is disabled")
		if err = projector.VerifyNewScreencastButtonDisabled(ctx, tconn, "Turn on microphone"); err != nil {
			s.Fatal("Microphone is unavailable, but new screencast button is enabled: ", err)
		}
		// Pass the test and exit prematurely.
		return
	}

	s.Log("Microphone is enabled, verifying that the new screencast button is enabled")
	ui := uiauto.New(tconn)
	newScreencastButton := nodewith.Name("New screencast").Role(role.Button).Focusable()
	if err := ui.WaitUntilExists(newScreencastButton)(ctx); err != nil {
		s.Fatal("Microphone is enabled, but new screencast button is disabled: ", err)
	}

	if err := a11y.VerifySodaInstalled(ctx); err != nil {
		s.Fatal("New screencast button should be disabled if SODA is not installed")
	}

	errorTooltip := nodewith.Name("Speech recognition not supported").Role(role.GenericContainer)
	if err := ui.WaitUntilExists(errorTooltip)(ctx); err == nil {
		s.Fatal("Speech recognition not supported tooltip should not appear if SODA is installed")
	}

	cantInstallSpeechFiles := nodewith.Name("Can't install speech files").Role(role.StaticText)
	if err := ui.WaitUntilExists(cantInstallSpeechFiles)(ctx); err == nil {
		s.Fatal("Can't install speech files dialog should not appear if SODA is installed")
	}
}
