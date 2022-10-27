// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const arcInstallLoggingTestTimeout = 13 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCInstallLogging,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that log is uploaded after forced app installation in ARC",
		Contacts:     []string{"mhasank@chromium.org", "arc-commercial@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      arcInstallLoggingTestTimeout,
		VarDeps:      []string{tape.ServiceAccountVar, arcent.LoginPoolVar},
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
	serverRequest           eventType = "SERVER_REQUEST"
	cloudDpcRequest         eventType = "CLOUDDPC_REQUEST"
	cloudDpsRequest         eventType = "CLOUDDPS_REQUEST"
	cloudDpsResponse        eventType = "CLOUDDPS_RESPONSE"
	phoneskyLog             eventType = "PHONESKY_LOG"
	success                 eventType = "SUCCESS"
	cancelled               eventType = "CANCELED"
	connectivityChange      eventType = "CONNECTIVITY_CHANGE"
	sessionStateChange      eventType = "SESSION_STATE_CHANGE"
	installationStarted     eventType = "INSTALLATION_STARTED"
	installationFinished    eventType = "INSTALLATION_FINISHED"
	installationFailed      eventType = "INSTALLATION_FAILED"
	directInstall           eventType = "DIRECT_INSTALL"
	cloudDpcMainLoopFailed  eventType = "CLOUDDPC_MAIN_LOOP_FAILED"
	playstoreLocalPolicySet eventType = "PLAYSTORE_LOCAL_POLICY_SET"
	unknown                 eventType = "UNKNOWN"
)

// ARCInstallLogging runs the install event logging test:
// - login with managed account from an account pool,
// - check that ARC is launched by user policy,
// - check ArcEnabled is true and test app is set to force-installed by policy,
// - check that the test app is installed,
// - verify all expected install events are logged.
func ARCInstallLogging(ctx context.Context, s *testing.State) {
	const (
		testPackage = "com.google.android.calculator"
		poolID      = "arc_logging_test"
		maxAttempts = 2
	)

	attempts := 1

	// Indicates a failure in the core feature under test so the polling should stop.
	exit := func(desc string, err error) error {
		s.Fatalf("Failed to %s: %v", desc, err)
		return nil
	}

	// Indicates that the error is retryable and unrelated to core feature under test.
	retry := func(desc string, err error) error {
		if attempts < maxAttempts {
			attempts++
			err = errors.Wrap(err, "failed to "+desc)
			s.Logf("%s. Retrying", err)
			return err
		}
		return exit(desc, err)
	}

	creds, err := chrome.PickRandomCreds(s.RequiredVar(arcent.LoginPoolVar))
	if err != nil {
		exit("get login creds", err)
	}
	login := chrome.GAIALogin(creds)

	packages := []string{testPackage}
	fdms, err := setupPolicyServerWithArcAppsAndEnableLogging(ctx, s.OutDir(), creds.User, packages)
	if err != nil {
		exit("setup fake policy server", err)
	}
	defer fdms.Stop(ctx)

	// Login to Chrome and allow to launch ARC if allowed by user policy.
	// Flag --arc-install-event-chrome-log-for-tests logs ARC install events to chrome log.
	args := append(arc.DisableSyncFlags(), "--arc-install-event-chrome-log-for-tests")
	if err := testing.Poll(ctx, func(ctx context.Context) (retErr error) {
		cr, err := chrome.New(
			ctx,
			login,
			chrome.ARCSupported(),
			chrome.UnRestrictARCCPU(),
			chrome.DMSPolicy(fdms.URL),
			chrome.ExtraArgs(args...))
		if err != nil {
			return retry("connect to Chrome", err)
		}
		defer cr.Close(ctx)

		// Ensure that ARC is launched.
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			return retry("start ARC by policy", err)
		}
		defer a.Close(ctx)

		_, err = cr.TestAPIConn(ctx)
		if err != nil {
			return retry("create test API connection", err)
		}

		// Ensure that test app is force-installed by ARC policy.
		if err := a.WaitForPackages(ctx, []string{testPackage}); err != nil {
			return retry("force install packages: ", err)
		}

		// Check if required sequence appears in chrome log.
		if err := waitForLoggedEvents(ctx, cr, testPackage); err != nil {
			return exit("log required events: ", err)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Install logging flow failed: ", err)
	}
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
		"15": playstoreLocalPolicySet,
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

func setupPolicyServerWithArcAppsAndEnableLogging(ctx context.Context, outDir, policyUser string, packages []string) (fdms *fakedms.FakeDMS, retErr error) {
	arcPolicy := arcent.CreateArcPolicyWithApps(packages, arcent.InstallTypeForceInstalled)
	arcEnabledPolicy := &policy.ArcEnabled{Val: true}
	arcInstallLoggingEnabledPolicy := &policy.ArcAppInstallEventLoggingEnabled{Val: true}
	policies := []policy.Policy{arcEnabledPolicy, arcPolicy, arcInstallLoggingEnabledPolicy}

	return policyutil.SetUpFakePolicyServer(ctx, outDir, policyUser, policies)
}
