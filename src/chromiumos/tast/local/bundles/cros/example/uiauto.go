// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/restrictions"
	"chromiumos/tast/testing"
)

type testConf struct {
	useName bool
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "exampleFixture",
		Desc:            "An example",
		Contacts:        []string{},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Impl:            &exampleFixture{},
		// Uncomment below line to make this comply the rule
		// Labels: []string{restrictions.FragileUIMatcherLabel},
	})
	testing.AddTest(&testing.Test{
		Func:     UIauto,
		Desc:     "UIauto",
		Contacts: []string{"yamaguchi@chromium.org", "tast-owners@google.com"},
		Attr:     []string{},
		Params: []testing.Param{{
			// fail
			Name:    "disallow",
			Fixture: "chromeLoggedIn",
			Val:     testConf{true},
			// no ExtraLabels
		}, {
			// pass
			Name:        "allow",
			Fixture:     "chromeLoggedIn",
			Val:         testConf{true},
			ExtraLabels: []string{restrictions.FragileUIMatcherLabel},
		}, {
			// pass
			Name:    "nouse_name",
			Fixture: "chromeLoggedIn",
			Val:     testConf{false},
		}, {
			// fail
			Name:    "fixture",
			Fixture: "exampleFixture", // uses Name() matcher inside
			Val:     testConf{false},
		}},
	})
}

func UIauto(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	ui := uiauto.New(tconn)
	if _, err := ui.IsNodeFound(ctx, nodewith.Role("A")); err != nil {
		s.Error("nodewith.Role error: ", err)
	}
	x, _ := s.Param().(testConf)
	if x.useName {
		if _, err := ui.IsNodeFound(ctx, nodewith.Name("A")); err != nil {
			s.Error("nodewith.Name error: ", err)
		}
	}
}

// exampleFixture implements testing.FixtureImpl.
type exampleFixture struct {
}

func (f *exampleFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, _ := chrome.New(ctx)
	tconn, _ := cr.TestAPIConn(ctx)
	// TODO: handle error
	ui := uiauto.New(tconn)
	if _, err := ui.IsNodeFound(ctx, nodewith.Name("A")); err != nil {
		s.Error("nodewith.Name error: ", err)
	}
	return cr
}

func (f *exampleFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *exampleFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *exampleFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *exampleFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}
