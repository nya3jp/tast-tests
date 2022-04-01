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
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// connectState for connected status
type connectState bool

// fixture status
const (
	isConnect    connectState = true
	isDisconnect connectState = false
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock3UsbCharging,
		Desc:         "Test power charging via a powered Dock over USB-C",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"FixtureWebUrl"},
		Pre:          chrome.LoggedIn(), // 1)  Boot-up and Sign-In to the device
	})
}

func Dock3UsbCharging(ctx context.Context, s *testing.State) {

	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := dock3UsbChargingStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := dock3UsbChargingStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - verification
	if err := dock3UsbChargingStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

}

func dock3UsbChargingStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := switchFixtures(s, "Display_HDMI_Switch", "ID2", "1", "0"); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock3UsbChargingStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook")

	if err := switchFixtures(s, "Docking_TYPEC_Switch", "ID1", "1", "0"); err != nil {
		return errors.Wrap(err, "failed to plug in station to chromebook")
	}

	return nil
}

func dock3UsbChargingStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Check chromebook is charging or not")

	if err := verifyPowerStatus(ctx, s, isConnect); err != nil {
		return err
	}

	return nil
}

func switchFixtures(s *testing.State, whatType, index, cmd, interval string) error {

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

// verifyPowerStatus verfiy power is charging or discharging
func verifyPowerStatus(ctx context.Context, s *testing.State, state connectState) error {

	s.Log("Start verifying power status")

	// define expect state to check
	var wantStatus string
	if state {
		wantStatus = "CHARGING"
	} else {
		wantStatus = "DISCHARGING"
	}

	command := testexec.CommandContext(ctx, "cat", "/sys/class/power_supply/BAT0/status")

	s.Logf("%s", command)

	output, err := command.Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// check currentStatus is match condition
	currentStatus := strings.ToUpper(strings.TrimSpace(string(output)))
	if currentStatus != wantStatus {
		return errors.Errorf("Power status is not match, got %s, want %s", currentStatus, wantStatus)
	}

	return nil
}
