// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const bundlingNotificationApkFileName = "ArcNotificationTest2.apk"

func init() {
	testing.AddTest(&testing.Test{
		Func:         BundlingNotifications,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that bundling notifications appear in notification centre can be interacted with",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{bundlingNotificationApkFileName},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      15 * time.Minute,
		Fixture:      "arcBootedWithDisableSyncFlags",
	})
}

// bnTestResource represents test resources for test arc.BundlingNotifications.
type bnTestResource struct {
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	ui     *uiauto.Context
	outDir string
}

// BundlingNotifications tests that notifications from gmail will be bundled.
func BundlingNotifications(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	nsApp, err := apputil.NewNotificationShowcaseApp(ctx, cr, a, tconn, kb, s.DataPath(bundlingNotificationApkFileName))
	if err != nil {
		s.Fatal("Failed to create Notification Showcase app: ", err)
	}
	defer nsApp.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := nsApp.Install(ctx); err != nil {
		s.Fatal("Failed to install Notification Showcase app: ", err)
	}

	res := &bnTestResource{
		cr:     cr,
		tconn:  tconn,
		ui:     uiauto.New(tconn),
		outDir: s.OutDir(),
	}

	if _, err := nsApp.Launch(ctx); err != nil {
		s.Fatal("Failed to launch Notification Showcase app: ", err)
	}

	if err := generateNotification(ctx, res, nsApp); err != nil {
		s.Fatal("Failed to generate notification: ", err)
	}
	defer ash.CloseNotifications(cleanupCtx, tconn)

	if err := verifyBundlingNotification(ctx, res); err != nil {
		s.Fatal("Failed to verify bundling notification: ", err)
	}
}

// generateNotification generates a notification and verifies it appears on the notification center.
func generateNotification(ctx context.Context, res *bnTestResource, nsApp *apputil.NotificationShowcase) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	// Opening Quick Settings is necessary in order to verify notification node later on.
	if err := quicksettings.Show(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to show quick settings")
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, "generate_notification")
		quicksettings.Hide(cleanupCtx, res.tconn)
	}(cleanupCtx)

	if err := nsApp.ComposeGroupedNotification(ctx, 5); err != nil {
		return errors.Wrap(err, "failed to compose notification")
	}

	if _, err := ash.WaitForNotification(ctx, res.tconn, 5*time.Second, ash.WaitIDContains("ranker_group")); err != nil {
		return errors.Wrap(err, "failed to wait for grouped notification")
	}

	// Remove notifications except for grouped one.
	notifications, err := ash.Notifications(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get existing notifications")
	}

	for _, n := range notifications {
		if !strings.Contains(n.ID, "ranker_group") {
			if err := browser.ClearNotification(ctx, res.tconn, n.ID); err != nil {
				return errors.Wrap(err, "failed to remove notification")
			}
		}
	}

	return nil
}

// verifyBundlingNotification verifies that the bundled notification can be expanded, collapsed and click features.
func verifyBundlingNotification(ctx context.Context, res *bnTestResource) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second*10)
	defer cancel()

	// TODO(b/224685870): Use UIAutomator to get the button position when the issue is solved.
	const (
		buttonOffsetX       = -28
		buttonOffsetY       = 44
		notificationOffsetX = -20
	)

	if err := quicksettings.Collapse(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to show quick settings")
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, "verify_notification")
		quicksettings.Hide(cleanupCtx, res.tconn)
	}(cleanupCtx)

	notificationView := nodewith.HasClass("ArcNotificationContentView").First()

	// notificationBounds returns the bound of grouped notification.
	notificationBounds := func(ctx context.Context) (*coords.Rect, error) {
		if err := res.ui.WaitForLocation(notificationView)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to find notification view")
		}

		bounds, err := res.ui.Location(ctx, notificationView)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the notification bounds")
		}

		return bounds, nil
	}

	// Verify expand / collapse features.
	for _, feature := range []string{"expand", "collapse"} {
		bounds, err := notificationBounds(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the notification bounds when %q", feature)
		}

		// TODO(b/224685870): Use UIAutomator to get the button position when the issue is solved.
		arrowButtonOffset := coords.NewPoint(buttonOffsetX, buttonOffsetY)
		if err := mouse.Click(res.tconn, bounds.TopRight().Add(arrowButtonOffset), mouse.LeftButton)(ctx); err != nil {
			return errors.Wrapf(err, "failed to click the arrow button when %q", feature)
		}

		if err := res.ui.WaitForLocation(notificationView)(ctx); err != nil {
			return errors.Wrapf(err, "failed to get the notification bounds when %q", feature)
		}
	}

	// Verify click feature.
	bounds, err := notificationBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the notification bounds when clicking")
	}

	// TODO(b/224685870): Use UIAutomator to get the button position when the issue is solved.
	notificationLoc := bounds.CenterPoint().Add(coords.NewPoint(notificationOffsetX, 0))
	if err := uiauto.Combine("move mouse and click notification",
		mouse.Move(res.tconn, notificationLoc, time.Second),
		mouse.Click(res.tconn, notificationLoc, mouse.LeftButton),
	)(ctx); err != nil {
		return err
	}
	return nil
}
