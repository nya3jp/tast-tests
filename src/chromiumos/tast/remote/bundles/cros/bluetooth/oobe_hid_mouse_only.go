// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/rpc"
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
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	resp, error := fv.BTS.WaitForCancelButton(ctx, &emptypb.Empty{})
	if error != nil {
		fs := dutfs.NewClient(cl.Conn)

		out, err := fs.ReadFile(ctx, resp.UiTreeFilePath)
		if err != nil {
			s.Fatalf("Failed to get UI tree file %s: %v", resp.UiTreeFilePath, err)
		}

		uiTreePath := filepath.Join(s.OutDir(), "ui_tree.txt")
		if err := ioutil.WriteFile(uiTreePath, out, 0644); err != nil {
			s.Fatalf("File write ui tree file %s: ", uiTreePath)
		}

		s.Fatal("Failed to find cancel button: ", error)
	}

	// fv := s.FixtValue().(*bluetooth.FixtValue)

	// if _, err := fv.BTPeers[0].GetMacAddress(ctx); err != nil {
	// 	s.Fatal("Failed to call chamleleond method 'GetMacAddress' on btpeer1: ", err)
	// }
}
