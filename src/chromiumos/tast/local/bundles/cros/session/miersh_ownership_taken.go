// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MiershOwnershipTaken,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Sign in and ensure that ownership of the device is taken",
		Contacts: []string{
			"hidehiko@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "group:asan"},
	})
}

// func loginHelper(ctx context.Context, opts ...Option) retErr error {
// 	agg := jslog.NewAggregator()
// 	defer func() {
// 		if retErr != nil {
// 			agg.Close()
// 		}
// 	}()

// 	sess, err := driver.NewSession(ctx, ashproc.ExecPath, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to establish connection to Chrome Debugging Protocol with debugging port path=%q", cdputil.DebuggingPortPath)
// 	}
// 	defer func() {
// 		if retErr != nil {
// 			sess.Close(ctx)
// 		}
// 	}()

// 	return login.LogIn(ctx, opts, sess)
// }

func MiershOwnershipTaken(ctx context.Context, s *testing.State) {
	// 	const (
	// 		testUser = "ownership_test@chromium.org"
	// 		testPass = "testme"
	// 	)

	// 	//
	// 	// MIERSH clear TPM
	// 	//

	// 	cmdRunner := hwseclocal.NewCmdRunner()

	// 	helper, err := hwseclocal.NewHelper(cmdRunner)
	// 	if err != nil {
	// 		s.Fatal("Failed to create hwsec local helper: ", err)
	// 	}

	// 	// Resets the TPM, system, and user states before running the tests.
	// 	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
	// 		s.Fatal("Failed to reset TPM or system states: ", err)
	// 	}

	// 	//
	// 	// MIERSH clear other stuff
	// 	//

	// 	if err := session.SetUpDevice(ctx); err != nil {
	// 		s.Fatal("Failed to reset device ownership: ", err)
	// 	}

	// 	//
	// 	// MIERSH start chrome, don't login
	// 	//

	// 	cr, err := chrome.New(ctx, chrome.NoLogin())
	// 	if err != nil {
	// 		s.Fatal("Failed to log in with Chrome: ", err)
	// 	}
	// 	defer cr.Close(ctx)

	// 	//
	// 	// MIERSH connect to session_manager
	// 	//

	// 	sm, err := session.NewSessionManager(ctx)
	// 	if err != nil {
	// 		s.Fatal("Failed to create session_manager binding: ", err)
	// 	}
	// 	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
	// 		s.Fatal("Failed to prepare Chrome for testing: ", err)
	// 	}

	// 	// s.Log("OwnershipTaken HERE ==========================================")
	// 	// time.Sleep(5 * time.Second)

	// 	//
	// 	// MIERSH login
	// 	//

	// if err:= loginHelper(ctx, {}); err != nil {
	// 	s.Fatal("Failed to login: ", err)
	// }

	// 	//
	// 	// wait for "ownership taken"
	// 	//

	// 	wp, err := sm.WatchPropertyChangeComplete(ctx)
	// 	if err != nil {
	// 		s.Fatal("Failed to start watching PropertyChangeComplete signal: ", err)
	// 	}
	// 	defer wp.Close(ctx)
	// 	ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	// 	if err != nil {
	// 		s.Fatal("Failed to start watching SetOwnerKeyComplete signal: ", err)
	// 	}
	// 	defer ws.Close(ctx)

	// 	s.Log("OwnershipTaken HERE 1.5 ==========================================")
	// 	time.Sleep(5 * time.Second)

	// 	user, ret := func() (string, *enterprise_management_proto.PolicyFetchResponse) {
	// 		cr, err := chrome.New(ctx)
	// 		if err != nil {
	// 			s.Fatal("Failed to log in with Chrome: ", err)
	// 		}
	// 		defer cr.Close(ctx)

	// 		select {
	// 		case <-wp.Signals:
	// 		case <-ws.Signals:
	// 		case <-ctx.Done():
	// 			s.Fatal("Timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal: ", ctx.Err())
	// 		}

	// 		ret, err := sm.RetrievePolicyEx(ctx, session.DevicePolicyDescriptor())
	// 		if err != nil {
	// 			s.Fatal("Failed to retrieve policy: ", err)
	// 		}
	// 		return cr.NormalizedUser(), ret
	// 	}()

	// 	if ret.PolicyData == nil {
	// 		s.Fatal("PolicyFetchResponse does not contain PolicyData")
	// 	}

	// 	s.Log("OwnershipTaken HERE 2 ==========================================")
	// 	time.Sleep(5 * time.Second)

	// 	pol := &enterprise_management_proto.PolicyData{}
	// 	if err = proto.Unmarshal(ret.PolicyData, pol); err != nil {
	// 		s.Fatal("Failed to parse PolicyData: ", err)
	// 	}
	// 	if polUser, err := session.NormalizeEmail(pol.GetUsername(), true); polUser != user {
	// 		if err != nil {
	// 			s.Fatalf("Failed to normalize username %q: %v", pol.GetUsername(), err)
	// 		}
	// 		s.Fatalf("Unexpected user: got %s; want %s", polUser, user)
	// 	}

	// 	settings := &enterprise_management_proto.ChromeDeviceSettingsProto{}
	// 	if err = proto.Unmarshal(pol.PolicyValue, settings); err != nil {
	// 		s.Fatal("Failed to parse PolicyValue: ", err)
	// 	}

	// 	s.Log("OwnershipTaken HERE 3 ==========================================")
	// 	time.Sleep(5 * time.Second)

	// 	if !settings.AllowNewUsers.GetAllowNewUsers() {
	// 		s.Fatal("AllowNewUsers should be true")
	// 	}
	// 	found := false
	// 	// TODO(crbug.com/1103816) - remove whitelist support when no longer
	// 	// supported by DMServer.
	// 	if settings.UserWhitelist != nil && settings.UserAllowlist == nil {
	// 		for _, u := range settings.UserWhitelist.UserWhitelist {
	// 			if normUser, _ := session.NormalizeEmail(u, true); normUser == user {
	// 				found = true
	// 				break
	// 			}
	// 		}
	// 	} else {
	// 		for _, u := range settings.UserAllowlist.UserAllowlist {
	// 			if normUser, _ := session.NormalizeEmail(u, true); normUser == user {
	// 				found = true
	// 				break
	// 			}
	// 		}
	// 	}
	// 	if !found {
	// 		s.Fatal("User is not found in the allowlist")
	// 	}
}
