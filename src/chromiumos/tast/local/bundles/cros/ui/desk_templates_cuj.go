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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
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
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild", "group:cuj"},
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
		chrome.EnableFeatures("DesksTemplates", "EnableSavedDesks"),
		chrome.DisableFeatures("DeskTemplateSync"),
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
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}

	defer func(ctx context.Context) {
		if err := recorder.Close(ctx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}(cleanupCtx)

	if err := recorder.AddCollectedMetrics(tconn, browser.TypeAsh, cujrecorder.DeprecatedMetricConfigs()...); err != nil {
		s.Fatal("Failed to add recorded metrics: ", err)
	}

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Define the UI elements.
		saveDeskAsTemplateButton := nodewith.ClassName("SaveDeskTemplateButton").Nth(0)
		savedTemplateGridView := nodewith.ClassName("SavedDeskGridView").Nth(0)
		saveDeskForLaterButton := nodewith.ClassName("SaveDeskTemplateButton").Nth(1)
		savedForLaterDeskGridView := nodewith.ClassName("SavedDeskGridView").Nth(1)
		savedTemplate := nodewith.ClassName("SavedDeskItemView").Nth(0)
		savedTemplateNameView := nodewith.ClassName("SavedDeskNameView").Name("Template 1")
		savedDesk := nodewith.ClassName("SavedDeskItemView").Nth(1)
		savedDeskNameView := nodewith.ClassName("SavedDeskNameView").Name("Saved Desk 1")
		savedTemplateMiniView :=
			nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Template 1"))
		savedDeskMiniView :=
			nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Saved Desk 1"))
		libraryButton := nodewith.ClassName("ZeroStateIconButton").Name("Library")

		// Open PlayStore, Chrome and Files.
		appsList := []apps.App{apps.PlayStore, apps.Chrome, apps.Files}
		for _, app := range appsList {
			if err := apps.Launch(ctx, tconn, app.ID); err != nil {
				return errors.Wrapf(err, "failed to open %s", app.Name)
			}
			if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
				return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
			}
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for app launch events to be completed")
		}

		// Define keyboard to perform keyboard shortcuts.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot create keyboard")
		}
		defer kb.Close()

		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}
		defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

		// Save a desk template.
		if err := uiauto.Combine(
			"save a desk template",
			ac.LeftClick(saveDeskAsTemplateButton),
			// Wait for the template grid to show up.
			ac.WaitUntilExists(savedTemplateGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to save a desk template")
		}

		// Type "Template 1" and press "Enter".
		if err := kb.Type(ctx, "Template 1"); err != nil {
			return errors.Wrap(err, "cannot type 'Template 1'")
		}
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "cannot press 'Enter'")
		}

		// Exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Close all existing windows.
		if ws, err := ash.GetAllWindows(ctx, tconn); err == nil {
			for _, w := range ws {
				if err := w.CloseWindow(ctx, tconn); err != nil {
					return errors.Wrapf(err, "failed to close window (%+v)", w)
				}
			}
		} else {
			return errors.Wrap(err, "failed to get all open windows")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for app close events to be completed")
		}

		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Show the saved template grid.
		if err := uiauto.Combine(
			"show the saved template grid",
			ac.LeftClick(libraryButton),
			// Wait for the saved grid to show up.
			ac.WaitUntilExists(savedTemplateGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to show the saved template grid")
		}

		// Launch the saved template of kTemplate type.
		if err := uiauto.Combine(
			"launch the saved desk template of kTemplate type",
			// Verify the existence of the saved template.
			ac.WaitUntilExists(savedTemplateNameView),
			ac.LeftClick(savedTemplate),
			// Wait for the new desk to appear.
			ac.WaitUntilExists(savedTemplateMiniView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to launch a saved template of kTemplate type")
		}

		// Press enter.
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "cannot press 'Enter'")
		}

		// Exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Wait for the app to launch.
		for _, app := range appsList {
			if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
				return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
			}
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for app launch events to be completed")
		}

		// Verify that there are the app windows.
		if ws, err := ash.GetAllWindows(ctx, tconn); err == nil {
			if len(ws) != len(appsList) {
				return errors.Wrapf(err, "found inconsistent number of window(s): got %v, want %v", len(ws), len(appsList))
			}
		} else {
			return errors.Wrap(err, "failed to get all open windows")
		}

		// Re-enter overview mode, so we can save a desk for later.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Save a desk for later.
		if err := uiauto.Combine(
			"save a desk for later",
			ac.LeftClick(saveDeskForLaterButton),
			// Wait for the saved for later grid to show up.
			ac.WaitUntilExists(savedForLaterDeskGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to save a desk for later")
		}

		// Type "Saved Desk 1" and press "Enter".
		if err := kb.Type(ctx, "Saved Desk 1"); err != nil {
			return errors.Wrap(err, "cannot type 'Saved Desk 1'")
		}
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "cannot press 'Enter'")
		}

		// Exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Enter overview mode, and launch the saved desk.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Show the saved desk template grid.
		if err := uiauto.Combine(
			"show the saved desk template grid",
			ac.LeftClick(libraryButton),
			// Wait for the saved for later grid to show up.
			ac.WaitUntilExists(savedForLaterDeskGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to show the saved for later grid")
		}

		// Launch the saved desk of kSaveAndRecall type.
		if err := uiauto.Combine(
			"launch the saved desk of kSaveAndRecall type",
			// Verify the existence of the saved desk.
			ac.WaitUntilExists(savedDeskNameView),
			ac.LeftClick(savedDesk),
			// Wait for the new desk to appear.
			ac.WaitUntilExists(savedDeskMiniView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to launch a saved desk of kSaveAndRecall type")
		}

		// Press enter.
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "cannot press 'Enter'")
		}

		// Exit overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "failed to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Wait for the app to launch.
		for _, app := range appsList {
			if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
				return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
			}
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for app launch events to be completed")
		}

		// Verify that there are the app windows.
		if ws, err := ash.GetAllWindows(ctx, tconn); err == nil {
			if len(ws) != len(appsList) {
				return errors.Wrapf(err, "found inconsistent number of window(s): got %v, want %v", len(ws), len(appsList))
			}
		} else {
			return errors.Wrap(err, "failed to get all open windows")
		}

		// Close all existing windows.
		if ws, err := ash.GetAllWindows(ctx, tconn); err == nil {
			for _, w := range ws {
				if err := w.CloseWindow(ctx, tconn); err != nil {
					return errors.Wrapf(err, "failed to close window (%+v)", w)
				}
			}
		} else {
			return errors.Wrap(err, "failed to get all open windows")
		}

		// Remove the active desk.
		if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to remove the active desk")
		}

		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "failed to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Show the saved desk template grid.
		if err := uiauto.Combine(
			"show the saved desk template grid",
			ac.LeftClick(libraryButton),
			// Wait for the saved for later grid to show up.
			ac.WaitUntilExists(savedForLaterDeskGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to show the saved for later grid")
		}

		// Verify that there is one saved template of kTemplate type.
		if savedDeskViewInfo, err := ash.FindDeskTemplates(ctx, ac); err == nil {
			if len(savedDeskViewInfo) != 1 {
				return errors.Wrapf(err, "found inconsistent number of saved template(s): got %v, want 1", len(savedDeskViewInfo))
			}
		} else {
			return errors.Wrap(err, "failed to find saved templates")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
