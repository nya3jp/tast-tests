// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package edu3dmodelingcuj contains the test code for EDU3DModeling CUJ.
package edu3dmodelingcuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/spera/edu3dmodelingcuj/tinkercad"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	tinkerCadTerm  = "Tinkercad"
	fileNamePrefix = "Copy of"
)

// Run runs the EDU3DModelingCUJ test.
func Run(ctx context.Context, cr *chrome.Chrome, isTablet bool, bt browser.Type, outDir, traceConfigPath, sampleDesignURL, rotateIconPath string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
	var uiHandler cuj.UIActionHandler
	if isTablet {
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	defer uiHandler.Close()

	testing.ContextLog(ctx, "Start to get browser start time")
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, isTablet, bt)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros test API conn")
		}
		br = l.Browser()
	}
	defer cuj.CloseAllWindows(ctx, tconn)

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Start recording actions")
	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}
	if traceConfigPath != "" {
		recorder.EnableTracing(outDir, traceConfigPath)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return errors.Wrap(err, "failed to get users Download path")
	}

	// Shorten the context to cleanup resources.
	cleanupResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := recorder.Run(ctx, func(ctx context.Context) (retErr error) {
		account := cr.Creds().User
		tinkerCad := tinkercad.NewTinkerCad(tconn, kb, br, rotateIconPath)

		// Open TinkerCAD on the chrome.
		if err := tinkerCad.Open(ctx); err != nil {
			return errors.Wrapf(err, "failed to open TinkerCAD URL %v", cuj.TinkerCadSignInURL)
		}
		defer func(ctx context.Context) {
			faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
			tinkerCad.Close(ctx)
		}(cleanupResourceCtx)

		// Maximize the TinkerCAD window to show all the browser UI elements for precise clicking.
		if !isTablet {
			// Find the TinkerCAD browser window.
			window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return (w.WindowType == ash.WindowTypeBrowser || w.WindowType == ash.WindowTypeLacros) && strings.Contains(w.Title, tinkerCadTerm)
			})
			if err != nil {
				return errors.Wrap(err, "failed to find the TinkerCAD window")
			}
			if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMaximized); err != nil {
				// Just log the error and try to continue.
				testing.ContextLog(ctx, "Try to continue the test even though maximizing the TinkerCAD window failed: ", err)
			}
		}

		// Login to TinkerCAD with google oauth.
		if err := tinkerCad.Login(account)(ctx); err != nil {
			return errors.Wrap(err, "failed to login to TinkerCAD")
		}

		// Open another tab than switch back to TinkerCAD.
		testing.ContextLog(ctx, "Open another tab in same browser")
		conn, err := br.NewConn(ctx, cuj.WikipediaMainURL)
		if err != nil {
			return errors.Wrapf(err, "failed to open URL %s", cuj.WikipediaMainURL)
		}
		defer conn.Close()
		defer conn.CloseTarget(ctx)
		if err := uiHandler.SwitchToChromeTabByName(tinkerCadTerm)(ctx); err != nil {
			// Sometimes it failed to get correct tab name after registration, so do it with tab index.
			if err := uiHandler.SwitchToChromeTabByIndex(0)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch tab")
			}
		}

		// Delete existing copies if previous tests failed to delete them.
		if err := tinkerCad.Delete(ctx, fileNamePrefix); err != nil {
			testing.ContextLog(ctx, "Failed to delete existing copies: ", err)
		}

		// Copy the design from URL than delete it at the final step.
		designName, err := tinkerCad.Copy(ctx, sampleDesignURL)
		if err != nil {
			return errors.Wrapf(err, "failed to copy the sample design from %s", sampleDesignURL)
		}
		defer func(ctx context.Context) {
			// If case fails, dump the last screen before deleting the design.
			faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_tree")
			tinkerCad.Delete(ctx, designName)
		}(cleanupResourceCtx)

		// Get editor window's rectangular region for other actions usage.
		rect, err := tinkerCad.GetEditorWindowRect(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the editor window location")
		}
		tinkerCad.EditorWinRect = rect

		// Run the design editing actions.
		if err := uiauto.NamedCombine("3D modeling on TinkerCAD",
			// Disable the grid so uidetection can successfully detect the icon.
			tinkerCad.DisableGrid(),
			// Add primitive shapes, move and rotate them.
			tinkerCad.AddShapeAndRotate("text"),
			tinkerCad.AddShapeAndRotate("cone"),
			tinkerCad.AddShapeAndRotate("pyramid"),
			// Enter fullscreen.
			kb.AccelAction("fullscreen"),
			// Switch to perspective view.
			tinkerCad.Visualize(tinkercad.ViewPerspective),
			// Select all blocks and roate them together.
			tinkerCad.RotateAll(),
			// Rotate the design by viewcube.
			tinkerCad.RotateViewCube(),
			// Exit fullscreen.
			kb.AccelAction("fullscreen"),
			// Export the design and verifies it.
			tinkerCad.ExportAndVerify(ctx, downloadsPath, designName),
			// Switch between tabs.
			uiHandler.SwitchToChromeTabByName("Wikipedia"),
			uiHandler.SwitchToChromeTabByName(tinkerCadTerm),
		)(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to conduct the recorder task")
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		return errors.Wrap(err, "failed to record the data")
	}
	if err := pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}
	return nil
}
