// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"path"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SysInfoPII,
		Desc:         "Verify that known-sensitive data doesn't show up in feedback reports",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// saveLog attempts to save the given log to the test's output directory
func saveLog(outDir, key, value string) error {
	return ioutil.WriteFile(path.Join(outDir, key+".log"), []byte(value), 0664)
}

// systemInformation corresponds to the "SystemInformation" defined in autotest_private.idl.
type systemInformation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SysInfoPII loads an external website URL to verify that the logs that the
// feedback system sends do not contain sensitive and hard-to-redact
// information, such as tab names.
func SysInfoPII(ctx context.Context, s *testing.State) {
	const (
		// Don't use a localhost URL or local file:// url because those
		// are not as sensitive and redaction pipelines treat them
		// differently.
		sensitiveURL    = "https://www.google.com/search?q=qwertyuiopasdfghjkl+sensitive"
		sensitiveURLEnd = "www.google.com/search?q=qwertyuiopasdfghjkl+sensitive"
		searchQuery     = "qwertyuiopasdfghjkl sensitive"
	)

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, sensitiveURL)
	if err != nil {
		s.Fatal("Failed to establish a chrome renderer connection: ", err)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not create test API conn: ", err)
	}
	var ret []*systemInformation
	if err := tconn.Eval(ctx, "tast.promisify(chrome.feedbackPrivate.getSystemInformation)()", &ret); err != nil {
		s.Fatal("Could not call getSystemInformation: ", err)
	}

	var title string
	if err := conn.Eval(ctx, "document.title", &title); err != nil {
		s.Fatal("Failed to get the tab title: ", err)
	}

	for _, info := range ret {
		if info.Key != "mem_usage_with_title" {
			// mem_usage_with_title is only included if the user
			// explicitly opts to send tab titles, so it's
			// acceptable for it to contain titles or possibly URLs.
			badContents := []struct {
				content string
				desc    string
			}{
				{title, "tabTitle"},
				{searchQuery, "searchQuery"},
				{sensitiveURLEnd, "URL"}}
			for _, entry := range badContents {
				if strings.Contains(info.Value, entry.content) {
					s.Errorf("Log %q unexpectedly contained %s", info.Key, entry.desc)
					if err := saveLog(s.OutDir(), info.Key, info.Value); err != nil {
						s.Error("Also, failed to save log contents: ", err)
					}
					if err := saveLog(s.OutDir(), entry.desc, entry.content); err != nil {
						s.Errorf("Also, failed to save %s: %v", entry.desc, err)
					}
				}
			}
		}
		// Trim "@gmail.com" to look for both username and full email
		// (The PII redaction should eliminate the email, so looking
		// for the full email would not be as useful as looking for the
		// username.)
		user := strings.TrimSuffix(chrome.DefaultUser, "@gmail.com")
		if strings.Contains(info.Value, user) {
			// DO NOT actually log the username here -- if we do, and the test fails,
			// then the username will be in the syslog and all future runs of the test
			// on that device will also fail.
			s.Errorf("Log %q unexpectedly contained username", info.Key)
			if err := saveLog(s.OutDir(), info.Key, info.Value); err != nil {
				s.Error("Also, failed to save log contents: ", err)
			}
		}
	}
}
