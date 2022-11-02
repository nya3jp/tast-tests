// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultSensorsSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that the DefaultSensor policy blocks or allows access to sensors",
		Contacts: []string{
			"fahadmansoor@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func DefaultSensorsSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	sensors := []string{"accelerometer", "gyroscope", "magnetometer"}

	for _, tc := range []struct {
		name  string
		value *policy.DefaultSensorsSetting
		allow bool
	}{
		{
			name:  "allow",
			value: &policy.DefaultSensorsSetting{Val: 1},
			allow: true,
		},
		{
			name:  "block",
			value: &policy.DefaultSensorsSetting{Val: 2},
			allow: false,
		},
		{
			name:  "unset",
			value: &policy.DefaultSensorsSetting{Stat: policy.StatusUnset},
			allow: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}
			// Open the browser
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			if tc.name == "unset" {
				return
			}

			conn, err := br.NewConn(ctx, "chrome://newtab")
			if err != nil {
				s.Fatal("Failed to connect to Chrome: ", err)
			}
			defer conn.Close()
			for _, sensorName := range sensors {
				s.Logf(" Testing sensor: %s", sensorName)
				result := ""
				err := conn.Call(ctx, &result, `async (sensor) => {
				const result = await navigator.permissions.query({name: sensor});
				return result.state;
			  }`, sensorName)
				if err != nil {
					s.Fatal("Failed to get result for the permission query: ", err)
				}

				allowed := result == "granted"
				if tc.allow != allowed {
					s.Fatalf("Expected %t got %t for sensor %s", tc.allow, allowed, sensorName)
				}
			}
		})
	}
}
