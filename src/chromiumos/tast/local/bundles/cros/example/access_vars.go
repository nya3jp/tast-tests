// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/common/global"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AccessVars,
		Desc:     "Access variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func AccessVars(ctx context.Context, s *testing.State) {
	if boolVal, ok := global.ExampleBoolVar.Value(); boolVal != true || !ok {
		s.Errorf("Got global variable value (%v, %v) from ContextVar want (%v, %v)", boolVal, ok, true, true)
	}
	if strVal, ok := global.ExampleStrVar.Value(); strVal != "test" || !ok {
		s.Errorf("Got global variable value (%q, %v) from ContextVar want (%q, %v)", strVal, ok, "test", true)
	}
	expected := global.ExampleStruct{Name: "t1", Value: 8}
	structVal, ok := global.ExampleStructVar.Value()
	if !ok {
		s.Fatal("Failed to find global variable ", global.ExampleStructVar)
	}
	if structVal.Name != expected.Name || structVal.Value != expected.Value {
		s.Errorf("Got global variable value (%v) from ContextVar want (%v)", structVal, expected)
	}
}
