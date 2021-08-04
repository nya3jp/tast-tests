// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AudioCaptureAllowed,
		Desc: "Checking if audio capture is allowed on websites or not, depending on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"}, // Remove informational once crbug/1236676 is fixed.
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"audio_capture_allowed.html"},
	})
}

// AudioCaptureAllowed tests the AudioCaptureAllowed policy.
func AudioCaptureAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name      string
		wantAsk   bool                        // wantAsk states whether a dialog to ask for permission should appear or not.
		wantAllow bool                        // wantAllow states whether access to the microphone is allowed.
		value     *policy.AudioCaptureAllowed // value is the value of the policy.
	}{
		{
			name:      "unset",
			wantAsk:   true,
			wantAllow: true,
			value:     &policy.AudioCaptureAllowed{Stat: policy.StatusUnset},
		},
		{
			name:      "allow",
			wantAsk:   false,
			wantAllow: true,
			value:     &policy.AudioCaptureAllowed{Val: true},
		},
		{
			name:      "deny",
			wantAsk:   false,
			wantAllow: false,
			value:     &policy.AudioCaptureAllowed{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the website.
			conn, err := cr.NewConn(ctx, server.URL+"/audio_capture_allowed.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)

			// Start a go routine before requesting the microphone as the Eval()
			// function will block when a dialog to ask for permission appears.
			// The routine will then click the allow button in the dialog.
			ch := make(chan error, 1)
			go func() {
				allowButton := nodewith.Name("Allow").Role(role.Button)

				if err = ui.WaitUntilExists(allowButton)(ctx); err != nil {
					if param.wantAsk {
						s.Error("Allow button not found: ", err)
					}
					ch <- nil
					return
				}

				if !param.wantAsk {
					s.Error("Unexpected dialog to ask for microphone permission found")
				}

				// TODO(crbug.com/1197511): investigate why this is needed.
				// Wait for a second before clicking the allow button as the click
				// won't be registered otherwise.
				testing.Sleep(ctx, time.Second)

				if err := ui.LeftClickUntil(allowButton, ui.Gone(allowButton))(ctx); err != nil {
					s.Fatal("Failed to click the Allow button: ", err)
				}

				ch <- nil
			}()

			// Try to access the microphone.
			var allowed bool
			if err := conn.Eval(ctx, "requestMicrophone()", &allowed); err != nil {
				s.Fatal("Failed to request microphone: ", err)
			}
			if err := <-ch; err != nil {
				s.Fatal("Failed to execute the routine to click the allow button: ", err)
			}

			// Check if we were allowed to use the microphone.
			if allowed != param.wantAllow {
				s.Errorf("Unexpected access to microphone received: got %v; want %v", allowed, param.wantAllow)
			}
		})
	}
}
