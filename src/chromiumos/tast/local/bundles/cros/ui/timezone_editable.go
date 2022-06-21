// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TimezoneEditable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that settings about time zone can only be changed by users and not guest",
		Contacts: []string{
			"ting.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

type timezoneUserType string

const (
	owner     timezoneUserType = "Owner"
	nonOwner  timezoneUserType = "NonOwner"
	guestUser timezoneUserType = "Guest"
)

// timezoneTestResources holds resources for timezone test.
type timezoneTestResources struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
}

// TimezoneEditable tests that settings about time zone can only be changed by users and not guest.
func TimezoneEditable(ctx context.Context, s *testing.State) {
	const (
		ownername    = "owner@gmail.com"
		nonOwnername = "nonowner@gmail.com"
		password     = "pass"
	)
	loginOpts := map[timezoneUserType]chrome.Option{
		owner:     chrome.FakeLogin(chrome.Creds{User: ownername, Pass: password}),
		nonOwner:  chrome.FakeLogin(chrome.Creds{User: nonOwnername, Pass: password}),
		guestUser: chrome.GuestLogin(),
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Expecting name with pattern like "Time zone (UTC+3:00) Arabian Standard Time (Baghdad)".
	timeZoneReg := regexp.MustCompile(`(Time zone )?\(UTC([+-])(\d+):(\d+)\).*Time.*`)

	for i, user := range []timezoneUserType{
		owner,
		nonOwner,
		// guestUser, // TODO(b/220639439): Add test for guest user once the issue is fixed.
	} {
		var is24HourBefore, is24HourAfter bool
		var timezoneBefore, timezoneAfter string

		s.Log("Start testing on ", user)
		func() {
			res, err := loginAndPrepare(ctx, loginOpts[user], i != 0, s.OutDir())
			if err != nil {
				s.Fatal("Failed to login and prepare for test: ", err)
			}
			defer res.cr.Close(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, res.cr, "verify_timezone_ui_dump")

			is24HourBefore, timezoneBefore, err = fetchTimeSettings(ctx, res, timeZoneReg)
			if err != nil {
				s.Fatal("Failed to save the value of timezone setting before start: ", err)
			}

			if err := verifyTimeSettings(ctx, res, timeZoneReg, is24HourBefore, timezoneBefore); err != nil {
				s.Fatal("Failed to verify the current settings: ", err)
			}

			is24HourAfter, timezoneAfter, err = changeTimeSettings(ctx, res, timezoneBefore)
			if err != nil {
				s.Fatal("Failed to change the current time zone value: ", err)
			}
		}()

		if err := signOutAndWait(ctx); err != nil {
			s.Fatal("Failed to wait for signed out: ", err)
		}

		func() {
			res, err := loginAndPrepare(ctx, loginOpts[user], true, s.OutDir())
			if err != nil {
				s.Fatal("Failed to login and prepare for test: ", err)
			}
			defer res.cr.Close(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, res.cr, "verify_timezone_ui_dump")

			is24HourNow, timezoneNow, err := fetchTimeSettings(ctx, res, timeZoneReg)
			if err != nil {
				s.Fatal("Failed to save the value of timezone setting: ", err)
			}

			if err := verifyTimeSettings(ctx, res, timeZoneReg, is24HourNow, timezoneNow); err != nil {
				s.Fatal("Failed to verify the time settings after change: ", err)
			}

			switch user {
			case owner, nonOwner:
				if timezoneNow != timezoneAfter {
					s.Fatalf("Failed to preserve the change of time zone, expected %q but get %q ", timezoneAfter, timezoneNow)
				}
				if is24HourNow != is24HourAfter {
					s.Fatalf("Failed to preserve the change of 24-hour clock mode, expected %t but get %t ", is24HourAfter, is24HourNow)
				}
			case guestUser:
				if (timezoneNow == timezoneAfter) || (timezoneNow != timezoneBefore) {
					s.Fatalf("Timezone shouldn't be preserved for Guest user, now: %q, before: %q, after: %q ", timezoneNow, timezoneBefore, timezoneAfter)
				}
				if (is24HourNow == is24HourAfter) || (is24HourNow != is24HourBefore) {
					s.Fatalf("Hour clock mode shouldn't be preserved for Guest user, now: %t, before: %t, after: %t ", is24HourNow, is24HourBefore, is24HourAfter)
				}
			}
		}()
	}
}

func loginAndPrepare(ctx context.Context, loginOpt chrome.Option, keepState bool, outDir string) (res *timezoneTestResources, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	opts := []chrome.Option{loginOpt}
	if keepState {
		opts = append(opts, chrome.KeepState())
	}

	var err error
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign in")
	}
	defer func() {
		if retErr != nil {
			cr.Close(cleanupCtx)
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Test API connection")
	}

	return &timezoneTestResources{cr: cr, tconn: tconn, ui: uiauto.New(tconn)}, nil
}

// changeTimeSettings changes the timezone and 24-hour clock setting.
func changeTimeSettings(ctx context.Context, res *timezoneTestResources, timezoneBefore string) (is24Hour bool, timezoneName string, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return is24Hour, "", errors.New("failed to get the OutDir")
	}

	timezoneSelect := nodewith.Name("Time zone").HasClass("md-select")
	settings, err := ossettings.LaunchAtPageURL(ctx, res.tconn, res.cr, "dateTime/timeZone", res.ui.WaitUntilExists(timezoneSelect))
	if err != nil {
		return is24Hour, "", errors.Wrap(err, "failed to launch OSSettings at date time page")
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, outdir, func() bool { return retErr != nil }, res.cr, "set_timezone_ui_dump")

	if err := uiauto.Combine("open drop down list of timezone",
		settings.LeftClick(timezoneSelect),
		settings.WaitForLocation(nodewith.Role(role.ListBox).Ancestor(timezoneSelect)),
	)(ctx); err != nil {
		return is24Hour, "", err
	}

	var targetTimezone *nodewith.Finder
	onscreenOptions := nodewith.Role(role.ListBoxOption).Onscreen()
	options, err := settings.NodesInfo(ctx, onscreenOptions)
	if err != nil {
		return is24Hour, "", errors.Wrap(err, "failed to get info of timezone options")
	}
	if len(options) == 0 {
		return is24Hour, "", errors.New("got 0 timezone options")
	}

	for i, node := range options {
		if !strings.Contains(timezoneBefore, node.Name) {
			targetTimezone = onscreenOptions.Nth(i)
			timezoneName = "Time zone " + node.Name
			testing.ContextLogf(ctx, "Time zone will be changed to %q", node.Name)
			break
		}
	}

	if err := uiauto.Combine("change the time zone and 24-hour clock setting",
		settings.LeftClick(targetTimezone),
		settings.LeftClick(nodewith.Name("Time zone subpage back button")),
		settings.LeftClick(nodewith.Name("Use 24-hour clock")),
	)(ctx); err != nil {
		return is24Hour, "", err
	}

	isEnabled, err := settings.IsToggleOptionEnabled(ctx, res.cr, "Use 24-hour clock")
	if err != nil {
		return is24Hour, "", err
	}

	return isEnabled, timezoneName, nil
}

func signOutAndWait(ctx context.Context) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to session manager")
	}

	const state = "stopped"
	sw, err := sm.WatchSessionStateChanged(ctx, state)
	if err != nil {
		return errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(cleanupCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()

	if err := uiauto.Combine("sign out by shortcut",
		kb.AccelAction("Shift+Ctrl+Q"),
		kb.AccelAction("Shift+Ctrl+Q"),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}

	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "Got SessionStateChanged signal")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get SessionStateChanged signal")
	}

	return nil
}

// fetchTimeSettings fetches the current time settings.
func fetchTimeSettings(ctx context.Context, res *timezoneTestResources, timeZoneReg *regexp.Regexp) (is24Hour bool, timezoneName string, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return is24Hour, "", errors.New("failed to get the OutDir")
	}

	chooseFromList := nodewith.Role(role.RadioButton).Name("Choose from list")
	settings, err := ossettings.LaunchAtPageURL(ctx, res.tconn, res.cr, "dateTime/timeZone", res.ui.WaitUntilExists(chooseFromList))
	if err != nil {
		return is24Hour, "", errors.Wrap(err, "failed to launch OSSettings at time zone page")
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, outdir, func() bool { return retErr != nil }, res.cr, "save_timezone_ui_dump")

	// Click 'Choose from list' to show the current timezone.
	if err := uiauto.Combine("make timezone show",
		settings.LeftClick(chooseFromList),
		settings.LeftClick(nodewith.Name("Time zone subpage back button")),
	)(ctx); err != nil {
		return is24Hour, "", err
	}

	timezone, err := settings.Info(ctx, nodewith.NameRegex(timeZoneReg))
	if err != nil {
		return is24Hour, "", errors.Wrap(err, "failed to get information of timezone")
	}

	isEnabled, err := settings.IsToggleOptionEnabled(ctx, res.cr, "Use 24-hour clock")
	if err != nil {
		return is24Hour, timezone.Name, err
	}

	return isEnabled, timezone.Name, nil
}

// verifyTimeSettings verifies the current time settings.
func verifyTimeSettings(ctx context.Context, res *timezoneTestResources, timeZoneReg *regexp.Regexp, is24Hour bool, timezone string) error {
	// TODO(b/220639439):
	// The 24 hour clock display of guest is initialized as the owner,
	// but the switch button in settings is always initialized as False.
	// Verify this part for guest user after the issue is fixed.
	if err := verifyHourClock(ctx, res.ui, is24Hour); err != nil {
		return errors.Wrap(err, "failed to verify hour clock")
	}

	if err := verifyTimeZone(ctx, res.ui, timeZoneReg, is24Hour, timezone); err != nil {
		return errors.Wrap(err, "failed to verify timezone")
	}

	return nil
}

// verifyHourClock verifies the hour-clock.
func verifyHourClock(ctx context.Context, ui *uiauto.Context, is24Hour bool) error {
	date := `Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday`
	Month := `January|February|March|April|May|June|July|August|September|October|November|December`

	// Expecting name with pattern like "10:59 AM, Friday, January 21, 2022".
	reg := regexp.MustCompile(fmt.Sprintf(`(\d+):(\d+)( AM| PM), (%s), (%s) (\d+), (\d+)`, date, Month))
	if is24Hour {
		// Expecting name with pattern like "10:59, Friday, January 21, 2022".
		reg = regexp.MustCompile(fmt.Sprintf(`(\d+):(\d+), (%s), (%s) (\d+), (\d+)`, date, Month))
	}

	if err := ui.WaitUntilExists(nodewith.NameRegex(reg).Ancestor(quicksettings.SystemTray))(ctx); err != nil {
		if is24Hour {
			return errors.Wrap(err, `AM/PM should disappear when 24-hour clock mode is enable`)
		}
		return errors.Wrap(err, `AM/PM should exist when 24-hour clock mode is disable`)
	}
	return nil
}

// verifyTimeZone verifies the timezone string matches with the time shown in time view.
func verifyTimeZone(ctx context.Context, ui *uiauto.Context, timeZoneReg *regexp.Regexp, is24Hour bool, timezoneName string) error {
	loc, err := parseTimezone(ctx, timezoneName, timeZoneReg)
	if err != nil {
		return errors.Wrapf(err, "failed to parse timezone: %q", timezoneName)
	}
	expectTime := time.Now().In(loc)
	testing.ContextLog(ctx, "Expect time: ", expectTime)

	timeFormat := map[bool]string{
		true:  "15:04, Monday, January _2, 2006",
		false: "15:04 PM, Monday, January _2, 2006",
	}

	timeviewInfo, err := ui.Info(ctx, nodewith.HasClass("TimeView").Ancestor(quicksettings.SystemTray))
	if err != nil {
		return errors.Wrap(err, "failed to get information of TimeView")
	}
	testing.ContextLog(ctx, "Time view: ", timeviewInfo.Name)

	currentTime, err := time.ParseInLocation(timeFormat[is24Hour], timeviewInfo.Name, loc)
	if err != nil {
		return errors.Wrap(err, "failed to parse timeview")
	}

	deviation := currentTime.Sub(expectTime)
	if deviation > 2*time.Minute || deviation < -2*time.Minute {
		return errors.Wrapf(err, "the clock didn't updated dynamically, expect: %s, got: %s", expectTime.String(), currentTime.String())
	}

	return nil
}

// parseTimezone parses the timezone string to *time.Location.
func parseTimezone(ctx context.Context, timezoneName string, timezoneReg *regexp.Regexp) (loc *time.Location, retErr error) {
	// Expecting name with pattern like "Time zone (UTC+3:00) Arabian Standard Time (Baghdad)".
	ss := timezoneReg.FindStringSubmatch(timezoneName)
	if ss == nil || len(ss) != 5 {
		return loc, errors.Errorf("unexpected timezone name: %q", timezoneName)
	}

	sign := 1
	if ss[2] == "-" {
		sign = -1
	}
	hour, err := strconv.Atoi(ss[3])
	if err != nil {
		return loc, errors.Wrap(err, "failed to parse timezone")
	}
	minute, err := strconv.Atoi(ss[4])
	if err != nil {
		return loc, errors.Wrap(err, "failed to calculate timezone")
	}

	loc = time.FixedZone(timezoneName, sign*(hour*3600+minute*60))
	return
}
