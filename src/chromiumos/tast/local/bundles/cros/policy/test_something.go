// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"go.chromium.org/luci/auth"

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

	s.Log("test")

	a := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{
		Method: auth.ServiceAccountMethod,
	})

	// No token yet, it is is lazily loaded below.
	tok, err := a.currentToken()
	if err != nil {
		s.Fatal("Failed to get current token")
	}

	// The token is minted on first request.
	oauthTok, err := a.GetAccessToken(time.Minute)
	if err != nil {
		s.Fatal("Failed to get token")
	}

	// And we also get an email straight from MintToken call.
	email, err := a.GetEmail()
	if err != nil {
		s.Fatal("Failed to get email")
	}
	s.Log(email)

	return
}
