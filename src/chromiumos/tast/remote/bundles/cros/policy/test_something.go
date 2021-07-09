// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"golang.org/x/oauth2/google"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestSomething,
		Desc: "Behavior of TestSomething policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      1 * time.Minute,
	})
}

func TestSomething(ctx context.Context, s *testing.State) {
	creds, err := google.FindDefaultCredentials(ctx, "")
	if err != nil {
		s.Fatal("Failed to find default credentials: ", err)
	}
	s.Log(creds)

	ts, err := google.DefaultTokenSource(ctx, "")
	if err != nil {
		s.Fatal("Failed to create default token source: ", err)
	}
	s.Log(ts)

	client, err := google.DefaultClient(ctx, "")
	if err != nil {
		s.Fatal("Failed to create client with default account: ", err)
	}
	s.Log(client)

	resp, err := client.Get("https://test-dot-tape-307412.appspot.com/test")
	if err != nil {
		s.Fatal("calling TAPE failed: ", err)
	}
	s.Log(resp)
	s.Fatal("Failed successfully")
}
