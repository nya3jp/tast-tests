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
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MiershGuestAndActualSession2,
		Desc: "Ensures that the session_manager correctly handles ownership when a guest signs in before user",
		Contacts: []string{
			"hidehiko@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func logout2(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Log("MIERSH err: ", err)
			return err
		}
		if err := tconn.Call(ctx, nil, "chrome.autotestPrivate.logout"); err != nil {
			s.Log("MIERSH err: ", err)
			return err
		}
		return nil

	}, nil); err != nil {
		s.Fatal("Failed to logout: ", err)
	}
}

func MiershGuestAndActualSession2(ctx context.Context, s *testing.State) {
	const testUser = "first_user@nowhere.com"

	//
	// Reset TPM
	//
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Resets the TPM, system, and user states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	//
	// RemoveVault, not sure it's needed
	//

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	//
	// Create the initial Chrome, don't login
	//

	chromeObj, err := chrome.New(ctx, chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to log in with Chrome: ", err)
	}
	defer chromeObj.Close(ctx)

	s.Log("Connect to session manager")

	//
	// Connection to the session manager
	//
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}

	//
	// Login into the user session
	//

	s.Log("MIERSH start guest login")

	chrome.MiershLogIn(ctx, chromeObj, chrome.FakeLogin(chrome.Creds{User: testUser, Pass: "123"}))

	//
	// Logout from the user session
	//

	logout2(ctx, s, chromeObj)

	//
	// Everything works until here, but not further.
	// With the current code it fails after the "Finding OOBE DevTools target" with a timeout error.
	// Also, ideally, the session above should be a guest session,
	// but my Chrome doesn't have the test extension, so the logout for it doesn't work.
	//

	// chrome.FixChromeSession(ctx, chromeObj)

	s.Log("MIERSH HERE 1")

	chrome.MiershLogIn(ctx, chromeObj, chrome.FakeLogin(chrome.Creds{User: testUser, Pass: "123"}))

	logout2(ctx, s, chromeObj)

	s.Log("MIERSH SUCCESS")
	time.Sleep(15 * time.Second)

	//
	// Success
	//

	// guestChrome, err := chrome.New(ctx, chrome.GuestLogin())
	// if err != nil {
	// 	s.Fatal("Failed to log in with Chrome: ", err)
	// }
	// defer guestChrome.Close(ctx)

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

	logout2(ctx, s, chromeObj)

	s.Log("MIERSH SUCCESS")
	time.Sleep(60 * time.Second)

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
