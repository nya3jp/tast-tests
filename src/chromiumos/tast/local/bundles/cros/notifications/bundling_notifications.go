// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil/notificationshowcase"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// bundlingNotificationApkFileName is the name of the apk file for Notification Showcase app.
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
		Timeout:      5 * time.Minute,
		Fixture:      "arcBootedWithDisableSyncFlags",
	})
}

// bundlingNotificationTestResource represents test resources for test arc.BundlingNotifications.
type bundlingNotificationTestResource struct {
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	ui     *uiauto.Context
	outDir string
}

// bundlingNotificationTestType indicates the type of notification used for test case arc.BundlingNotifications.
type bundlingNotificationTestType string

const (
	// single represents a single notification.
	single bundlingNotificationTestType = "single"
	// multiple represents a bundling notification.
	multiple bundlingNotificationTestType = "multi"
)

// BundlingNotifications tests that notifications from Notification Showcase app will be bundled.
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
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, err := notificationshowcase.NewApp(ctx, a, tconn, kb, s.DataPath(bundlingNotificationApkFileName))
	if err != nil {
		s.Fatal("Failed to create Notification Showcase app: ", err)
	}
	defer app.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := app.Install(ctx); err != nil {
		s.Fatal("Failed to install Notification Showcase app: ", err)
	}

	res := &bundlingNotificationTestResource{
		cr:     cr,
		tconn:  tconn,
		ui:     uiauto.New(tconn),
		outDir: s.OutDir(),
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	if err := screenRecorder.Start(ctx, tconn); err != nil {
		s.Log("Failed to start ScreenRecorder: ", err)
	}
	defer uiauto.ScreenRecorderStopSaveRelease(cleanupCtx, screenRecorder, filepath.Join(s.OutDir(), "record.webm"))

	if _, err := app.Launch(ctx); err != nil {
		s.Fatal("Failed to launch Notification Showcase app: ", err)
	}

	// Generating single notification and get its height.
	// Remove all notifications before generating a new one.
	if err := ash.CloseNotifications(ctx, res.tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	if err := generateNotification(ctx, res, app, single); err != nil {
		s.Fatal("Failed to generate notification: ", err)
	}
	defer ash.CloseNotifications(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, res.outDir, s.HasError, res.cr, "bundling_notifications_single")

	singleNotificationBounds, err := notificationBounds(ctx, res)
	if err != nil {
		s.Fatal("Failed to get notification bounds for single notification: ", err)
	}
	s.Logf("Height of single notification: %d", singleNotificationBounds.Height)

	s.Log("Generating bundling notifications")
	// Remove all notifications before generating a new ones.
	if err := ash.CloseNotifications(ctx, res.tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	if err := generateNotification(ctx, res, app, multiple); err != nil {
		s.Fatal("Failed to generate notification: ", err)
	}
	defer ash.CloseNotifications(cleanupCtx, tconn)
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, s.HasError, res.cr, "verify_notification")
		quicksettings.Hide(ctx, res.tconn)
	}(cleanupCtx)

	groupedNotificationBounds, err := notificationBounds(ctx, res)
	if err != nil {
		s.Fatal("Failed to get notification bounds for grouped notification: ", err)
	}

	// Some DUTs might not include all 4 notifications into single grouped notification.
	// Therefore, we use the equation "HeightOfGroupNotification ÷ HeightOfSingleNotification"
	// to retrieve the number of notifications inside the grouped notification.
	notificationGroupCnt := groupedNotificationBounds.Height / singleNotificationBounds.Height
	s.Logf("%d notifications are grouped into one notification", notificationGroupCnt)

	s.Log("Verifying bundling notifications")
	for _, mode := range []verifyMode{collapse, expand, click} {
		s.Logf("Verifying %q functionality", mode)
		if err := verifyGroupedNotifications(ctx, res, singleNotificationBounds.Height, notificationGroupCnt, mode); err != nil {
			s.Fatalf("Failed to verify notification with mode %q: %s", mode, err)
		}
	}
}

// generateNotification generates a notification and verifies it appears on the notification center.
func generateNotification(ctx context.Context, res *bundlingNotificationTestResource, app *notificationshowcase.NotificationShowcase, notification bundlingNotificationTestType) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Opening Quick Settings is necessary in order to verify notification node later on.
	if err := quicksettings.Show(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to show quick settings")
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.cr, fmt.Sprintf("generate_%s_notification", notification))

	if err := quicksettings.Collapse(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to collapse quick settings")
	}

	var (
		// notificationID is the ID of the notification generated by the system automatically.
		notificationID string
		// notificationCount is the number of notifications that Notification Showcase app will generate.
		notificationCount int
	)
	switch notification {
	case single:
		// "null" represents the notification ID used for the single notification.
		notificationID = "null"
		notificationCount = 1
	case multiple:
		// "ranker_group" represents the notification ID used for the grouped notification.
		notificationID = "ranker_group"
		notificationCount = 4
	default:
		return errors.New("unsupported notification type")
	}

	if err := app.ComposeGroupedNotification(ctx, notificationCount); err != nil {
		return errors.Wrap(err, "failed to compose grouped notification")
	}

	// Ensure wanted notification(s) is shown on the notification center.
	if _, err := ash.WaitForNotification(ctx, res.tconn, 15*time.Second, ash.WaitIDContains(notificationID)); err != nil {
		return errors.Wrap(err, "failed to wait for grouped notification")
	}

	return nil
}

// notificationBounds returns the bound of grouped notification.
func notificationBounds(ctx context.Context, res *bundlingNotificationTestResource) (*coords.Rect, error) {
	// notificationView is the view of grouped notification.
	// Bundling notification might be separated into multiple notification views.
	// Therefore, we use First() to select the first one.
	notificationView := nodewith.HasClass("ArcNotificationContentView").First()

	if err := res.ui.WaitForLocation(notificationView)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find notification view")
	}

	bounds, err := res.ui.Location(ctx, notificationView)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the notification bounds")
	}

	return bounds, nil
}

// verifyMode is the mode of verification for test case notifications.BundlingNotifications.
type verifyMode string

const (
	// expand verifies that the notification can be expanded.
	expand verifyMode = "expand"
	// collapse verifies that the notification can be collapsed.
	collapse verifyMode = "collapse"
	// click verifies that the notification will disappear after being clicked.
	click verifyMode = "click"
)

// verifyGroupedNotifications verifies the notification with the given mode.
func verifyGroupedNotifications(ctx context.Context, res *bundlingNotificationTestResource, singleNotificationHeight, notificationGroupCnt int, mode verifyMode) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.cr, fmt.Sprintf("verify_grouped_notification_with_%s_mode", mode))

	var (
		bundledBoundsBefore *coords.Rect
		bundledBoundsAfter  *coords.Rect
		err                 error
	)

	if bundledBoundsBefore, err = notificationBounds(ctx, res); err != nil {
		return errors.Wrapf(err, "failed to get the notification bounds before %q", mode)
	}

	// TODO(b/224685870): Use UIAutomator to get the button position dynamically when it supports multi-display.
	var targetLocation coords.Point
	switch mode {
	case expand, collapse:
		const (
			// offsetX is the X-axis offset of the button from the grouped notification top-right corner.
			offsetX = 28
			// offsetY is the Y-axis offset of the button from the grouped notification top-right corner.
			offsetY = 44
		)
		buttonCenter := bundledBoundsBefore.TopRight().Add(coords.NewPoint(-offsetX, offsetY))

		targetLocation = buttonCenter
	case click:
		// notificationOffsetY is the half height of single notification on the DUT.
		// This offset aims at making the cursor to be able to click the notification content instead of its border.
		notificationOffsetY := singleNotificationHeight / 2
		notificationViewBottomCenter := bundledBoundsBefore.BottomCenter()
		lastNotificationCenter := notificationViewBottomCenter.Add(coords.NewPoint(0, -notificationOffsetY))

		targetLocation = lastNotificationCenter
	default:
		return errors.Errorf("unknown mode: %q", mode)
	}

	if err := res.ui.MouseClickAtLocation(0, targetLocation)(ctx); err != nil {
		return errors.Wrapf(err, "failed to click the specified cords %q when %q", targetLocation, mode)
	}

	bundledBoundsAfter, err = notificationBounds(ctx, res)
	if err != nil {
		return errors.Wrapf(err, "failed to get the notification bounds after %q", mode)
	}

	// We use the difference of the height to determine whether the notification is expanded, collapsed or clicked.
	switch mode {
	case collapse:
		// The height should be smaller than the original one after collapsing the notification
		if !(bundledBoundsAfter.Height < bundledBoundsBefore.Height) {
			return errors.Errorf("the height of grouped notification does not match the regulation while 'collapse': height of grouped notification %v, height of collapsed grouped notification %v", bundledBoundsBefore.Height, bundledBoundsAfter.Height)
		}
	case expand:
		// If the notification is expanded, the expected height will be somewhere between
		// "singleNotificationHeight * numberOfExpectedNotifications" and "singleNotificationHeight * (numberOfExpectedNotifications + 1)".
		if !(bundledBoundsAfter.Height < singleNotificationHeight*(notificationGroupCnt+1) && bundledBoundsAfter.Height > singleNotificationHeight*notificationGroupCnt) {
			return errors.Errorf("the height of grouped notification does not match the regulation while 'expand': height of grouped notification %v, number of expected notifications %v", bundledBoundsAfter.Height, notificationGroupCnt)
		}
	case click:
		// If notificationGroupCnt is 1, clicking the notification will not change the height
		if notificationGroupCnt == 1 {
			testing.ContextLog(ctx, "There are only one notification inside the grouped notification, skip verification for clicking")
			return nil
		}

		heightDiff := math.Abs(float64(bundledBoundsAfter.Height - bundledBoundsBefore.Height))

		// If the notification is clicked, the reduced height should be somewhere between 0.5 to 1.5 height of single notification.
		if !(heightDiff > float64(singleNotificationHeight)*0.5 && heightDiff < float64(singleNotificationHeight)*1.5) {
			return errors.Errorf("the height of grouped notification does not match the regulation while 'click': before %v, after %v, heightDiff %v", bundledBoundsBefore.Height, bundledBoundsAfter.Height, heightDiff)
		}
	default:
		return errors.Errorf("unexpected toggle mode: %q", mode)
	}

	return nil
}
