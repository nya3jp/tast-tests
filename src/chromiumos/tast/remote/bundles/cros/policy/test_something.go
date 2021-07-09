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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      1 * time.Minute,
	})
}

func TestSomething(ctx context.Context, s *testing.State) {
	creds, err := google.FindDefaultCredentials(ctx, "")
	if err != nil {
		s.Fatal("Failed to create client with default account")
	}
	s.Log(creds)

	ts, err := google.DefaultTokenSource(ctx, "")
	if err != nil {
		s.Fatal("Failed to create client with default account")
	}
	s.Log(ts)

	client, err := google.DefaultClient(ctx, "")
	if err != nil {
		s.Fatal("Failed to create client with default account")
	}
	s.Log(client)
}
