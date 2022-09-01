// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OwnershipTaken,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Sign in and ensure that ownership of the device is taken",
		Contacts: []string{
			"hidehiko@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "group:asan"},
	})
}

func OwnershipTaken(ctx context.Context, s *testing.State) {
	const (
		testUser = "ownership_test@chromium.org"
		testPass = "testme"
	)

	//
	// MIERSH clear TPM
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

	if err := session.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

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

	user, ret := func() (string, *enterprise_management_proto.PolicyFetchResponse) {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in with Chrome: ", err)
		}
		defer cr.Close(ctx)

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
		return cr.NormalizedUser(), ret
	}()

	if ret.PolicyData == nil {
		s.Fatal("PolicyFetchResponse does not contain PolicyData")
	}

	pol := &enterprise_management_proto.PolicyData{}
	if err = proto.Unmarshal(ret.PolicyData, pol); err != nil {
		s.Fatal("Failed to parse PolicyData: ", err)
	}
	if polUser, err := session.NormalizeEmail(pol.GetUsername(), true); polUser != user {
		if err != nil {
			s.Fatalf("Failed to normalize username %q: %v", pol.GetUsername(), err)
		}
		s.Fatalf("Unexpected user: got %s; want %s", polUser, user)
	}

	settings := &enterprise_management_proto.ChromeDeviceSettingsProto{}
	if err = proto.Unmarshal(pol.PolicyValue, settings); err != nil {
		s.Fatal("Failed to parse PolicyValue: ", err)
	}

	if !settings.AllowNewUsers.GetAllowNewUsers() {
		s.Fatal("AllowNewUsers should be true")
	}
	found := false
	// TODO(crbug.com/1103816) - remove whitelist support when no longer
	// supported by DMServer.
	if settings.UserWhitelist != nil && settings.UserAllowlist == nil {
		for _, u := range settings.UserWhitelist.UserWhitelist {
			if normUser, _ := session.NormalizeEmail(u, true); normUser == user {
				found = true
				break
			}
		}
	} else {
		for _, u := range settings.UserAllowlist.UserAllowlist {
			if normUser, _ := session.NormalizeEmail(u, true); normUser == user {
				found = true
				break
			}
		}
	}
	if !found {
		s.Fatal("User is not found in the allowlist")
	}
}
