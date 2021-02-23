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
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type googleMeetConference struct {
	cr       *chrome.Chrome
	roomSize int
}

func (conf *googleMeetConference) Join(ctx context.Context, room string) error {
	const timeout = 15 * time.Second

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

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
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Join now"}, timeout); err != nil {
				return errors.Wrap(err, `failed to wait "Join now" button gone`)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 3, Interval: time.Second}); err != nil {
			return errors.Wrap(err, `failed to join conference`)
		}
		return nil
	}

	switchUserJoin := func(ctx context.Context, tconn *chrome.TestConn) error {
		participant := "videodut0@cienetqa.education"
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
			return errors.Wrapf(err, "failed to switch account to %s", participant)
		}

		testing.ContextLog(ctx, "Select videodut0")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conference.ClickUIByName(ctx, tconn, participant, timeout); err != nil {
				return errors.Wrapf(err, `failed to find account %s`, participant)
			}
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Choose an account"}, timeout); err != nil {
				return errors.Wrapf(err, `failed to wait account %s gone`, participant)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", participant)
		}
		return nil
	}

	checkParticipantsNum := func() error {
		// Confirm that the number of participants is correct
		if err := conference.ClickUIByName(ctx, tconn, "Show everyone", time.Second*30); err != nil {
			return err
		}

		params := ui.FindParams{
			Role:       ui.RoleTypeTab,
			Attributes: map[string]interface{}{"name": regexp.MustCompile("[0-9]+ participants")},
		}

		part, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*30)
		if err != nil {
			return err
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

	if _, err = conf.cr.NewConn(ctx, room); err != nil {
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

func (conf *googleMeetConference) AdmitParticipant(ctx context.Context) error {
	const timeout = time.Second * 15

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

	if err := conference.ClickUIByName(ctx, tconn, "Admit", timeout); err != nil {
		return errors.Wrap(err, `failed to click "Admit" button`)
	}

	return nil
}

func (conf *googleMeetConference) VideoAudioControl(ctx context.Context) error {
	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

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

func (conf *googleMeetConference) SwitchTabs(ctx context.Context) error {
	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

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

func (conf *googleMeetConference) ChangeLayout(ctx context.Context) error {
	const timeout = time.Second * 20
	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close notifications")
	}

	testing.ContextLog(ctx, "Change Layout")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, "More options", time.Second*3); err != nil {
			return err
		}
		if err := conference.ClickUIByName(ctx, tconn, "Change layout", time.Second*3); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to change layout")
	}

	for _, v := range []string{"Tiled", "Spotlight"} {
		if err := conference.ClickUIByName(ctx, tconn, v, timeout); err != nil {
			return err
		}
		if err := testing.Sleep(ctx, time.Second*10); err != nil {
			return errors.Wrap(err, "failed to wait layout change")
		}
	}
	return nil
}

func (conf *googleMeetConference) BackgroundBlurring(ctx context.Context) error {
	const (
		blurBackground    = "Blur your background"
		skyBackground     = "Blurry sky with purple horizon background"
		turnOffBackground = "Turn off background effects"
		timeout           = time.Second * 20
	)

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {

		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()
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
		if err := conference.ClickUIByName(ctx, tconn, "More options", timeout); err != nil {
			return err
		}

		if err := conference.ClickUIByName(ctx, tconn, "Change background", timeout); err != nil {
			return err
		}

		if err := conference.ClickUIByName(ctx, tconn, "Blur your background", timeout); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Select background: ", background)
		if err := conference.ClickUIByName(ctx, tconn, background, timeout); err != nil {
			return err
		}

		closeButtonParams := ui.FindParams{Name: "Close", Role: ui.RoleTypeButton}
		closeButton, err := ui.FindWithTimeout(ctx, tconn, closeButtonParams, timeout)
		if err != nil {
			return errors.Wrap(err, "failed to find close button")
		}
		defer closeButton.Release(ctx)
		if err := closeButton.LeftClick(ctx); err != nil {
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

func (conf *googleMeetConference) PresentSlide(ctx context.Context) error {
	const (
		timeout  = time.Second * 30
		slideURL = "https://docs.google.com/presentation/d/1BuvbMyZ0KE_kgtJ3WODZe0dXz2hs2qrjgM82NxhIQos/edit"
	)

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

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
		if err := testing.Sleep(ctx, time.Second*2); err != nil {
			return errors.Wrap(err, "failed to sleep for wait windows selection appear")
		}

		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait select window popup")
		}

		if err := kb.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to type the tab key")
		}

		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to type the enter key")
		}

		if err := testing.Sleep(ctx, time.Second*2); err != nil {
			return errors.Wrap(err, "failed to sleep for wait current page element rendering")
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

func (conf *googleMeetConference) ExtendedDisplayPresenting(ctx context.Context, tabletMode bool) error {
	const (
		timeout  = time.Second * 30
		slideURL = "https://docs.google.com/presentation/d/1BuvbMyZ0KE_kgtJ3WODZe0dXz2hs2qrjgM82NxhIQos/edit"
	)

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

	// Make Google Meet to show the bottom bar
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	unsetMirrorDisplay := func(ctx context.Context, tConn *chrome.TestConn) error {
		testing.ContextLog(ctx, "Launch os settins to disable mirror")
		settings, err := ossettings.LaunchAtPage(ctx, tConn, ossettings.Device)
		if err != nil {
			return errors.Wrap(err, "failed to launch os-settings page")
		}

		if err := settings.LaunchDisplay()(ctx); err != nil {
			return errors.Wrap(err, "failed to launch display page")
		}

		if err := settings.ClickMirrorDisplay()(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}

		if err := settings.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close settings")
		}

		return nil
	}

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

	openSlide := func(ctx context.Context) error {
		slideConn, err := conf.cr.NewConn(ctx, slideURL, cdputil.WithNewWindow())
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

	moveConferenceTab := func(ctx context.Context) error {
		testing.Sleep(ctx, time.Millisecond*400)
		if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
			return errors.Wrap(err, "failed to switch split tabs")
		}
		testing.Sleep(ctx, time.Millisecond*400)
		if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
			return errors.Wrap(err, "failed to move tab to extended display")
		}
		testing.Sleep(ctx, time.Millisecond*400)
		return nil
	}

	enterPresentingMode := func(ctx context.Context) error {
		if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
			return err
		}
		testing.Sleep(ctx, time.Millisecond*400)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := moveMouseToScreenCenter(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to move mouse to screen center")
			}

			if err := conference.ClickUIByName(ctx, tconn, "Present now", time.Second*3); err != nil {
				return err
			}

			if err := conference.ClickUIByName(ctx, tconn, "A tab", time.Second*3); err != nil {
				return err
			}

			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "A tab"}, time.Second*3); err != nil {
				return err
			}

			return nil
		}, &testing.PollOptions{Timeout: time.Minute * 2, Interval: time.Second}); err != nil {
			return errors.Wrap(err, "failed to present slide")
		}
		return nil
	}

	selectSharedTab := func(ctx context.Context) error {
		// Focus on Tab selection window
		if err := kb.Accel(ctx, "Tab"); err != nil {
			return err
		}
		testing.Sleep(ctx, time.Millisecond*400)
		// Select second tab (slide)
		if err := kb.Accel(ctx, "Down"); err != nil {
			return err
		}
		testing.Sleep(ctx, time.Millisecond*400)
		if err := kb.Accel(ctx, "Down"); err != nil {
			return err
		}
		testing.Sleep(ctx, time.Millisecond*400)
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return err
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{Name: "Stop presenting"}, time.Second); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second * 2}); err != nil {
			return err
		}
		return nil
	}

	startPresent := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			params := ui.FindParams{Name: "Start presentation (Ctrl+F5)", Role: ui.RoleTypeButton}
			presentButton, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*2)
			if err != nil {
				return err
			}
			defer presentButton.Release(ctx)
			if err := presentButton.LeftClick(ctx); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
	}

	switchSlide := func(ctx context.Context) error {
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
		return nil
	}

	if tabletMode {
		if err := unsetMirrorDisplay(ctx, tconn); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Open a slide")
	if err := openSlide(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Enter Presenting Mode")
	if err := enterPresentingMode(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Select Tab for Sharing")
	if err := selectSharedTab(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Move conference tab to extended display")
	if err := moveConferenceTab(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start presenting")
	if err := startPresent(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch slides")
	if err := switchSlide(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := conference.EditSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, `failed to edit slide when leave presentation mode`)
	}
	return nil
}

func (conf *googleMeetConference) StopPresenting(ctx context.Context) error {
	const timeout = time.Second * 20

	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

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
	}, &testing.PollOptions{Timeout: time.Second * 60, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to stop presenting slide")
	}

	return nil
}

func (conf *googleMeetConference) End(ctx context.Context) error {
	tconn, err := conf.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()
	cuj.CloseAllWindows(ctx, tconn)
	return nil
}

var _ conference.Conference = (*googleMeetConference)(nil)

// NewGoogleMeetConference creates Google Meet conference room instance which implements Conference interface.
func NewGoogleMeetConference(cr *chrome.Chrome, roomSize int) *googleMeetConference {
	return &googleMeetConference{cr: cr, roomSize: roomSize}
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
