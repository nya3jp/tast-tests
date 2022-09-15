// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/remote/crosserverutil"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeHidMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a bluetooth mouse is connected to in OOBE",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BTTestService"},
		Fixture:      "chromeOobeWith1BTPeer",
		Timeout:      time.Second * 15,
	})
}

// OobeHidMouseOnly tests that a single Blueooth mouse is connected to during OOBE.
func OobeHidMouseOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cl, err := crosserverutil.GetGRPCClient(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	defer fv.BTS.DumpUITree(cleanupCtx, &emptypb.Empty{})

	uiautoSvc := ui.NewAutomationServiceClient(cl.Conn)
	continueButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "BluetoothHidDetector"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: continueButtonFinder}); err != nil {
		log.Println("==== DumpUITree error ====!")
		if _, er := fv.BTS.DumpUITree(cleanupCtx, &emptypb.Empty{}); er != nil {
			s.Error("Failed to DumpUITree: ", er)
		}
		s.Fatal("Failed to find continue button: ", err)
	}
	// fv := s.FixtValue().(*bluetooth.FixtValue)

	// if _, err := fv.BTPeers[0].GetMacAddress(ctx); err != nil {
	// 	s.Fatal("Failed to call chamleleond method 'GetMacAddress' on btpeer1: ", err)
	// }
}
