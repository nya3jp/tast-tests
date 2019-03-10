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
		Desc:         "Checks that JavaScript exceptions are reported correctly",
		Contacts:     []string{"derat@chromium.org"},
		SoftwareDeps: []string{"chrome_login"},
		Pre:          chrome.LoggedIn(),
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
	checkError("Exec", conn.Exec(ctx, fmt.Sprintf("throw new Error(%q)", msg)))
	checkError("Eval", conn.Eval(ctx, fmt.Sprintf("throw new Error(%q)", msg), &i))
	checkError("EvalPromise (reject string)",
		conn.EvalPromise(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { reject(%q); })", msg), &i))
	checkError("EvalPromise (reject Error)",
		conn.EvalPromise(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { reject(new Error(%q)); })", msg), &i))
	checkError("EvalPromise (throw)",
		conn.EvalPromise(ctx, fmt.Sprintf("new Promise(function(resolve, reject) { throw new Error(%q); })", msg), &i))
}
