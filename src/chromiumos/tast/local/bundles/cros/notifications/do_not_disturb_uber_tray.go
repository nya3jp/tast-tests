// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DoNotDisturbUberTray,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the display of uber tray when switching on and off do not disturb",
		Contacts: []string{
			"bob.yang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
			Val:               browser.TypeLacros,
		}},
		Timeout: 3*time.Minute + notificationNumberTimeout,
	})
}

const (
	// defaultTimeout is timeout for waiting incidents to be triggers in the case.
	defaultTimeout = 30 * time.Second
	// notificationNumberTimeout is timeout for waiting notificationNumber to change in UI tree.
	notificationNumberTimeout = 2 * time.Minute
)

type doNotDisturbStatus bool

const (
	turnOn  doNotDisturbStatus = true
	turnOff doNotDisturbStatus = false
)

type disturbTestResources struct {
	bTconn, tconn *chrome.TestConn
	ui            *uiauto.Context
}

// DoNotDisturbUberTray verifies the display of uber tray when switching on and off do not disturb.
func DoNotDisturbUberTray(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get brwoser test API connection: ", err)
	}

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to clear notifications: ", err)
	}

	res := &disturbTestResources{
		bTconn: bTconn,
		tconn:  tconn,
		ui:     uiauto.New(tconn),
	}

	for i, test := range []struct {
		notification notificationTest
		doNotDisturb doNotDisturbStatus
	}{
		{
			notification: newScreenshot(res),
			doNotDisturb: turnOff,
		}, {
			notification: newBrowserNotification("notification test title 1", res),
			doNotDisturb: turnOff,
		}, {
			notification: newBrowserNotification("notification test title 2", res),
			doNotDisturb: turnOn,
		},
	} {
		if err := doNotDisturb(ctx, res, test.doNotDisturb); err != nil {
			s.Fatal("Failed to toggle do not disturb: ", err)
		}

		s.Logf("Creating notificaton %d", i+1)
		if err := test.notification.create(ctx); err != nil {
			s.Fatalf("Failed to create notification %q: %v", test.notification.getTitle(), err)
		}
		defer test.notification.cleanup(cleanupCtx)
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("ui_dump_%s", test.notification.getTitle()))

		s.Logf("Verifying notificaton %d", i+1)
		if err := test.notification.verify(ctx); err != nil {
			s.Fatalf("Failed to verify test %q: %v", test.notification.getTitle(), err)
		}

		if test.doNotDisturb == turnOn {
			if err := verifyNumberOfNotifications(ctx, res, 0); err != nil {
				s.Fatal("Failed to verify the number of notifications: ", err)
			}

			// Ensure notification won't be offscreen by collapse the quicksettings in advance.
			if err := quicksettings.Collapse(ctx, res.tconn); err != nil {
				s.Fatal("Failed to show quicksettings in collapse mode: ", err)
			}
			defer quicksettings.Hide(ctx, res.tconn)

			// Ensure notification exists in the message section of quicksettings.
			notificationCenter := nodewith.Name("Notification Center").HasClass("Widget").Role(role.Dialog)
			if err := res.ui.WaitUntilExists(nodewith.NameContaining(test.notification.getTitle()).HasClass("MessageView").Ancestor(notificationCenter))(ctx); err != nil {
				s.Fatal("Failed to find notification: ", err)
			}

			// DoNotDisturb icon should persist when receiving notification.
			doNotDisturbIcon := nodewith.Name("Do Not Disturb is on").HasClass("ImageView").Ancestor(quicksettings.StatusAreaWidget)
			if found, err := uiauto.New(tconn).IsNodeFound(ctx, doNotDisturbIcon); err != nil {
				s.Fatal("Failed to verify do not disturb icon: ", err)
			} else if !found {
				s.Fatal("Failed to verify the functionality of do not disturb: the icon didn't persist when receiving notification")
			}

			// Turn the DoNotDisturb back to off to verify the number of notifications.
			if err := doNotDisturb(ctx, res, turnOff); err != nil {
				s.Fatal("Failed to turn off do not disturb: ", err)
			}
		}

		if err := verifyNumberOfNotifications(ctx, res, i+1); err != nil {
			s.Fatal("Failed to verify the number of notifications: ", err)
		}
	}
}

type notificationTest interface {
	getTitle() string

	create(ctx context.Context) error
	verify(ctx context.Context) error
	cleanup(ctx context.Context)
}

type screenshotTest struct {
	title string
	res   *disturbTestResources
}

func newScreenshot(res *disturbTestResources) *screenshotTest {
	return &screenshotTest{
		title: "Screenshot taken",
		res:   res,
	}
}

func (s *screenshotTest) getTitle() string {
	return s.title
}

func (s *screenshotTest) create(ctx context.Context) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	defer kb.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}

	if err := kb.Accel(ctx, "Ctrl+"+topRow.SelectTask); err != nil {
		return errors.Wrap(err, "failed to take screenshot by keyboard shortcut")
	}

	return nil
}

func (s *screenshotTest) verify(ctx context.Context) error {
	if _, err := ash.WaitForNotification(ctx, s.res.tconn, defaultTimeout, ash.WaitTitle(s.title)); err != nil {
		return errors.Wrapf(err, "failed to wait for notifciation with title %q", s.title)
	}
	return nil
}

func (s *screenshotTest) cleanup(ctx context.Context) {
	// Only log the error when cleanup.
	if err := screenshot.RemoveScreenshots(); err != nil {
		testing.ContextLog(ctx, "Failed to clean up screenshots: ", err)
	}

	// There's no valid id to call browser.ClearNotification for notification of screenshot.
	if err := ash.CloseNotifications(ctx, s.res.tconn); err != nil {
		testing.ContextLog(ctx, "Failed to close notifications: ", err)
	}
}

type browserNotificationTest struct {
	title          string
	res            *disturbTestResources
	notificationID string
}

func newBrowserNotification(title string, res *disturbTestResources) *browserNotificationTest {
	return &browserNotificationTest{
		title: title,
		res:   res,
	}
}

func (b *browserNotificationTest) getTitle() string {
	return b.title
}

func (b *browserNotificationTest) create(ctx context.Context) error {
	id, err := browser.CreateTestNotification(ctx, b.res.bTconn, browser.NotificationTypeBasic, b.title, b.title)
	if err != nil {
		return errors.Wrap(err, "failed to create test notification")
	}
	b.notificationID = id
	return nil
}

func (b *browserNotificationTest) verify(ctx context.Context) error {
	if _, err := ash.WaitForNotification(ctx, b.res.tconn, defaultTimeout, ash.WaitTitle(b.title)); err != nil {
		return errors.Wrapf(err, "failed to wait for notifciation with title %q", b.title)
	}
	return nil
}

func (b *browserNotificationTest) cleanup(ctx context.Context) {
	// Only log the error when cleanup.
	if err := browser.ClearNotification(ctx, b.res.bTconn, b.notificationID); err != nil {
		testing.ContextLog(ctx, "Failed to close notifications: ", err)
	}
}

// verifyNumberOfNotifications verifies if the number of notifications is expected.
func verifyNumberOfNotifications(ctx context.Context, res *disturbTestResources, num int) error {
	widgetBtn := nodewith.Role(role.Button).Ancestor(quicksettings.StatusAreaWidget)
	numberOfNotificationIcon := widgetBtn.NameContaining("notification")
	if num == 0 {
		// Ensure the number of notification icon does not exist if expected number = 0.
		if err := res.ui.WaitUntilGone(numberOfNotificationIcon)(ctx); err != nil {
			return errors.Wrap(err, "failed to verify number of notifications: notifications exist")
		}

		// EnsureGoneFor immediately checks if node is gone, needs to be called after WaitUntilGone.
		return res.ui.EnsureGoneFor(numberOfNotificationIcon, 5*time.Second)(ctx)
	} else if num > 0 {
		// It takes up to 2 mins to wait until the number of notifications to update in the UI tree.
		// Node name of notification number can be "x notifications" or "x other notifications"
		return testing.Poll(ctx, func(ctx context.Context) error {
			info, err := res.ui.Info(ctx, numberOfNotificationIcon)
			if err != nil {
				testing.PollBreak(errors.Wrap(err, "failed to get number of notification info"))
			}

			re := regexp.MustCompile("(\\d+)(\\sother)?\\snotification")
			numInfo := re.FindStringSubmatch(info.Name)[1]
			notificationNum, err := strconv.Atoi(numInfo)
			if err != nil {
				testing.PollBreak(errors.Wrap(err, "failed to convert string to int"))
			}

			if num != notificationNum {
				return errors.Errorf("failed to verify number of notifications, want: %d, got: %d", num, notificationNum)
			}

			return nil
		}, &testing.PollOptions{Timeout: notificationNumberTimeout})
	} else {
		return errors.New("wrong input number, expected number >= 0")
	}
}

// doNotDisturb switches do not disturb status and verifies the icon corresponding to the status appears.
func doNotDisturb(ctx context.Context, res *disturbTestResources, status doNotDisturbStatus) error {
	if err := quicksettings.ToggleSetting(ctx, res.tconn, quicksettings.SettingPodDoNotDisturb, bool(status)); err != nil {
		return err
	}

	doNotDisturbIcon := nodewith.Name("Do Not Disturb is on").HasClass("ImageView").Ancestor(quicksettings.StatusAreaWidget)
	if status == turnOn {
		return res.ui.WaitUntilExists(doNotDisturbIcon)(ctx)
	}

	if err := res.ui.WaitUntilGone(doNotDisturbIcon)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for do not disturb icon to disappear")
	}

	// EnsureGoneFor immediately checks if node is gone, needs to be called after WaitUntilGone.
	return res.ui.EnsureGoneFor(doNotDisturbIcon, 5*time.Second)(ctx)
}
