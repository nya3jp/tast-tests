// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        BlockingSync,
		Desc:        "Verifies that firmware tests can remotely perform a blocking sync on the DUT",
		Contacts:    []string{"cros-fw-engprod@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
	})
}

func BlockingSync(ctx context.Context, s *testing.State) {
	h := firmware.NewHelper(s.DUT())
	defer h.Close(ctx)
	h.RPCHint = s.RPCHint()
	h.RequireRPCUtils(ctx)

	if _, err := h.RPCUtils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during BlockingSync: ", err)
	}
}
