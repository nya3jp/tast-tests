// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Exceptions,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that JavaScript exceptions are reported correctly",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline"},
	})
}

func Exceptions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()

	const msg = "intentional error"
	checkError := func(name string, err error) {
		if err == nil {
			s.Errorf("%s didn't return expected error", name)
		} else if !strings.Contains(err.Error(), msg) {
			s.Errorf("%s returned error %q, which doesn't contain %q", name, err.Error(), msg)
		}
	}

	var i int
	checkError("Eval", conn.Eval(ctx, fmt.Sprintf("throw new Error(%q)", msg), &i))
	checkError("Eval (reject string)",
		conn.Eval(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { reject(%q); })", msg), &i))
	checkError("Eval (reject Error)",
		conn.Eval(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { reject(new Error(%q)); })", msg), &i))
	checkError("Eval (throw from Promise)",
		conn.Eval(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { throw new Error(%q); })", msg), &i))
}
