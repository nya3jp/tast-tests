// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCInstallLogging,
		Desc:         "Checks that log is uploaded after forced app installation in ARC",
		Contacts:     []string{"yixie@chromium.org", "arc-eng-muc@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      13 * time.Minute,
		Vars:         []string{"enterprise.ARCInstallLogging.user", "enterprise.ARCInstallLogging.password"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

type eventType string

const (
	serverRequest          eventType = "SERVER_REQUEST"
	cloudDpcRequest        eventType = "CLOUDDPC_REQUEST"
	cloudDpsRequest        eventType = "CLOUDDPS_REQUEST"
	cloudDpsResponse       eventType = "CLOUDDPS_RESPONSE"
	phoneskyLog            eventType = "PHONESKY_LOG"
	success                eventType = "SUCCESS"
	cancelled              eventType = "CANCELED"
	connectivityChange     eventType = "CONNECTIVITY_CHANGE"
	sessionStateChange     eventType = "SESSION_STATE_CHANGE"
	installationStarted    eventType = "INSTALLATION_STARTED"
	installationFinished   eventType = "INSTALLATION_FINISHED"
	installationFailed     eventType = "INSTALLATION_FAILED"
	directInstall          eventType = "DIRECT_INSTALL"
	cloudDpcMainLoopFailed eventType = "CLOUDDPC_MAIN_LOOP_FAILED"
	unknown                eventType = "UNKNOWN"
)

// ARCInstallLogging runs the install event logging test:
// - login with managed account,
// - check that ARC is launched by user policy,
// - check ArcEnabled is true and test app is set to force-installed by policy,
// - check that the test app is installed,
// - upload a log from the test app to test server for comparison,
// - check that app installation log is uploaded from Chrome.
func ARCInstallLogging(ctx context.Context, s *testing.State) {
	const testPackage = "com.managedchrome.arcloggingtest"

	user := s.RequiredVar("enterprise.ARCInstallLogging.user")
	password := s.RequiredVar("enterprise.ARCInstallLogging.password")

	// Login to Chrome and allow to launch ARC if allowed by user policy. Flag --install-log-fast-upload-for-tests reduces delay of uploading chrome log.
	// Flag --arc-install-event-chrome-log-for-tests logs ARC install events to chrome log.
	args := append(arc.DisableSyncFlags(), "--install-log-fast-upload-for-tests", "--arc-install-event-chrome-log-for-tests")
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: user, Pass: password}),
		chrome.ARCSupported(),
		chrome.ProdPolicy(),
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Ensure that ARC is launched.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Ensure chrome://policy shows correct ArcEnabled and ArcPolicy values.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
		s.Fatal("Failed to verify ArcEnabled: ", err)
	}

	if err := arc.VerifyArcPolicyForceInstalled(ctx, tconn, []string{testPackage}); err != nil {
		s.Fatal("Failed to verify force-installed apps in ArcPolicy: ", err)
	}

	// Ensure that test app is force-installed by ARC policy.
	if err := a.WaitForPackages(ctx, []string{testPackage}); err != nil {
		s.Fatal("Failed to force install packages: ", err)
	}

	// Launch test app.
	s.Log("Starting log test app")
	act, err := arc.NewActivity(a, testPackage, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed starting test app: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Select current account in the account selection UI.
	s.Log("Selecting android account")
	accountSelection := d.Object(ui.ResourceIDMatches("(android:id/text1)"))
	if err := accountSelection.WaitForExists(ctx, time.Second); err != nil {
		s.Fatal("Failed to find account selection: ", err)
	}
	if err := accountSelection.Click(ctx); err != nil {
		s.Fatal("Failed to select account: ", err)
	}

	doneButton := d.Object(ui.ResourceIDMatches("(android:id/button1)"))
	if err := doneButton.WaitForExists(ctx, time.Second); err != nil {
		s.Fatal("Failed to find done button: ", err)
	}
	if err := doneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click done: ", err)
	}

	// Start uploading logs to test server by clicking "UPLOAD" button.
	s.Log("Starting log upload")
	uploadButtonID := fmt.Sprintf("(%s:id/uploadButton)", testPackage)
	uploadButton := d.Object(ui.ResourceIDMatches(uploadButtonID))
	if err := uploadButton.WaitForExists(ctx, time.Second); err != nil {
		s.Fatal("Failed to find upload button: ", err)
	}
	if err := uploadButton.Click(ctx); err != nil {
		s.Fatal("Failed to click upload button: ", err)
	}

	// Wait for status label in the test app to show "SUCCESS".
	s.Log("Checking for app log upload status")
	uploadStatusLabelID := fmt.Sprintf("(%s:id/uploadStatusLabel)", testPackage)
	if err := waitForAppLogUpload(ctx, d, uploadStatusLabelID); err != nil {
		s.Fatal("Failed to upload app log: ", err)
	}

	// Check if required sequence appears in chrome log.
	if err := waitForLoggedEvents(ctx, cr, testPackage); err != nil {
		s.Fatal("Required events not logged: ", err)
	}

	// Wait for chrome to upload logs to enterprise management server.
	s.Log("Checking for chrome log upload status")
	if err := waitForChromeLogUpload(ctx, cr, testPackage); err != nil {
		s.Fatal("Chrome log not uploaded: ", err)
	}
}

// waitForAppLogUpload waits for test app to upload logs to test server. Status label will show "SUCCESS" after successful upload.
func waitForAppLogUpload(ctx context.Context, d *ui.Device, uploadStatusLabelID string) error {
	ctx, st := timing.Start(ctx, "wait_app_log_upload")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		text, err := d.Object(ui.ResourceIDMatches(uploadStatusLabelID)).GetText(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get status"))
		}

		if text == "UPLOADING" {
			return errors.New("Log upload not finished yet")
		} else if text == "SUCCESS" {
			return nil
		}
		return testing.PollBreak(errors.Wrap(err, "unknown log upload status: "+text))
	}, nil)
}

// statusCodeToEvent converts status code to eventType. Should be in sync with device_management_backend.proto in chrome.
func statusCodeToEvent(code string) eventType {
	statusCodeMap := map[string]eventType{
		"1":  serverRequest,
		"2":  cloudDpcRequest,
		"3":  cloudDpsRequest,
		"4":  cloudDpsResponse,
		"5":  phoneskyLog,
		"6":  success,
		"7":  cancelled,
		"8":  connectivityChange,
		"9":  sessionStateChange,
		"10": installationStarted,
		"11": installationFinished,
		"12": installationFailed,
		"13": directInstall,
		"14": cloudDpcMainLoopFailed,
	}
	event, ok := statusCodeMap[code]
	if !ok {
		event = unknown
	}
	return event
}

// readLoggedEvents reads logged events from /var/log/chrome/chrome file.
func readLoggedEvents(packageName string) ([]eventType, error) {
	logContent, err := ioutil.ReadFile(syslog.ChromeLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read "+syslog.ChromeLogFile)
	}

	r := regexp.MustCompile(fmt.Sprintf(`Add ARC install event: %s, (.*)`, packageName))
	matches := r.FindAllStringSubmatch(string(logContent), -1)
	if matches == nil {
		return nil, errors.New("no event logged yet")
	}

	var events []eventType
	for _, m := range matches {
		events = append(events, statusCodeToEvent(m[1]))
	}
	return events, nil
}

// waitForLoggedEvents waits for desired sequence to appear in chrome log.
func waitForLoggedEvents(ctx context.Context, cr *chrome.Chrome, packageName string) error {
	var expectedEvents = []eventType{serverRequest, installationStarted, installationFinished, success}

	ctx, st := timing.Start(ctx, "wait_logged_events")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		loggedEvents, err := readLoggedEvents(packageName)
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

// readChromeLogFile reads content of serialized installation events log file.
func readChromeLogFile(ctx context.Context, cr *chrome.Chrome) ([]byte, error) {
	const logFilePath = "/app_push_install_log"

	// Cryptohome dir for the current user.
	rootCryptDir, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	return ioutil.ReadFile(filepath.Join(rootCryptDir, logFilePath))
}

// waitForChromeLogUpload waits for chrome to upload logs to the server. After the logs are successfully uploaded contents of the log file will be cleared.
func waitForChromeLogUpload(ctx context.Context, cr *chrome.Chrome, packageName string) error {
	ctx, st := timing.Start(ctx, "wait_chrome_log_upload")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		chromeLog, err := readChromeLogFile(ctx, cr)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read app_push_install_log"))
		}

		if bytes.Contains(chromeLog, []byte(packageName)) {
			return errors.New("Chrome log not uploaded yet")
		}

		return nil
	}, nil)
}
