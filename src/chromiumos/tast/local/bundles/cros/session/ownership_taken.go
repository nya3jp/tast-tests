// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/local/bundles/cros/session/ownership"
	"chromiumos/tast/local/bundles/cros/session/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OwnershipTaken,
		Desc: "Sign in and ensure that ownership of the device is taken",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func OwnershipTaken(ctx context.Context, s *testing.State) {
	const (
		testUser = "ownership_test@chromium.org"
		testPass = "testme"
	)

	if err := ownership.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := util.PrepareChromeForTesting(ctx, sm); err != nil {
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

	user, ret := func() (string, *enterprise_management.PolicyFetchResponse) {
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

		ret, err := sm.RetrievePolicyEx(ctx, ownership.DevicePolicyDescriptor())
		if err != nil {
			s.Fatal("Failed to retrieve policy: ", err)
		}
		return cr.User(), ret
	}()

	if ret.PolicyData == nil {
		s.Fatal("PolicyFetchResponse does not contain PolicyData")
	}

	pol := &enterprise_management.PolicyData{}
	if err = proto.Unmarshal(ret.PolicyData, pol); err != nil {
		s.Fatal("Failed to parse PolicyData: ", err)
	}
	if pol.GetUsername() != user {
		s.Fatalf("Unexpected user: got %s; want %s", pol.GetUsername(), user)
	}

	settings := &enterprise_management.ChromeDeviceSettingsProto{}
	if err = proto.Unmarshal(pol.PolicyValue, settings); err != nil {
		s.Fatal("Failed to parse PolicyValue: ", err)
	}

	if !settings.AllowNewUsers.GetAllowNewUsers() {
		s.Fatal("AllowNewUsers should be true")
	}
	found := false
	for _, u := range settings.UserWhitelist.UserWhitelist {
		if u == user {
			found = true
			break
		}
	}
	if !found {
		s.Fatal("User is not found in the whitelist")
	}
}
