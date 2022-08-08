// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcpipvideotest facilitates using the ArcPipVideoTest app.
package arcpipvideotest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// EstablishARCPIPVideo installs the ArcPipVideoTest app, launches it, and
// makes it play bear-320x240.h264.mp4 in PIP. That video must be listed in
// the Data field on test registration. Returns a cleanup action.
func EstablishARCPIPVideo(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, dataFS http.FileSystem, bigPIP bool) (action.Action, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var cleanupActionsInReverseOrder []action.Action
	cleanUp := func(ctx context.Context) error {
		var firstErr error
		for i := len(cleanupActionsInReverseOrder) - 1; i >= 0; i-- {
			if err := cleanupActionsInReverseOrder[i](ctx); firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	success := false
	defer func(ctx context.Context) {
		if success {
			return
		}
		if err := cleanUp(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up after detecting error condition: ", err)
		}
	}(cleanupCtx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close(cleanupCtx)

	if err := a.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		return nil, errors.Wrap(err, "failed installing app")
	}

	const pkgName = "org.chromium.arc.testapp.pictureinpicturevideo"
	act, err := arc.NewActivity(a, pkgName, ".VideoActivity")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create activity")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		act.Close()
		return nil
	})

	srv := httptest.NewServer(http.FileServer(dataFS))
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		srv.Close()
		return nil
	})

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse test server URL")
	}

	hostPort, err := strconv.Atoi(srvURL.Port())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse test server port")
	}

	androidPort, err := a.ReverseTCP(ctx, hostPort)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start reverse port forwarding")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		return a.RemoveReverseTCP(ctx, androidPort)
	})

	withVideo := arc.WithExtraString("video_uri", fmt.Sprintf("http://localhost:%d/bear-320x240.h264.mp4", androidPort))
	cantPlayThisVideo := d.Object(
		ui.Text("Can't play this video."),
		ui.PackageName(pkgName),
		ui.ClassName("android.widget.TextView"),
	)
	pollOpts := &testing.PollOptions{Timeout: 10 * time.Second}
	var pipWindow *ash.Window
	if err := action.Retry(3, func(ctx context.Context) (retErr error) {
		if err := act.Start(ctx, tconn, withVideo); err != nil {
			return errors.Wrap(err, "failed to start app")
		}
		defer func(ctx context.Context) {
			if retErr == nil {
				return
			}
			if err := act.Stop(ctx, tconn); err != nil {
				testing.ContextLog(ctx, "Failed to stop ARC app after failing to start it: ", err)
			}
		}(cleanupCtx)

		// Wait until the video is playing, or at least the app is
		// idle and not showing the message "Can't play this video."
		if err := d.WaitForIdle(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC app to be idle (before minimizing)")
		}
		if err := cantPlayThisVideo.WaitUntilGone(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for \"Can't play this video.\" message to be absent")
		}

		// The test activity enters PIP mode in onUserLeaveHint().
		if err := act.SetWindowState(ctx, tconn, arc.WindowStateMinimized); err != nil {
			return errors.Wrap(err, "failed to minimize app")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pipWindow, err = ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
			if err != nil {
				return errors.Wrap(err, "the PIP window hasn't been created yet")
			}
			return nil
		}, pollOpts); err != nil {
			return errors.Wrap(err, "failed to wait for PIP window")
		}

		if err := d.WaitForIdle(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC app to be idle (as PIP)")
		}

		return nil
	}, 0)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to create ARC PIP window")
	}
	cleanupActionsInReverseOrder = append(cleanupActionsInReverseOrder, func(ctx context.Context) error {
		return act.Stop(ctx, tconn)
	})

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the primary display info")
	}

	if bigPIP {
		// To resize the PIP window as reliably as possible,
		// use uiauto (not activity.ResizeWindow) and drag
		// from the corner (not the ARC++ PIP resize handle).

		pc := pointer.NewMouse(tconn)
		defer pc.Close()

		// The resizing drag begins this far from the corner
		// outward along each dimension. This offset ensures
		// that we drag the corner and not the resize handle.
		const pipCornerOffset = 5

		if err := pc.Drag(
			pipWindow.BoundsInRoot.TopLeft().Sub(coords.NewPoint(pipCornerOffset, pipCornerOffset)),
			pc.DragTo(info.WorkArea.TopLeft(), time.Second),
		)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to resize the PIP window")
		}

		pipWindow, err = ash.GetWindow(ctx, tconn, pipWindow.ID)
		if err != nil {
			return nil, errors.Wrap(err, "PIP window gone after resize")
		}

		// For code maintainability, just check a relatively permissive expectation for the
		// maximum size of the PIP window: it should be either strictly wider than 2/5 of
		// the work area width, or strictly taller than 2/5 of the work area height.
		if 5*pipWindow.TargetBounds.Width <= 2*info.WorkArea.Width && 5*pipWindow.TargetBounds.Height <= 2*info.WorkArea.Height {
			return nil, errors.Errorf("expected a bigger PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	} else {
		// For code maintainability, just check a relatively permissive expectation for the
		// minimum size of the PIP window: it should be either strictly narrower than 3/10
		// of the work area width, or strictly shorter than 3/10 of the work area height.
		if 10*pipWindow.TargetBounds.Width >= 3*info.WorkArea.Width && 10*pipWindow.TargetBounds.Height >= 3*info.WorkArea.Height {
			return nil, errors.Errorf("expected a smaller PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	}

	success = true
	return cleanUp, nil
}
