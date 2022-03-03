// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
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

// ControlPeripherals such as ext-display1, ethernet, usbs
func ControlPeripherals(ctx context.Context, s *testing.State, uc *UsbController, action ActionState, needToDelay bool) error {

	// ext-display 1
	if err := ControlFixture(ctx, s, ExtDisp1Type, ExtDisp1Index, action, needToDelay); err != nil {
		return errors.Wrap(err, "failed to swithc fixture on ext-display")
	}

	// ethernet
	if err := ControlFixture(ctx, s, EthernetType, EthernetIndex, action, needToDelay); err != nil {
		return errors.Wrap(err, "failed to switch fixture on ethernet")
	}

	// usbs
	if err := uc.ControlUsbs(ctx, s, action, needToDelay); err != nil {
		return errors.Wrap(err, "failed to switch fixture on usb")
	}

	// audio
	return nil
}
