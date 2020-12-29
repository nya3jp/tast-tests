// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googlemeet

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type googleMeetConference struct {
	cr       *chrome.Chrome
	roomSize int
	account  string
	password string
}

func (conf *googleMeetConference) Join(ctx context.Context, tconn *chrome.TestConn, room string) error {
	const timeout = 15 * time.Second

	// The Chrome browser would ask for camera and microphone permissions
	// before joining Google Meet conference. allowPerm allows both of camera
	// and audio permissions.
	allowPerm := func(ctx context.Context) error {
		// Meet will ask the permission: wait for the permission bubble to appear.
		// Note that there may be some other bubbles, so find only within the main
		// container -- which should be named as "Desk_Container_A", the primary
		// desk.
		container, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "Desk_Container_A"})
		if err != nil {
			return errors.Wrap(err, "failed to find the container")
		}
		defer container.Release(ctx)
		for i := 0; i < 5; i++ {
			bubble, err := container.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "BubbleDialogDelegateView"}, timeout)
			if err != nil {
				// It is fine not finding the bubble.
				return nil
			}
			defer bubble.Release(ctx)
			allowButton, err := bubble.Descendant(ctx, ui.FindParams{Name: "Allow", Role: ui.RoleTypeButton})
			if err != nil {
				return errors.Wrap(err, "failed to find the allow button")
			}
			defer allowButton.Release(ctx)
			if err := allowButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click the allow button")
			}
			if err := testing.Sleep(ctx, time.Second*10); err != nil {
				return errors.Wrap(err, "failed to wait for the next cycle of permission")
			}
		}
		return errors.New("too many permission requests")
	}

	joinConf := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Join Conference")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "Join now", timeout); err != nil {
				return errors.Wrap(err, `failed to click "Join now" button`)
			}
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Return to home screen"}, timeout); err != nil {
				return errors.Wrap(err, `failed to wait "Meet logo" gone`)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 3, Interval: time.Second}); err != nil {
			return errors.Wrap(err, `failed to join conference`)
		}
		return nil
	}

	meetAccount := conf.account

	// Using existed conference-test account for Google Meet testing,
	// and add the test account if it doesn't add in the DUT before.
	addMeetAccount := func(ctx context.Context, tconn *chrome.TestConn) error {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to initialize keyboard input")
		}
		defer kb.Close()

		if err := conference.ClickUIByName(ctx, tconn, "Use another account", timeout); err != nil {
			return errors.Wrap(err, `failed to click "Use another account" link`)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Shown in the first time
			if err := conference.ClickUIByName(ctx, tconn, "View accounts", time.Second*3); err != nil {
				testing.ContextLog(ctx, "View accounts is not shown")
			}
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "My accounts"}, time.Second*3); err != nil {
				return errors.Wrap(err, `failed to wait "My accounts"`)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second * 20, Interval: time.Second}); err != nil {
			return err
		}

		if err := conference.ClickUIByName(ctx, tconn, "Add account", timeout); err != nil {
			return errors.Wrap(err, `failed to click "Add account" Button`)
		}
		if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "Email or phone"}, timeout); err != nil {
			return errors.Wrap(err, `failed to wait "Email or phone"`)
		}
		if err := kb.Type(ctx, meetAccount); err != nil {
			return errors.Wrapf(err, "failed to type %q", meetAccount)
		}
		if err := conference.ClickUIByName(ctx, tconn, "Next", timeout); err != nil {
			return errors.Wrap(err, `failed to click "Next" Button`)
		}
		if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: meetAccount}, timeout); err != nil {
			return errors.Wrapf(err, "failed to wait %q", meetAccount)
		}
		if err := conference.ClickUIByName(ctx, tconn, "Enter your password", timeout); err != nil {
			return errors.Wrap(err, `failed to click "Enter your password"`)
		}
		if err := kb.Type(ctx, conf.password); err != nil {
			return errors.Wrapf(err, "failed to type %q", meetAccount)
		}
		if err := conference.ClickUIByName(ctx, tconn, "Next", timeout); err != nil {
			return errors.Wrap(err, `failed to click "Next" Button`)
		}
		if err := conference.ClickUIByName(ctx, tconn, "I agree", timeout); err != nil {
			return errors.Wrap(err, `failed to click "I agree" Button`)
		}
		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			return errors.Wrap(err, `failed to close settings page`)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "Switch account", timeout); err != nil {
				return errors.Wrap(err, `failed to click "Switch account" link`)
			}
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Switch account"}, timeout); err != nil {
				return errors.Wrap(err, `failed to wait "Switch account" gone`)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", meetAccount)
		}
		return nil
	}

	// If gaia-login isn't use the conference-test only account, it would switch
	// when running the case. And also add the test account if the DUT doesn't
	// be added before.
	switchUserJoin := func(ctx context.Context, tconn *chrome.TestConn) error {
		testing.ContextLog(ctx, "Switch account")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "Switch account", timeout); err != nil {
				return errors.Wrap(err, `failed to click "Switch account" link`)
			}
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Switch account"}, timeout); err != nil {
				return errors.Wrap(err, `failed to wait "Switch account" gone`)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", meetAccount)
		}

		if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: meetAccount}, time.Second*10); err != nil {
			if err := addMeetAccount(ctx, tconn); err != nil {
				return errors.Wrapf(err, "failed to add account %s", meetAccount)
			}
		}
		testing.ContextLogf(ctx, "Select meet account %s", meetAccount)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, meetAccount, time.Second*5); err != nil {
				return errors.Wrapf(err, `failed to find account %s`, meetAccount)
			}
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Choose an account"}, time.Second*5); err != nil {
				return errors.Wrapf(err, `failed to wait account %s gone`, meetAccount)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", meetAccount)
		}
		return nil
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size conference.
	checkParticipantsNum := func() error {
		if err := conference.ClickUIByName(ctx, tconn, "Show everyone", time.Minute*2); err != nil {
			return err
		}

		params := ui.FindParams{
			Role:       ui.RoleTypeTab,
			Attributes: map[string]interface{}{"name": regexp.MustCompile("[0-9]+ participant[s]?")},
		}

		part, err := ui.FindWithTimeout(ctx, tconn, params, time.Minute*2)
		if err != nil {
			return errors.Wrap(err, "failed to find number of participants")
		}

		strs := strings.Split(part.Name, " ")
		num, err := strconv.ParseInt(strs[0], 10, 64)
		if err != nil {
			return errors.Wrap(err, "cannot parse number of participants")
		}
		if int(num) < conf.roomSize {
			return errors.Wrap(err, `the number of participants is not as expected`)
		}
		if int(num) > conf.roomSize {
			testing.ContextLogf(ctx, "There are %d participants, more than expectation", num)
		}
		return nil
	}

	if _, err := conf.cr.NewConn(ctx, room); err != nil {
		return errors.Wrap(err, "failed to create participant join conference")
	}

	if err := allowPerm(ctx); err != nil {
		return err
	}

	if err := switchUserJoin(ctx, tconn); err != nil {
		return err
	}

	if err := joinConf(ctx); err != nil {
		return err
	}

	if err := checkParticipantsNum(); err != nil {
		return errors.Wrap(err, "failed to check participants number")
	}

	return nil
}

func (conf *googleMeetConference) VideoAudioControl(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Turn off camera")
	if err := toggleVideo(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}

	if err := testing.Sleep(ctx, time.Second*5); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Turn on camera")
	if err := toggleVideo(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}

	if err := testing.Sleep(ctx, time.Second*5); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}

	if err := testing.Sleep(ctx, time.Second*5); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}

	if err := testing.Sleep(ctx, time.Second*5); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	return nil
}

func (conf *googleMeetConference) SwitchTabs(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Open wiki page")
	wikiURL := "https://www.wikipedia.org/"
	wikiConn, err := conf.cr.NewConn(ctx, wikiURL)
	if err != nil {
		return errors.Wrap(err, "failed to open the wiki url")
	}
	defer wikiConn.Close()

	// switch tab
	if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}

	return nil
}

func (conf *googleMeetConference) ChangeLayout(ctx context.Context, tconn *chrome.TestConn) error {
	const timeout = time.Second * 20

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close notifications")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, "More options", time.Second*3); err != nil {
			return err
		}
		if err := conference.ClickUIByName(ctx, tconn, "Change layout", time.Second*3); err != nil {
			return err
		}
		for _, mode := range []string{"Tiled", "Spotlight"} {
			testing.ContextLog(ctx, "Change Layout to ", mode)
			if err := conference.ClickUIByName(ctx, tconn, mode, timeout); err != nil {
				return err
			}
			if err := testing.Sleep(ctx, time.Second*10); err != nil {
				return errors.Wrap(err, "failed to wait layout change")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to change layout")
	}

	return nil
}

func (conf *googleMeetConference) BackgroundBlurring(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		blurBackground    = "Blur your background"
		skyBackground     = "Blurry sky with purple horizon background"
		turnOffBackground = "Turn off background effects"
		timeout           = time.Second * 20
	)

	enterFullScreen := func() error {
		if err := conference.ClickUIByName(ctx, tconn, "More options", timeout); err != nil {
			return err
		}

		if err := conference.ClickUIByName(ctx, tconn, "Full screen", timeout); err != nil {
			return err
		}
		return nil
	}
	changeBackground := func(background string) error {
		testing.ContextLog(ctx, "Open More Options")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "More options", timeout); err != nil {
				return err
			}
			if err := conference.ClickUIByName(ctx, tconn, "Change background", timeout); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Select background: ", background)
			if err := conference.ClickUIByName(ctx, tconn, background, timeout); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return errors.Wrap(err, "failed to change background")
		}

		closeButtonParams := ui.FindParams{Name: "Backgrounds", Role: ui.RoleTypeButton}
		if err := cuj.WaitAndClick(ctx, tconn, closeButtonParams, timeout); err != nil {
			return errors.Wrap(err, "failed to click close button")
		}
		if err := testing.Sleep(ctx, time.Second*5); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		return nil
	}

	if err := enterFullScreen(); err != nil {
		return errors.Wrap(err, "failed to enter fullscreen")
	}

	if err := changeBackground(blurBackground); err != nil {
		return errors.Wrap(err, "failed to change background to blur background")
	}

	if err := conference.ClickUIByName(ctx, tconn, "Pin yourself to your main screen.", timeout); err != nil {
		return errors.Wrap(err, "failed to switch to your main screen")
	}

	if err := changeBackground(skyBackground); err != nil {
		return errors.Wrap(err, "failed to change background to sky background")
	}

	if err := changeBackground(turnOffBackground); err != nil {
		return errors.Wrap(err, "failed to turn off background effects")
	}

	return nil
}

func (conf *googleMeetConference) PresentSlide(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		timeout  = time.Second * 30
		slideURL = "https://docs.google.com/presentation/d/1BuvbMyZ0KE_kgtJ3WODZe0dXz2hs2qrjgM82NxhIQos/edit"
	)

	// Make Google Meet to show the bottom bar
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	moveMouseToScreenCenter := func(ctx context.Context, tconn *chrome.TestConn) error {
		const xRatio, yRatio = .5, .5
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			return err
		}

		dw := info.WorkArea.Width
		dh := info.WorkArea.Height
		mw, mh := int(float64(dw)*xRatio), int(float64(dh)*yRatio)

		if err := mouse.Move(ctx, tconn, coords.Point{X: mw, Y: mh}, time.Millisecond*50); err != nil {
			return err
		}
		return nil
	}

	presentSlide := func(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := moveMouseToScreenCenter(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to move mouse to screen center")
			}

			if err := conference.ClickUIByName(ctx, tconn, "Present now", time.Second*3); err != nil {
				return err
			}

			if err := conference.ClickUIByName(ctx, tconn, "A window", time.Second*3); err != nil {
				return err
			}

			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 2, Interval: time.Second}); err != nil {
			return errors.Wrap(err, "failed to present slide")
		}
		return nil
	}

	selectPresentWindow := func(ctx context.Context) error {
		presentWindowParams := ui.FindParams{ClassName: "DesktopMediaSourceView", Role: ui.RoleTypeButton}
		if err := cuj.WaitAndClick(ctx, tconn, presentWindowParams, timeout); err != nil {
			return errors.Wrap(err, "failed to click present window")
		}
		shareParams := ui.FindParams{Name: "Share", Role: ui.RoleTypeButton}
		if err := cuj.WaitAndClick(ctx, tconn, shareParams, timeout); err != nil {
			return errors.Wrap(err, "failed to click share button")
		}
		return nil
	}

	waitForPresentMode := func(ctx context.Context, tconn *chrome.TestConn) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "Stop presenting", time.Second*3); err != nil {
				return err
			}

			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 20, Interval: time.Second})
	}

	openSlide := func(ctx context.Context) error {
		slideConn, err := conf.cr.NewConn(ctx, slideURL)
		if err != nil {
			return errors.Wrap(err, "failed to open the slide url")
		}
		defer slideConn.Close()

		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "OK", time.Second*3); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 20, Interval: time.Second})
	}

	startPresent := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, "Present", time.Second*3); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
	}

	waitForLeavePresentMode := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.WaitUIByName(ctx, tconn, "Present", time.Second); err != nil {
				return errors.New("present button does not disappear")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 30, Interval: time.Second})
	}

	testing.ContextLog(ctx, "Click present now button")
	if err := presentSlide(ctx, tconn, kb); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Select window to present")
	if err := selectPresentWindow(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Wait presenting")
	if err := waitForPresentMode(ctx, tconn); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Open a slide")
	if err := openSlide(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start present")
	if err := startPresent(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch slides")
	for i := 0; i < 6; i++ {
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, `failed to type enter key to switch slide`)
		}
		if err := testing.Sleep(ctx, time.Second*1); err != nil {
			return errors.Wrap(err, "failed to sleep for wait slide switching")
		}
	}

	testing.ContextLog(ctx, "Leave presentation mode")
	if err := kb.Accel(ctx, "Esc"); err != nil {
		return errors.Wrap(err, `failed to type esc to leave presentation mode`)
	}

	if err := waitForLeavePresentMode(ctx); err != nil {
		return errors.Wrap(err, `failed to leave presenting mode`)
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := conference.EditSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, `failed to edit slide when leave presentation mode`)
	}

	if err := kb.Accel(ctx, "Ctrl+1"); err != nil {
		return errors.Wrap(err, `failed to send keyboard event`)
	}

	return nil
}

func (conf *googleMeetConference) StopPresenting(ctx context.Context, tconn *chrome.TestConn) error {
	const timeout = time.Second * 20

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Stop presenting")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, "Stop presenting", time.Second*3); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to stop presenting slide")
	}

	return nil
}

func (conf *googleMeetConference) End(ctx context.Context, tconn *chrome.TestConn) error {
	cuj.CloseAllWindows(ctx, tconn)
	return nil
}

var _ conference.Conference = (*googleMeetConference)(nil)

// NewGoogleMeetConference creates Google Meet conference room instance which implements Conference interface.
func NewGoogleMeetConference(cr *chrome.Chrome, roomSize int, account, password string) *googleMeetConference {
	return &googleMeetConference{cr, roomSize, account, password}
}

func toggleVideo(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		stopVideo  = "Turn off camera (ctrl + e)"
		startVideo = "Turn on camera (ctrl + e)"
	)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, stopVideo, time.Second*3); err != nil {
			if err := conference.ClickUIByName(ctx, tconn, startVideo, time.Second*3); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Open Video")
			return nil
		}
		testing.ContextLog(ctx, "Close Video")
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 20, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}

func toggleAudio(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		muteAudio   = "Turn off microphone (ctrl + d)"
		unmuteAudio = "Turn on microphone (ctrl + d)"
	)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, muteAudio, time.Second*3); err != nil {
			if err := conference.ClickUIByName(ctx, tconn, unmuteAudio, time.Second*3); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Open Audio")
			return nil
		}
		testing.ContextLog(ctx, "Mute Audio")
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 20, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}
