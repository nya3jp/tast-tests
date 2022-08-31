// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"time"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CloseLid,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that screen-locking works by closing lid",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CloseLid(ctx context.Context, s *testing.State) {
	const (
		username      = "testuser@gmail.com"
		password      = "good"
		wrongPassword = "bad"

		setAllowedPref = "tast.promisify(chrome.autotestPrivate.setAllowedPref)"
		prefName       = "settings.enable_screen_lock"

		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
	)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}
	defer kb.Close()

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	emitter, err := power.NewPowerManagerEmitter(ctx)
	if err != nil {
		s.Fatal("Unable to create power manager emitter: ", err)
	}
	defer func() {
		if err := emitter.Stop(ctx); err != nil {
			s.Error("Unable to stop emitter: ", err)
		}
	}()

	s.Log("Screen should not lock")
	if err := conn.Call(ctx, nil, setAllowedPref, prefName, false); err != nil {
		s.Fatal("Failed to enable auto lock: ", err)
	}
	eventType := pmpb.InputEvent_LID_CLOSED
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send LID_CLOSED failed: ", err)
	}
	testing.Sleep(ctx, 2*time.Second)
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
	eventType = pmpb.InputEvent_LID_OPEN
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send LID_OPEN failed: ", err)
	}

	s.Log("Locking screen via lid close")
	if err := conn.Call(ctx, nil, setAllowedPref, prefName, true); err != nil {
		s.Fatal("Failed to enable auto lock: ", err)
	}
	defer func() {
		if err := conn.Call(ctx, nil, setAllowedPref, prefName, false); err != nil {
			s.Error("Set pref false failed: ", err)
		}
	}()
	eventType = pmpb.InputEvent_LID_CLOSED
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send LID_CLOSED failed: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
	eventType = pmpb.InputEvent_LID_OPEN
	if err := emitter.EmitInputEvent(ctx, &pmpb.InputEvent{Type: &eventType}); err != nil {
		s.Fatal("Send LID_OPEN failed: ", err)
	}

	s.Log("Unlocking screen by typing correct password")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Typing correct password failed: ", err)
	}
	if st, err := lockscreen.WaitState(ctx, conn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}
}
