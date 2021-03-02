// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IWGetSet,
		Desc: "Test IW getter and setter functions",
		Contacts: []string{
			"deanliao@google.com",             // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func IWGetSet(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
	res, err := iwr.RegulatoryDomain(ctx)
	if err != nil {
		s.Fatal("GetRegulatoryDomain failed: ", err)
	}
	s.Log("Regulatory Domain: ", res)
	// TODO: Flesh out the test and add unit tests to include more of the getters/setters. Tests
	// can't really be made for them right now because they require a link to
	// an AP.
}
