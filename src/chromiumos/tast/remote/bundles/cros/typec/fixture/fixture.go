// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture contains fixtures which can be used while running Type C tests.
package fixture

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "typeCServo",
		Desc:            "Set up Servo for Type C tests",
		Contacts:        []string{"pmalani@chromium.org", "chromeos-usb@google.com"},
		Impl:            &impl{},
		Vars:            []string{"servo"},
		SetUpTimeout:    5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		PreTestTimeout:  2 * time.Minute,
		PostTestTimeout: 2 * time.Minute,
	})
}

// Value allows tests to obtain servo object pointers. It also stashes some state used during TearDown.
type Value struct {
	pxy         *servo.Proxy
	svo         *servo.Servo
	keepaliveEn bool
}

// Servo returns a pointer to the saved servo object.
func (v *Value) Servo() *servo.Servo {
	return v.svo
}

type impl struct {
	v *Value
}

func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	i.v = &Value{pxy: pxy, svo: pxy.Servo()}

	return i.v
}

func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	i.v.pxy.Close(ctx)
}

func (i *impl) Reset(ctx context.Context) error {
	return nil
}

func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	svo := i.v.svo

	// Configure Servo to be OK with CC being off. Only bother doing this if the device has CCD.
	if ret, err := svo.HasCCD(ctx); err != nil {
		s.Fatal("Failed to check servo CCD capability: ", err)
	} else if ret {
		if err := svo.SetOnOff(ctx, servo.CCDKeepaliveEn, servo.Off); err != nil {
			s.Log("Failed to disable CCD keepalive: ", err)
		} else {
			i.v.keepaliveEn = true
		}
	}

	// Wait for servo control to take effect.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep after CCD keepalive disable: ", err)
	}

	if err := svo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to switch CCD watchdog off: ", err)
	}

	// Wait for servo control to take effect.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep after CCD watchdog off: ", err)
	}

	// Turn CC Off before modifying DTS Mode.
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		s.Fatal("Failed CC off and wait: ", err)
	}

	// Servo DTS mode needs to be off to configure enable DP alternate mode support.
	if err := svo.SetOnOff(ctx, servo.DTSMode, servo.Off); err != nil {
		s.Fatal("Failed to disable Servo DTS mode: ", err)
	}

	// Wait for DTS-off PD negotiation to complete.
	if err := testing.Sleep(ctx, 2500*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep for DTS-off power negotiation: ", err)
	}
}

func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	svo := i.v.svo

	// Turn CC Off before modifying DTS Mode in cleanup.
	if err := typecutils.CcOffAndWait(ctx, svo); err != nil {
		s.Fatal("Failed CC off and wait: ", err)
	}

	if err := svo.SetOnOff(ctx, servo.DTSMode, servo.On); err != nil {
		s.Error("Failed to enable Servo DTS mode: ", err)
	}

	// Wait for DTS-on PD negotiation to complete.
	if err := testing.Sleep(ctx, 2500*time.Millisecond); err != nil {
		s.Fatal("Failed to sleep for DTS-on power negotiation: ", err)
	}

	// Make sure that CC is switched on at the end of the test.
	if err := svo.SetCC(ctx, servo.On); err != nil {
		s.Error("Unable to enable Servo CC: ", err)
	}

	if i.v.keepaliveEn {
		if err := svo.SetOnOff(ctx, servo.CCDKeepaliveEn, servo.On); err != nil {
			s.Log("Unable to enable CCD keepalive: ", err)
		}
	}
}
