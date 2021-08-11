// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type eventType string

const (
	policyRequest      eventType = "POLICY_REQUEST"
	success            eventType = "SUCCESS"
	cancelled          eventType = "CANCELED"
	connectivityChange eventType = "CONNECTIVITY_CHANGE"
	sessionStateChange eventType = "SESSION_STATE_CHANGE"
	installationFailed eventType = "INSTALLATION_FAILED"
	unknown            eventType = "EXTENSION_INSTALL_STATUS_UNKNOWN"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionInstallEventLoggingEnabled,
		Desc: "Behavior of ExtensionInstallEventLoggingEnabled policy, checking if all events from the installation of an extension are logged.",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"policy.ExtensionInstallEventLoggingEnabled.username", "policy.ExtensionInstallEventLoggingEnabled.password"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "enrolled",
		Timeout:      chrome.GAIALoginTimeout + 3*time.Minute,
	})
}

func ExtensionInstallEventLoggingEnabled(ctx context.Context, s *testing.State) {
	// The user has the ExtensionInstallEventLoggingEnabled policy set.
	username := s.RequiredVar("policy.ExtensionInstallEventLoggingEnabled.username")
	password := s.RequiredVar("policy.ExtensionInstallEventLoggingEnabled.password")

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ProdPolicy(),
		chrome.KeepState(),
		chrome.ExtraArgs("--enable-features=EncryptedReportingPipeline", "--install-log-fast-upload-for-tests", "--extension-install-event-chrome-log-for-tests"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	const (
		extensionID = "hoppbgdeajkagempifacalpdapphfoai"
		downloadURL = "https://chrome.google.com/webstore/detail/platformkeys-test-extensi/" + extensionID
	)

	extension_page, err := cr.NewConn(ctx, downloadURL)
	if err != nil {
		s.Fatal("Failed to connect to the extension page: ", err)
	}
	defer extension_page.Close()

	ui := uiauto.New(tconn)
	// If the extension is installed, the Installed button will be present which is not clickable.
	installedButton := nodewith.Name("Installed").Role(role.Button).First()
	if err = ui.WaitUntilExists(installedButton)(ctx); err != nil {
		s.Fatal("Finding Installed button failed: ", err)
	}

	// Check if required sequence appears in chrome log.
	s.Log("Wait for events logged")
	if err := waitForLoggedEvents(ctx, cr, extensionID); err != nil {
		s.Fatal("Required events not logged: ", err)
	}
}

// statusCodeToEvent converts status code to eventType. Should be in sync with device_management_backend.proto in chrome.
func statusCodeToEvent(code string) eventType {
	statusCodeMap := map[string]eventType{
		"1": policyRequest,
		"2": success,
		"3": cancelled,
		"4": connectivityChange,
		"5": sessionStateChange,
		"6": installationFailed,
	}
	event, ok := statusCodeMap[code]
	if !ok {
		event = unknown
	}
	return event
}

// readLoggedEvents reads logged events from /var/log/chrome/chrome file.
func readLoggedEvents(extensionId string) ([]eventType, error) {
	logContent, err := ioutil.ReadFile(syslog.ChromeLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read "+syslog.ChromeLogFile)
	}

	r := regexp.MustCompile(fmt.Sprintf(`Add extension install event: %s, (.*)`, extensionId))
	matches := r.FindAllStringSubmatch(string(logContent), -1)
	if matches == nil {
		return nil, nil
	}

	var events []eventType
	for _, m := range matches {
		events = append(events, statusCodeToEvent(m[1]))
	}
	return events, nil
}

// waitForLoggedEvents waits for desired sequence to appear in chrome log.
func waitForLoggedEvents(ctx context.Context, cr *chrome.Chrome, extensionId string) error {
	var expectedEvents = []eventType{sessionStateChange, policyRequest, success}

	ctx, st := timing.Start(ctx, "wait_logged_events")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		loggedEvents, err := readLoggedEvents(extensionId)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read chrome log"))
		}

		eventsMap := make(map[eventType]bool)
		for _, e := range loggedEvents {
			eventsMap[e] = true
		}

		for _, expected := range expectedEvents {
			if !eventsMap[expected] {
				var strEvents []string
				for _, e := range loggedEvents {
					strEvents = append(strEvents, string(e))
				}
				return errors.New("incomplete sequence: " + strings.Join(strEvents[:], ","))
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}
