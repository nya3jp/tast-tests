// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock13ChangeResolution,
		Desc:         "Change Resolution being displayed on external monitor",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(), // 1) Boot-up and Sign-In to the device
		Vars:         []string{"FixtureWebUrl"},
	})
}

func Dock13ChangeResolution(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Boot-up and Sign-In to the device ")

	// step 2 - connect ext-display
	if err := dock13ChangeResolutionStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// step 3 - connect docking station
	if err := dock13ChangeResolutionStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step 3: ", err)
	}

	// step 4 - change resolution - low / medium / high
	if err := dock13ChangeResolutionStep4(ctx, s, tconn); err != nil {
		s.Fatal("Fatal to execute step 4: ", err)
	}
}

func dock13ChangeResolutionStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := switchFixture(s, "Display_HDMI_Switch", "ID2", "1", "0"); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock13ChangeResolutionStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook")

	if err := switchFixture(s, "Docking_TYPEC_Switch", "ID1", "1", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in station to chromebook")
	}

	return nil
}

func dock13ChangeResolutionStep4(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	if err := testing.Poll(ctx, func(c context.Context) error {
		// get external display info
		extDispInfo, err := getExternalDisplay(ctx, s, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display info")
		}

		s.Log("external info : ", extDispInfo)

		s.Logf("length of ext-display's mode is %d", len(extDispInfo.Modes))

		low := extDispInfo.Modes[0]

		medium := extDispInfo.Modes[(len(extDispInfo.Modes)-1)/2]

		high := extDispInfo.Modes[len(extDispInfo.Modes)-1]

		// change resolution - (low, medium, highest), then check
		// 	using mode to change
		for _, param := range []struct {
			displayMode display.DisplayMode
		}{
			{*low}, {*medium}, {*high},
		} {

			mode := param.displayMode

			s.Log("Setting display properties: mode = ", mode)

			p := display.DisplayProperties{DisplayMode: &mode}
			if err := display.SetDisplayProperties(ctx, tconn, extDispInfo.ID, p); err != nil {
				return errors.Wrap(err, "failed to set display properties")
			}

			testing.Sleep(ctx, 5*time.Second)

			// get external display info
			info, err := getExternalDisplay(ctx, s, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get external display info")
			}

			// check external display info resolution
			if info.Bounds.Width != mode.Width || info.Bounds.Height != mode.Height {
				return errors.Wrap(err, "failed to check width and height")
			}

		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil

}

func switchFixture(s *testing.State, whatType, index, cmd, interval string) error {

	WWCBURL, ok := s.Var("FixtureWebUrl")
	if !ok {
		return errors.New("failed to get vars WWCB url")
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/switchfixture?Type=%s&Index=%s&cmd=%s&Interval=%s",
		WWCBURL,
		whatType,
		index,
		cmd,
		interval)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	// notice : is "Success "
	if m["resultCode"] != "0000" || m["resultTxt"] != "Success" {
		return errors.Errorf("failed to check response: %v", data)
	}

	// print response
	s.Log("response: ", data)

	return nil
}

// getExternalDisplay to get display with attribute is not internal
func getExternalDisplay(ctx context.Context, s *testing.State, tconn *chrome.TestConn) (*display.Info, error) {
	return display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return !info.IsInternal
	})
}
