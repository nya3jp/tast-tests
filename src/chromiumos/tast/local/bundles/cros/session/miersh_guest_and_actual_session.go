// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MiershGuestAndActualSession,
		Desc: "Ensures that the session_manager correctly handles ownership when a guest signs in before user",
		Contacts: []string{
			"hidehiko@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

// DO NOT SUBMIT MIERSH this doesn't work

func MiershGuestAndActualSession(ctx context.Context, s *testing.State) {
	const testUser = "first_user@nowhere.com"

	// cmdRunner := hwseclocal.NewCmdRunner()

	// helper, err := hwseclocal.NewHelper(cmdRunner)
	// if err != nil {
	// 	s.Fatal("Failed to create hwsec local helper: ", err)
	// }

	// // Resets the TPM, system, and user states before running the tests.
	// if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
	// 	s.Fatal("Failed to reset TPM or system states: ", err)
	// }

	if err := session.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	if err := cryptohome.MountGuest(ctx); err != nil {
		s.Fatal("Failed to mount guest: ", err)
	}

	if err := sm.StartSession(ctx, cryptohome.GuestUser, ""); err != nil {
		s.Fatal("Failed to start guest session: ", err)
	}

	guestChrome, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		s.Fatal("Failed to log in with Chrome: ", err)
	}
	defer guestChrome.Close(ctx)

	guestTookOwnership := false

	wp, err := sm.WatchPropertyChangeComplete(ctx)
	if err != nil {
		s.Fatal("Failed to start watching PropertyChangeComplete signal: ", err)
	}
	defer wp.Close(ctx)
	ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	if err != nil {
		s.Fatal("Failed to start watching SetOwnerKeyComplete signal: ", err)
	}
	defer ws.Close(ctx)

	timer := time.NewTimer(10 * time.Second)

	select {
	case <-wp.Signals:
		guestTookOwnership = true
	case <-ws.Signals:
		guestTookOwnership = true
	case <-timer.C:
		if guestTookOwnership {
			s.Fatal("Guest user took ownership")
		}
	}

	s.Log("waiting done, guestTookOwnership: ", guestTookOwnership)

	// // Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// // manually restart "ui" job for the emulation.
	// if err := upstart.RestartJob(ctx, "ui"); err != nil {
	// 	s.Fatal("Failed to log out: ", err)
	// }

	tconn, err := guestChrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test api: ", err)
	}
	if err := tconn.Call(ctx, nil, "chrome.autotestPrivate.logout"); err != nil {
		s.Fatal("Failed to logout: ", err)
	} else {
		s.Log("logout success")
	}

	normalChrome, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: testUser, Pass: "123"}))
	if err != nil {
		s.Fatal("Failed to log in with Chrome: ", err)
	}
	defer normalChrome.Close(ctx)

	select {
	case <-wp.Signals:
	case <-ws.Signals:
	case <-ctx.Done():
		s.Fatal("Timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal: ", ctx.Err())
	}

	// s.Log("HERE 2 ==========================================")
	// time.Sleep(60 * time.Second)

	// // Note: presumably the guest session would actually stop and the
	// // guest dir would be unmounted before a regular user would sign in.
	// // This should be revisited later.
	// if err := cryptohome.CreateVault(ctx, testUser, ""); err != nil {
	// 	s.Fatalf("Failed to create a vault for %s: %v", testUser, err)
	// }

	// wp, err := sm.WatchPropertyChangeComplete(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to start watching PropertyChangeComplete signal: ", err)
	// }
	// defer wp.Close(ctx)
	// ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to start watching SetOwnerKeyComplete signal: ", err)
	// }
	// defer ws.Close(ctx)

	// if err := sm.StartSession(ctx, testUser, ""); err != nil {
	// 	s.Fatalf("Failed to start session for %s: %v", testUser, err)
	// }

	// select {
	// case <-wp.Signals:
	// case <-ws.Signals:
	// case <-ctx.Done():
	// 	s.Fatal("Timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal: ", ctx.Err())
	// }

	ret, err := sm.RetrievePolicyEx(ctx, session.DevicePolicyDescriptor())
	if err != nil {
		s.Fatal("Failed to retrieve policy: ", err)
	}

	pol := &enterprise_management_proto.PolicyData{}
	if err = proto.Unmarshal(ret.PolicyData, pol); err != nil {
		s.Fatal("Failed to parse PolicyData: ", err)
	}

	if *pol.Username != testUser {
		s.Errorf("Unexpected user name: got %s; want %s", *pol.Username, testUser)
	}
}
