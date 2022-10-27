// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "arc", "no_kernel_upstream"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 3*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func DeskTemplatesCUJ(ctx context.Context, s *testing.State) {
	// TODO(b/238645466): Remove `no_kernel_upstream` from SoftwareDeps once kernel_uprev boards are more stable.
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("DesksTemplates"),
		chrome.DisableFeatures("DeskTemplateSync", "FirmwareUpdaterApp"),
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
	recorder, err := cujrecorder.NewRecorder(ctx, cr, tconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}

	defer func(ctx context.Context) {
		if err := recorder.Close(ctx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}(cleanupCtx)

	if err := recorder.AddCommonMetrics(tconn, tconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Open PlayStore, Chrome and Files.
		appsList := []apps.App{apps.PlayStore, apps.Chrome, apps.FilesSWA}
		for _, app := range appsList {
			if err := apps.Launch(ctx, tconn, app.ID); err != nil {
				return errors.Wrapf(err, "failed to open %s", app.Name)
			}
		}

		if err := waitforAppsToLaunch(ctx, tconn, ac, appsList); err != nil {
			return errors.Wrap(err, "failed to wait for apps to launch")
		}

		// Enter overview mode.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "error in setting overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}
		defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

		// Find the "save desk as a template" button.
		saveDeskButton := nodewith.ClassName("SavedDeskSaveDeskButton").First()
		desksTemplatesGridView := nodewith.ClassName("SavedDeskLibraryView").First()

		if err := uiauto.Combine(
			"save a desk template",
			ac.DoDefault(saveDeskButton),
			// Wait for the desk templates grid shows up.
			ac.WaitUntilExists(desksTemplatesGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "error in saving a desk template")
		}

		// Exit overview mode.
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "unable to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Close Play Store.
		if err := optin.ClosePlayStore(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close Play Store")
		}

		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for close Play store action to be completed")
		}

		// Close all existing windows.
		if err := ash.CloseAllWindows(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all windows")
		}

		// Enter overview mode, and launch the saved desk template.
		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			return errors.Wrap(err, "unable to set overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Show saved desk template.
		libraryButton := nodewith.Name("Library")
		if err := uiauto.Combine(
			"show the saved desks template",
			ac.DoDefault(libraryButton),
			// Wait for the desks templates grid shows up.
			ac.WaitUntilExists(desksTemplatesGridView),
		)(ctx); err != nil {
			return errors.Wrap(err, "unable to show saved desks templates")
		}

		// Confirm there is one desk template.
		deskTemplatesInfo, err := ash.FindSavedDesks(ctx, ac)
		if err != nil {
			return errors.Wrap(err, "unable to find desk templates")
		}
		if len(deskTemplatesInfo) != 1 {
			return errors.Errorf("got %v desk template(s), there should be one desk template", len(deskTemplatesInfo))
		}

		// Find the the first desk template.
		firstDeskTemplate := nodewith.ClassName("SavedDeskItemView")
		newDeskMiniView :=
			nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Desk 1 (1)"))

		// Launch the saved desk template.
		if err := uiauto.Combine(
			"launch the saved desk template",
			ac.DoDefault(firstDeskTemplate),
			// Wait for the new desk to appear.
			ac.WaitUntilExists(newDeskMiniView),
		)(ctx); err != nil {
			return errors.Wrap(err, "unable to launch a desk template")
		}

		// Wait for apps to launch.
		if err := waitforAppsToLaunch(ctx, tconn, ac, appsList); err != nil {
			return errors.Wrap(err, "failed to wait for apps to launch")
		}

		// Wait for apps to be visible.
		if err := waitforAppsToBeVisible(ctx, tconn, ac, appsList); err != nil {
			return errors.Wrap(err, "failed to wait for apps to be visible")
		}

		// Exit overview mode.
		if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
			return errors.Wrap(err, "unable to exit overview mode")
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for overview animation to be completed")
		}

		// Verify that there are the app windows.
		ws, err := ash.GetAllWindows(ctx, tconn)
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

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}

// waitforAppsToLaunch waits for the given apps to launch.
func waitforAppsToLaunch(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := ash.WaitForApp(ctx, tconn, app.ID, 90*time.Second); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}

		// Some apps may take a long time to load such as Play Store. Wait for launch event to be completed.
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for the app launch event to be completed")
		}
	}
	return nil
}

// waitforAppsToBeVisible waits for the windows of the given apps to be visible.
func waitforAppsToBeVisible(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		// Wait for the launched app window to become visible.
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			if !w.IsVisible {
				return false
			}
			return strings.Contains(w.Title, app.Name)
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return errors.Wrapf(err, "%s app window not visible after launching", app.Name)
		}
	}

	return nil
}
