// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type ethernet struct {
	ethtype string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BrowseUsingEthernet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Browse using ethernet LAN",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "native",
			Val:  ethernet{ethtype: "native"},
		}, {
			Name: "type_a",
			Val:  ethernet{ethtype: "typeA"},
		}},
	})
}

func BrowseUsingEthernet(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	testOpts := s.Param().(ethernet)

	if testOpts.ethtype == "typeA" {
		usbDetectionRe := regexp.MustCompile(`Class=.*(480M|5000M|10G|20G)`)
		out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
		if err != nil {
			s.Fatal("Failed to execute lsusb command: ", err)
		}

		if !usbDetectionRe.MatchString(string(out)) {
			s.Fatal("Failed: ethernet is not connected to DUT using type-a adapter")
		}
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Manager object: ", err)
	}

	if ethernetAvailable, err := manager.IsAvailable(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Failed to call IsAvailable: ", err)
	} else if !ethernetAvailable {
		s.Fatal("Failed to verify ethernet, ethernet not available")
	}

	var browseURL = "https://www.google.com/"
	conn, err := cr.NewConn(ctx, browseURL)
	if err != nil {
		s.Fatal(err, "failed to connect to chrome")
	}
	defer conn.Close()
}
