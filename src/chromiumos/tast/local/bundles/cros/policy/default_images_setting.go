// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultImagesSetting,
		Desc: "Behavior of DefaultImagesSetting policy",
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
		name  string                       // name is the subtest name.
		value *policy.DefaultImagesSetting // value is the policy we test.
	}{
		{
			name:  "Allow all sites to show all images",
			value: &policy.DefaultImagesSetting{Val: 1},
		},
		{
			name:  "Do not allow any site to show images",
			value: &policy.DefaultImagesSetting{Val: 2},
		},
		{
			name:  "unset",
			value: &policy.DefaultImagesSetting{Stat: policy.StatusUnset},
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

			dconn, err := cr.NewConn(ctx, server.URL+"/default_images_setting_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconn.Close()

			var imgwdth int
			if err := dconn.Eval(ctx, `document.getElementById('kittens_id').naturalWidth`, &imgwdth); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			s.Logf("Imagw Width is %q", imgwdth)

			if imgwdth < 640 {
				if param.name == "unset" || param.value.Val == 1 {
					s.Error("Image was blocked")
				}
			} else {
				if param.value.Val == 2 {
					s.Error("Image wasn't blocked")
				}
			}
		})
	}
}
