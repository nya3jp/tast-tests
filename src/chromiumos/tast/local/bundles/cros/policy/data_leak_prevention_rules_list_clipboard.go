// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboard,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with screenshot blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
	})
}

type Notification struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Progress int    `json:"progress"`
}

// A DUTPolicy represents the information about a single policy as returned by
// the getAllEnterprisePolicies API.
// Example JSON: {"scope": "user", "level": "mandatory", "source": "cloud",
//                "value": false, "error": "This policy has been deprecated."}
type DUTPolicy struct {
	Level     string
	Scope     string
	Source    string
	Status    string
	ValueJSON json.RawMessage `json:"value"`
	Error     string
}

// DUTPolicies represents the format returned from the getAllEnterprisePolicies API.
// Each member map matches a string policy name (as shown in chrome://policy,
// not a device policy field name) to a DUTPolicy struct of information on that
// policy.
type DUTPolicies struct {
	Chrome      map[string]*DUTPolicy `json:"chromePolicies"`
	DeviceLocal map[string]*DUTPolicy `json:"deviceLocalAccountPolicies"`
	Extension   map[string]*DUTPolicy `json:"extensionPolicies"`
}

// // prepareCopyInChrome sets up a copy operation with Chrome as the source
// // clipboard.
// func prepareCopyInChrome(tconn *chrome.TestConn, format, data string) copyFunc {
// 	return func(ctx context.Context) error {
// 		return tconn.Call(ctx, nil, `
// 		  (format, data) => {
// 		    document.addEventListener('copy', (event) => {
// 		      event.clipboardData.setData(format, data);
// 		      event.preventDefault();
// 		    }, {once: true});
// 		    if (!document.execCommand('copy')) {
// 		      throw new Error('Failed to execute copy');
// 		    }
// 		  }`, format, data,
// 		)
// 	}
// }

// // preparePasteInChrome sets up a paste operation with Chrome as the
// // destination clipboard.
// func preparePasteInChrome(tconn *chrome.TestConn, format string) pasteFunc {
// 	return func(ctx context.Context) (string, error) {
// 		var result string
// 		if err := tconn.Call(ctx, &result, `
// 		  (format) => {
// 		    let result;
// 		    document.addEventListener('paste', (event) => {
// 		      result = event.clipboardData.getData(format);
// 		    }, {once: true});
// 		    if (!document.execCommand('paste')) {
// 			    throw new Error('Failed to execute paste');
// 		    }
// 		    return result;
// 		  }`, format,
// 		); err != nil {
// 			return "", err
// 		}
// 		return result, nil
// 	}
// }

func DataLeakPreventionRulesListClipboard(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with screenshots blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable Screenshot in confidential content",
				Description: "User should not be able to take screen of confidential content",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"salesforce.com",
						"google.com",
						"example.com",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"google.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// Update the policy blob.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policyDLP)
	if err := fakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	// Policies are only updated after Chrome startup.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fakeDMS.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	captureNotAllowed := "Can't capture confidential content"

	for _, param := range []struct {
		name             string // Name
		wantNotification string // Want Notification
		wantAllowed      bool   // Want Allowed
		url              string // Url String
	}{
		// {
		// 	name:             "Salesforce",
		// 	wantAllowed:      false,
		// 	wantNotification: captureNotAllowed,
		// 	url:              "https://www.salesforce.com/",
		// },
		// {
		// 	name:             "Google",
		// 	wantAllowed:      false,
		// 	wantNotification: captureNotAllowed,
		// 	url:              "https://www.google.com/",
		// },
		{
			name:             "example",
			wantAllowed:      false,
			wantNotification: captureNotAllowed,
			url:              "https://www.example.com/",
		},
		// {
		// 	name:             "Chromium",
		// 	wantAllowed:      true,
		// 	wantNotification: "Screenshot taken",
		// 	url:              "https://www.chromium.org/",
		// },
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			if _, err = cr.NewConn(ctx, param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if _, err = cr.NewConn(ctx, "https://www.google.com/"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+V"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}
			// Please check kScreenshotMinimumIntervalInMS constant in
			// ui/snapshot/screenshot_grabber.cc
			if err := testing.Sleep(ctx, time.Second*5); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var notifications []Notification
				if err := tconn.Call(ctx,
					&notifications,
					`tast.promisify(chrome.autotestPrivate.getVisibleNotifications)`,
				); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get visible notifications"))
				}

				found := false
				for _, n := range notifications {
					if n.Title == "notificationTitle" {
						found = true
						break
					}
				}
				if !found {
					s.Fatal("%q %q in %q", "notificationTitle", "notificationBody", notifications)
				}
				s.Fatal("Did not find expected notification: ")
				return nil
			}, &testing.PollOptions{
				Timeout: 10 * time.Second,
			}); err != nil {
				s.Fatal("Did not find expected notification: ", err)
			}

		})
	}
}

// Notifications returns an array of notifications in Chrome.
// tconn must be the connection returned by chrome.TestAPIConn().
//
// Note: it uses an autotestPrivate API with the misleading name
// getVisibleNotifications under the hood.
func Notifications(ctx context.Context, tconn *chrome.TestConn) ([]*Notification, error) {
	var ret []*Notification
	if err := tconn.Call(ctx, &ret, "tast.promisify(chrome.autotestPrivate.getVisibleNotifications)"); err != nil {
		return nil, errors.Wrap(err, "failed to call getVisibleNotifications")
	}
	return ret, nil
}
