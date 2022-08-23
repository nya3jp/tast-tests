// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

// piiTestParam contains all the data needed to run a single test iteration.
type piiTestParam struct {
	testType    int
	browserType browser.Type
}

// Define test types.
const (
	localFileTest int = iota
	thirdPartySiteTest
)

const (
	testPageFilename = "sys_info_pii_test_page.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SysInfoPII,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify that known-sensitive data doesn't show up in feedback reports",
		Contacts:     []string{"xiangdongkong@google.com", "cros-feedback-app@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "local_on_ash",
			Val: piiTestParam{
				testType:    localFileTest,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{testPageFilename},
			ExtraAttr: []string{"informational"},
			Pre:       chrome.LoggedIn(),
		}, {
			Name: "third_party_site_on_ash",
			Val: piiTestParam{
				testType:    thirdPartySiteTest,
				browserType: browser.TypeAsh,
			},
			ExtraAttr: []string{"informational"},
			Pre:       chrome.LoggedIn(),
		}, {
			Name: "local_on_lacros",
			Val: piiTestParam{
				testType:    localFileTest,
				browserType: browser.TypeLacros,
			},
			ExtraData:         []string{testPageFilename},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
		}, {
			Name: "third_party_site_on_lacros",
			Val: piiTestParam{
				testType:    thirdPartySiteTest,
				browserType: browser.TypeLacros,
			},
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
		}},
	})
}

// saveLog attempts to save the given log to the test's output directory.
func saveLog(outDir, key, value string) error {
	return ioutil.WriteFile(path.Join(outDir, key+".log"), []byte(value), 0664)
}

// systemInformation corresponds to the "SystemInformation" defined in autotest_private.idl.
type systemInformation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type contentsDescriptor struct {
	content string
	desc    string
}

// SysInfoPII loads an external website URL to verify that the logs that the
// feedback system sends do not contain sensitive and hard-to-redact
// information, such as tab names.
func SysInfoPII(ctx context.Context, s *testing.State) {
	const (
		localPageTitle    = "feedback.SysInfoPII test page title"
		localPageContents = "feedback.SysInfoPII test page contents"
	)
	testType := s.Param().(piiTestParam).testType
	browserType := s.Param().(piiTestParam).browserType

	sensitiveURL := ""
	sensitiveURLWithoutScheme := ""
	if testType == localFileTest {
		dataFile := s.DataPath(testPageFilename)
		targetPath := path.Join("/tmp", testPageFilename)
		if err := fsutil.CopyFile(dataFile, targetPath); err != nil {
			s.Fatal("Failed to put dataFile in tmp dir: ", err)
		}
		if err := os.Chmod(targetPath, 0666); err != nil {
			s.Fatal("Failed to make dataFile readable: ", err)
		}
		sensitiveURL = "file://" + targetPath
		sensitiveURLWithoutScheme = dataFile
	} else {
		// Arbitrary third-party website, which shouldn't be logged.
		// We use a third-party website to reduce the risk of false positives:
		// some utilities hard-code "www.google.com" and log that string,
		// which is acceptable (as it's not in response to any user actions).
		sensitiveURL = "https://www.facebook.com"
		sensitiveURLWithoutScheme = "www.facebook.com"
	}

	var bTconn *chrome.TestConn
	var conn *chrome.Conn

	if browserType == browser.TypeAsh {
		cr := s.PreValue().(*chrome.Chrome)
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Could not create test API conn: ", err)
		}

		conn, err = cr.NewConn(ctx, sensitiveURL)
		if err != nil {
			s.Fatal("Failed to establish a chrome renderer connection: ", err)
		}
		defer conn.Close()

		bTconn = tconn
	} else {
		tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Could not create test API conn: ", err)
		}

		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch Lacros: ", err)
		}
		defer l.Close(ctx)

		// Get all pages.
		ts, err := l.FindTargets(ctx, chrome.MatchAllPages())
		if err != nil {
			s.Fatal("Failed to find pages: ", err)
		}

		if len(ts) != 1 {
			s.Fatal("Expected only one page target, got ", ts)
		}

		conn, err = l.NewConnForTarget(ctx, chrome.MatchTargetID(ts[0].TargetID))
		if err := conn.Navigate(ctx, sensitiveURL); err != nil {
			s.Fatal("Failed to navigate to sensitiveURL: ", err)
		}

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	}

	s.Log("Calling getSystemInformation")
	var ret []*systemInformation
	if err := bTconn.Eval(ctx, "tast.promisify(chrome.feedbackPrivate.getSystemInformation)()", &ret); err != nil {
		s.Fatal("Could not call getSystemInformation: ", err)
	}

	var actualTitle string
	if err := conn.Eval(ctx, "document.title", &actualTitle); err != nil {
		s.Fatal("Failed to get the tab title: ", err)
	}
	// Check that page actually loaded as expected.
	if testType == localFileTest && actualTitle != localPageTitle {
		// Don't log the expected page title so we don't make later
		// runs of this test fail.
		s.Error("Unexpected title: ", actualTitle)
	}

	for _, info := range ret {
		if info.Key != "mem_usage_with_title" {
			// mem_usage_with_title is only included if the user
			// explicitly opts to send tab titles, so it's
			// acceptable for it to contain titles or possibly URLs.
			badContents := []contentsDescriptor{
				{actualTitle, "tabTitle"},
				{sensitiveURL, "URL"},
				{sensitiveURLWithoutScheme, "URLWithoutScheme"},
			}
			if testType == localFileTest {
				badContents = append(
					badContents,
					contentsDescriptor{localPageContents, "pageContents"},
				)
			}
			for _, entry := range badContents {
				if strings.Contains(info.Value, entry.content) {
					// Don't log actual contents so that we don't make later runs
					// of this test fail.
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
		// Trim "@gmail.com" to look for both username and full email.
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
