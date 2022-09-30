// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExampleTestDumpUITree,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test that dumps the UI tree",
		Contacts: []string{
			"chadduffin@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.baserpc.FaillogService",
			"tast.cros.browser.ChromeService",
		},
		Timeout: time.Second * 15,
	})
}

// ExampleTestDumpUITree ...
func ExampleTestDumpUITree(ctx context.Context, s *testing.State) {
	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create RPC client: ", err)
	}

	chrome := ui.NewChromeServiceClient(rpcClient.Conn)
	faillog := baserpc.NewFaillogServiceClient(rpcClient.Conn)

	if _, err = chrome.New(ctx, &ui.NewRequest{
		LoginMode: ui.LoginMode_LOGIN_MODE_GUEST_LOGIN,
	}); err != nil {
		s.Fatal("Failed to open Chrome on the DUT: ", err)
	}

	if response, err := faillog.Create(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to dump the UI tree")
	} else {
		testing.ContextLog(ctx, "UI tree dumped to "+response.Path)
	}

	if _, err = chrome.Close(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to close Chrome on the DUT: ", err)
	}

	if err = rpcClient.Close(ctx); err != nil {
		s.Error("Failed to close RPC client: ", err)
	}
}
