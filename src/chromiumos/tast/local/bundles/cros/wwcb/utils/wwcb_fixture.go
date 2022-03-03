// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// InitFixtures reset all fixtures at the beginning of testing
// in case, between tests' fixture status affect each other, cause testing failed
func InitFixtures(ctx context.Context) error {
	testing.ContextLog(ctx, "Initialize fixtures")
	// disconnect all fixtures
	api := fmt.Sprintf("api/closeall")
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return errors.Wrap(err, "failed to disconnect all fixtures")
	}
	// turn off docking station's power
	if err := ControlAviosys(ctx, "0", "1"); err != nil {
		return errors.Wrap(err, "failed to turn off docking station's power")
	}
	// turn on docking station's power
	if err := ControlAviosys(ctx, "1", "1"); err != nil {
		return errors.Wrap(err, "failed to turn on docking station's power")
	}
	return nil
}

// ControlFixture do switch fixture
// switchType means what kind fixture, like TYPE-A, TYPE-C
// switchIndex means which fixture ID in this "switchType", like ID1, ID2, ID3.. sequentially
// action means what kind action wnat to do, like plug in or unplug
// needToDelay means that is there a need for delaying seconds for response time for peripherals connect to docking station
func ControlFixture(ctx context.Context, s *testing.State, switchType, switchIndex string, action ActionState, needToDelay bool) error {

	// var interval string
	// if needToDelay {
	// 	interval = "5"
	// } else {
	// 	interval = "0"
	// }

	// restrict input range
	if action < ActionUnplug || action > ActionFlip {
		return errors.Errorf("Incorrect action value; got %d, want [%d - %d]", action, ActionUnplug, ActionFlip)
	}

	// according to input action & fixture
	// to correspond port to switch
	port := port(action, switchType)
	if port == "" {
		return errors.New("failed to get correspond port")
	}

	// according parameter, to switch fixture
	// if err := SwitchFixture(ctx, s, switchType, switchIndex, port, interval); err != nil {
	// 	return errors.Wrap(err, "failed to execute SwitchFixture")
	// }

	// waiting for chromebook response
	var delayTime int
	if needToDelay {
		delayTime = 1
	} else {
		delayTime = 10
	}
	testing.Sleep(ctx, time.Duration(delayTime)*time.Second)

	return nil
}

// ActionState define action possible state
type ActionState int

// fixture possible status
const (
	ActionUnplug ActionState = 0
	ActionPlugin ActionState = 1
	ActionFlip   ActionState = 2
)

// String returns a human-readable string representation for type ActionState.
func (a ActionState) String() string {
	switch a {
	case ActionUnplug:
		return "Unplug"
	case ActionPlugin:
		return "Plug in"
	case ActionFlip:
		return "Flip"
	default:
		return fmt.Sprintf("Unknown action state: %d", a)
	}
}

func hdmiPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func typecPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "0"
	case ActionPlugin:
		return "1"
	case ActionFlip:
		return "2"
	default:
		return ""
	}
}

func typeaPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func ethernetPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func dpPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

// port get corrspond port for what action to type
// Control switch fixture
// Type:HDMI_Switch & TYPEC_Switch & TYPEA_Switch
// Index:ID1 & ID2 & ID3....
// cmd:
// HDMI,0:Close All;1:PortA;2:PortB;3:PortC;4:PortD
// Type-C,1:PortA;2:PortB AUS19129:(0:Close;1:Normal;2:Filp)
// Type-A,1:PortA;2:PortB
// DP,1:PortA;2PortB;3:Close
// Ethernet,1:On;2:Off
// resultCode：0000 成功
// resultTxt：回應之訊息。
func port(action ActionState, switchType string) string {

	// hdmi
	if strings.Contains(switchType, SwitchHDMI) {
		return hdmiPort(action)
	}

	// type a
	if strings.Contains(switchType, SwitchTYPEA) {
		return typeaPort(action)
	}

	// type c
	if strings.Contains(switchType, SwitchTYPEC) {
		return typecPort(action)
	}

	// dp
	if strings.Contains(switchType, SwitchDP) {
		return dpPort(action)
	}

	// ethernet
	if strings.Contains(switchType, SwitchETHERNET) {
		return ethernetPort(action)
	}

	return ""

}
