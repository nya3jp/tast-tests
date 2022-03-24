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
		Func: IwDump,
		Desc: "Dump SoC capabilities",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func IwDump(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
	text, err := iwr.DumpPhys(ctx)
	if err != nil {
		s.Fatal("DumpPhys failed: ", err)
	}
	if len(text) == 0 {
		s.Fatal("Empty iw list result")
	}

	s.Log(text)
}
