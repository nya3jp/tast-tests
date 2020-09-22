// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultImagesSetting,
		Desc: "Behavior of DefaultImagesSetting policy, check whether an image is displayed in a website",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"default_images_setting_index.html", "default_images_setting_index_img.jpg"},
	})
}

func DefaultImagesSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name        string                       // name is the subtest name.
		wantAllowed bool                         // wantAllowed is the allow state of images.
		value       *policy.DefaultImagesSetting // value is the policy we test.
	}{
		{
			name:        "allow",
			wantAllowed: true,
			value:       &policy.DefaultImagesSetting{Val: 1},
		},
		{
			name:        "disable",
			wantAllowed: false,
			value:       &policy.DefaultImagesSetting{Val: 2},
		},
		{
			name:        "unset",
			wantAllowed: true,
			value:       &policy.DefaultImagesSetting{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, server.URL+"/default_images_setting_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Wait until the page is loaded.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var pageReady bool
				if err := conn.Eval(ctx, `pageReady`, &pageReady); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to execute js expression"))
				}
				if pageReady == false {
					return errors.New("page isn't loaded")
				}
				return nil
			}, &testing.PollOptions{
				Timeout: 10 * time.Second,
			}); err != nil {
				s.Fatal("Failed to load the page: ", err)
			}

			var imgwdth int
			if err := conn.Eval(ctx, `document.getElementById('img_id').naturalWidth`, &imgwdth); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// If imgwdth is > 0 the image was allowed.
			if allowed := imgwdth > 0; allowed != param.wantAllowed {
				s.Errorf("Unexpected Allow behavior: got %t; want %t", allowed, param.wantAllowed)
			}
		})
	}
}
