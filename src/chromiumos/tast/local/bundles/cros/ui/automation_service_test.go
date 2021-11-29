// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"regexp"
	gotesting "testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	pb "chromiumos/tast/services/cros/ui"
)

func TestToRole(t *gotesting.T) {

	for _, tc := range []struct {
		r1 pb.Role_Role
		r2 role.Role
	}{
		{pb.Role_ALERT_DIALOG, role.AlertDialog},
		{pb.Role_ALERT, role.Alert},
		{pb.Role_COMBO_BOX_MENU_BUTTON, role.ComboBoxMenuButton},
	} {
		got, err := toRole(tc.r1)
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
		f1 *pb.Finder
		f2 *nodewith.Finder
	}{
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_HasClass{HasClass: "myTextArea"}},
				{Value: &pb.NodeWith_Name{Name: "NAME"}},
				{Value: &pb.NodeWith_Role{Role: pb.Role_ALERT_DIALOG}},
				{Value: &pb.NodeWith_Nth{Nth: 2}},
				{Value: &pb.NodeWith_Focused{}},
				{Value: &pb.NodeWith_Required{}},
				{Value: &pb.NodeWith_State{
					State: &pb.NodeWith_StateValue{
						State: pb.State_DEFAULT,
						Value: false,
					},
				}},
			},
		}, nodewith.HasClass("myTextArea").Name("NAME").Role(role.AlertDialog).Nth(2).Focused().Required().State(state.Default, false)},
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_NameRegex{NameRegex: "What('|’)s (n|N)ew"}},
			},
		}, nodewith.NameRegex(regexp.MustCompile("What('|’)s (n|N)ew"))},
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_NameStartingWith{NameStartingWith: "Chrome"}},
			},
		}, nodewith.NameStartingWith("Chrome")},
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_NameContaining{NameContaining: "Chrome"}},
			},
		}, nodewith.NameContaining("Chrome")},
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_Root{}},
			},
		}, nodewith.Root()},
		{&pb.Finder{
			NodeWiths: []*pb.NodeWith{
				{Value: &pb.NodeWith_HasClass{HasClass: "NewTabButton"}},
				{Value: &pb.NodeWith_Role{Role: pb.Role_BUTTON}},
				{Value: &pb.NodeWith_Visible{}},
				{Value: &pb.NodeWith_Onscreen{}},
				{Value: &pb.NodeWith_First{}},
				{Value: &pb.NodeWith_Ancestor{
					Ancestor: &pb.Finder{
						NodeWiths: []*pb.NodeWith{
							{Value: &pb.NodeWith_HasClass{HasClass: "BrowserFrame"}},
							{Value: &pb.NodeWith_Role{Role: pb.Role_WINDOW}},
						},
					},
				}},
			},
		}, nodewith.HasClass("NewTabButton").Role(role.Button).Visible().Onscreen().
			First().Ancestor(nodewith.Role(role.Window).HasClass("BrowserFrame"))},
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

func TestToFinderRoot(t *gotesting.T) {
	f1 := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_Root{}},
			{Value: &pb.NodeWith_Name{Name: "NAME"}},
		},
	}
	if _, err := toFinder(f1); err == nil {
		t.Errorf("Can only have 1 predicate when Nodewith.Root() is used: %v", err)
	}
}

func TestToConstCase(t *gotesting.T) {
	for _, tc := range []struct {
		camelCase string
		constCase string
	}{
		{"abbr", "ABBR"},
		{"comboBoxMenuButton", "COMBO_BOX_MENU_BUTTON"},
	} {
		got := toConstCase(tc.camelCase)
		want := tc.constCase
		if !cmp.Equal(got, want) {
			t.Errorf("Unexpected toConstCase conversion, got %v, want %v", got, want)
		}
	}
}

func TestToRolePB(t *gotesting.T) {
	for _, tc := range []struct {
		r1 role.Role
		r2 pb.Role_Role
	}{
		{role.Abbr, pb.Role_ABBR},
		{role.ComboBoxMenuButton, pb.Role_COMBO_BOX_MENU_BUTTON},
	} {
		got := toRolePB(tc.r1)
		want := tc.r2
		if !cmp.Equal(got, want) {
			t.Errorf("Unexpected toRolePB conversion, got %v, want %v", got, want)
		}
	}
}
