// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeskTemplatesCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the performance of desks templates",
		Contacts: []string{
			"yzd@chromium.org",
			"aprilzhou@chromium.org",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 2*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func DeskTemplatesCUJ(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates"),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	// Setup for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	// Set up metrics recorder for TPS calculation
	recorder, err := cuj.NewRecorder(ctx, cr, nil)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}

	defer func() {
		if err := recorder.Close(ctx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Opens PlayStore, Chrome and Files.
		appsList := []apps.App{apps.PlayStore, apps.Chrome, apps.Files}
		for _, app := range appsList {
			if err := apps.Launch(ctx, tconn, app.ID); err != nil {
				return errors.Wrapf(err, "%s can't be opened", app.Name)
			}
			if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
				return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
			}
		}

		if err := ac.WaitForLocation(nodewith.Root())(ctx); err != nil {
			return errors.Wrap(err, "error in waiting for app launch events to be completed")
		}

		// Enters overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "error in setting overview mode")
		}

		if err := ac.WaitForLocation(nodewith.Root())(ctx); err != nil {
			return errors.Wrap(err, "error in waiting for overview animation to be completed")
		}

		// Find the "save desk as a template" button.
		saveDeskButton := nodewith.ClassName("SaveDeskTemplateButton")
		desksTemplatesGridView := nodewith.ClassName("DesksTemplatesGridView")

		if err := uiauto.Combine(
			"save a desk template",
			ac.LeftClick(saveDeskButton),
			// Wait for the desk templates grid shows up.
			ac.WaitUntilExists(desksTemplatesGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "error in saving a desk template")
		}

		// Exits overview mode.
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "unable to exit overview mode")
		}

		// Close all existing windows.
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "unable to get all open windows")
		}
		for _, w := range ws {
			if err := w.CloseWindow(ctx, tconn); err != nil {
				return errors.Wrapf(err, "unable to close window (%+v)", w)
			}
		}

		// Enters overview mode, and launch the saved desk template.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "unable to set overview mode")
		}

		// Find the "Templates" button.
		templatesButton := nodewith.Name("Templates")

		// Show saved desk template.
		if err := uiauto.Combine(
			"show the saved desks template",
			ac.LeftClick(templatesButton),
			// Wait for the desks templates grid shows up.
			ac.WaitUntilExists(desksTemplatesGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "unable to show saved desks templates")
		}

		// Confirm there is one desk template.
		deskTemplatesInfo, err := ash.FindDeskTemplates(ctx, ac)
		if err != nil {
			return errors.Wrap(err, "unable to find desk templates")
		}
		if len(deskTemplatesInfo) != 1 {
			return errors.Errorf("got %v desk template(s), there should be one desk template", len(deskTemplatesInfo))
		}

		// Find the the first desk template.
		firstDeskTemplate := nodewith.ClassName("DesksTemplatesItemView")
		newDeskMiniView :=
			nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Desk 1 (1)"))

		// Launch the saved desk template.
		if err := uiauto.Combine(
			"launch the saved desk template",
			ac.LeftClick(firstDeskTemplate),
			// Wait for the new desk to appear.
			ac.WaitUntilExists(newDeskMiniView),
		)(ctx); err != nil {
			return errors.Wrap(err, "unable to launch a desk template")
		}

		// Exits overview mode.
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "unable to exit overview mode")
		}

		// Verifies that there are the app windows.
		ws, err = ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "unable to get all open windows")
		}

		if len(ws) != len(appsList) {
			return errors.Errorf("got %v window(s), should have %v windows", len(ws), len(appsList))
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}

	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
