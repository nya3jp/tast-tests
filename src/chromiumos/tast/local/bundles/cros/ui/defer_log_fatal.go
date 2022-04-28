// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeferLogFatal,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Investigation of what if a deferred function call invokes s.Fatal()",
		Contacts:     []string{"amusbach@chromium.org", "achuith@chromium.org", "chromeos-wmp@google.com"},
	})
}

func DeferLogFatal(ctx context.Context, s *testing.State) {
	defer s.Log("This message is deferred first")
	defer func() {
		s.Fatal("This is the error")
		s.Log("This message comes after the error, in the same deferred function call")
	}()
}
