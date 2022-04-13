// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils provides funcs to cleanup folders in ChromeOS.
package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ControlFixture do switch fixture
func ControlFixture(ctx context.Context, s *testing.State, switchType, switchIndex string, action ActionState, needToDelay bool) error {

	var interval string
	if needToDelay {
		interval = "5"
	} else {
		interval = "0"
	}

	// restrict input range
	if action < ActionUnplug || action > ActionFlip {
		return errors.Errorf("Incorrect action value: got %d, want [%d - %d]", action, ActionUnplug, ActionFlip)
	}

	// according to input action & fixture
	// to correspond port to switch
	port := getPort(action, switchType)
	if port == "" {
		return errors.New("failed to get correspond port")
	}

	// according parameter, to switch fixture
	if err := SwitchFixture(s, switchType, switchIndex, port, interval); err != nil {
		return errors.Wrap(err, "failed to execute SwitchFixture")
	}

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

// // ControlPeripherals such as ext-display1, ethernet, usbs
// func ControlPeripherals(ctx context.Context, s *testing.State, uc *UsbController, action ActionState, needToDelay bool) error {

// 	// ext-display 1
// 	if err := ControlFixture(ctx, s, ExtDisp1Type, ExtDisp1Index, action, needToDelay); err != nil {
// 		return errors.Wrap(err, "failed to swithc fixture on ext-display")
// 	}

// 	// ethernet
// 	if err := ControlFixture(ctx, s, EthernetType, EthernetIndex, action, needToDelay); err != nil {
// 		return errors.Wrap(err, "failed to switch fixture on ethernet")
// 	}

// 	// usbs
// 	if err := uc.ControlUsbs(ctx, s, action, needToDelay); err != nil {
// 		return errors.Wrap(err, "failed to switch fixture on usb")
// 	}

// 	// audio
// 	return nil
// }

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

func getHdmiPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "0"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getTypeCPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	case ActionFlip:
		return "2"
	default:
		return ""
	}
}

func getTypeAPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getEthernetPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getDPPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

// getPort get corrspond port for what action to type
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
func getPort(action ActionState, switchType string) string {

	// hdmi
	if strings.Contains(switchType, SwitchHDMI) {
		return getHdmiPort(action)
	}

	// type a
	if strings.Contains(switchType, SwitchTYPEA) {
		return getTypeAPort(action)
	}

	// type c
	if strings.Contains(switchType, SwitchTYPEC) {
		return getTypeCPort(action)
	}

	// dp
	if strings.Contains(switchType, SwitchDP) {
		return getDPPort(action)
	}

	// ethernet
	if strings.Contains(switchType, SwitchETHERNET) {
		return getEthernetPort(action)
	}

	return ""

}
