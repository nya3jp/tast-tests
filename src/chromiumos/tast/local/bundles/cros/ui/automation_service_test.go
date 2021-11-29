// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/services/cros/ui"
	"regexp"
	gotesting "testing"

	"github.com/google/go-cmp/cmp"
)

func TestToRole(t *gotesting.T) {

	for _, tc := range []struct {
		r1 ui.Role
		r2 role.Role
	}{
		{ui.Role_ROLE_ALERT_DIALOG, role.AlertDialog},
	} {
		got, err := toRole(&tc.r1)
		if err != nil {
			t.Errorf("Failed when calling toRole() for %v", tc.r1)
		}
		want := tc.r2
		if !cmp.Equal(got, want) {
			t.Errorf("Unexpected toRole conversion, got %v, want %v", got, want)
		}
	}
}

func TestToFinder(t *gotesting.T) {

	for _, tc := range []struct {
		f1 *ui.Finder
		f2 *nodewith.Finder
	}{
		{&ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{
					Value: &ui.NodeWith_HasClass{
						HasClass: "myTextArea",
					},
				},
				{
					Value: &ui.NodeWith_Name{
						Name: "NAME",
					},
				},
				{
					Value: &ui.NodeWith_Role{
						Role: ui.Role_ROLE_ALERT_DIALOG,
					},
				},
				{
					Value: &ui.NodeWith_Nth{
						Nth: 2,
					},
				},
				{
					Value: &ui.NodeWith_Focused{},
				},
				{
					Value: &ui.NodeWith_Required{},
				},
				{
					Value: &ui.NodeWith_State{
						State: &ui.NodeWith_StateValue{
							State: ui.State_STATE_DEFAULT,
							Value: false,
						},
					},
				},
			},
		}, nodewith.HasClass("myTextArea").Name("NAME").Role(role.AlertDialog).Nth(2).Focused().Required().State(state.Default, false)},
		{&ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{
					Value: &ui.NodeWith_NameRegex{
						NameRegex: "What('|’)s (n|N)ew",
					},
				},
			},
		}, nodewith.NameRegex(regexp.MustCompile("What('|’)s (n|N)ew"))},
		{&ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{
					Value: &ui.NodeWith_NameStartingWith{
						NameStartingWith: "Chrome",
					},
				},
			},
		}, nodewith.NameStartingWith("Chrome")},
		{&ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{
					Value: &ui.NodeWith_NameContaining{
						NameContaining: "Chrome",
					},
				},
			},
		}, nodewith.NameContaining("Chrome")},
		{&ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{
					Value: &ui.NodeWith_HasClass{
						HasClass: "NewTabButton",
					},
				},
				{
					Value: &ui.NodeWith_Role{
						Role: ui.Role_ROLE_BUTTON,
					},
				},
				{
					Value: &ui.NodeWith_Ancestor{
						Ancestor: &ui.Finder{
							NodeWiths: []*ui.NodeWith{
								{
									Value: &ui.NodeWith_HasClass{
										HasClass: "BrowserFrame",
									},
								},
								{
									Value: &ui.NodeWith_Role{
										Role: ui.Role_ROLE_WINDOW,
									},
								},
							},
						},
					},
				},
			},
		}, nodewith.HasClass("NewTabButton").Role(role.Button).Ancestor(nodewith.Role(role.Window).HasClass("BrowserFrame"))},
	} {
		got, err := toFinder(tc.f1)
		if err != nil {
			t.Errorf("Failed when calling toFinder() for %v", tc.f1)
		}
		want := tc.f2
		if !cmp.Equal(got.Pretty(), want.Pretty()) {
			t.Errorf("Unexpected finder conversion, got %v, want %v", got.Pretty(), want.Pretty())
		}
	}
}
