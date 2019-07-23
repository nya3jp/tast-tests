// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"context"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiReset,
		Desc:     "",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}
func WifiReset(ctx context.Context, s *testing.State) {

}
