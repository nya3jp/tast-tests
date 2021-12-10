// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebauthnU2fMode,
		Desc: "Checks that WebAuthn under u2f mode succeeds in different configurations",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:firmware", "firmware_cr50"},
		SoftwareDeps: []string{"chrome", "gsc"},
		Vars:         []string{"servo"},
	})
}

type userVerification int
type authenticatorType int

const (
	discouraged userVerification = iota
	preferred
	required
)

const (
	unspecified authenticatorType = iota
	crossPlatform
	platform
)

type webAuthnConfig struct {
	userVerification
	authenticatorType
	// Whether there would be an auth dialog that we need to wait for.
	hasDialog    bool
	authCallback func() error
}

func WebauthnU2fMode(ctx context.Context, s *testing.State) {
	// We need truly random values for username strings so that different test runs don'taffect each other.
	rand.Seed(time.Now().UnixNano())

	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	// Try to get the system into a consistent state, since it seems like having
	// an already-mounted user dir can cause problems: https://crbug.com/963084
	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	srvo, err := servo.NewDirect(ctx, s.RequiredVar("servo"))
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer srvo.Close(ctx)

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Set u2f mode to enable power button press authentication.
	setU2fdFlags(ctx, helper, true, false)
	// Clean up the flags in u2fd after the tests finished.
	defer setU2fdFlags(ctx, helper, false, false)

	const (
		username   = fixtures.Username
		password   = fixtures.Password
		PIN        = "123456"
		autosubmit = true
	)

	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: username, Pass: password}),
		// Enable device event log in Chrome logs for validation.
		chrome.ExtraArgs("--vmodule=device_event_log*=1"),
		chrome.DMSPolicy(fdms.URL)}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)

	pinPolicies := []policy.Policy{
		&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
		&policy.PinUnlockAutosubmitEnabled{Val: true}}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, pinPolicies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	tconn, err := util.SetUpUserPIN(ctx, cr, PIN, password, autosubmit)
	if err != nil {
		s.Fatal("Failed to set up PIN: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Could not get Chrome log reader: ", err)
	}
	defer logReader.Close()

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	pinAuthCallback := func() error {
		// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
		if err := keyboard.Type(ctx, PIN); err != nil {
			return errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
		}
		return nil
	}
	powerButtonAuthCallback := func() error {
		// Press power button using servo.
		if err := srvo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "failed to press the power key")
		}
		return nil
	}

	for _, tc := range []struct {
		name string
		cfg  webAuthnConfig
	}{
		{
			name: "discouraged_unspecified",
			cfg: webAuthnConfig{
				userVerification:  discouraged,
				authenticatorType: unspecified,
				hasDialog:         false,
				authCallback:      powerButtonAuthCallback,
			},
		},
		{
			name: "discouraged_cross_plaform",
			cfg: webAuthnConfig{
				userVerification:  discouraged,
				authenticatorType: crossPlatform,
				hasDialog:         false,
				authCallback:      powerButtonAuthCallback,
			},
		},
		{
			name: "discouraged_platform",
			cfg: webAuthnConfig{
				userVerification:  discouraged,
				authenticatorType: platform,
				hasDialog:         false,
				authCallback:      powerButtonAuthCallback,
			},
		},
		{
			name: "preferred_unspecified",
			cfg: webAuthnConfig{
				userVerification:  preferred,
				authenticatorType: unspecified,
				hasDialog:         true,
				authCallback:      pinAuthCallback,
			},
		},
		{
			name: "preferred_cross_plaform",
			cfg: webAuthnConfig{
				userVerification:  preferred,
				authenticatorType: crossPlatform,
				hasDialog:         false,
				authCallback:      powerButtonAuthCallback,
			},
		},
		{
			name: "preferred_platform",
			cfg: webAuthnConfig{
				userVerification:  preferred,
				authenticatorType: platform,
				hasDialog:         true,
				authCallback:      pinAuthCallback,
			},
		},
		{
			name: "required_unspecified",
			cfg: webAuthnConfig{
				userVerification:  required,
				authenticatorType: unspecified,
				hasDialog:         true,
				authCallback:      pinAuthCallback,
			},
		},
		{
			name: "required_platform",
			cfg: webAuthnConfig{
				userVerification:  required,
				authenticatorType: platform,
				hasDialog:         true,
				authCallback:      pinAuthCallback,
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := testWebAuthnFlow(ctx, cr, tconn, logReader, tc.cfg); err != nil {
				s.Error("Failed to perform WebAuthn flow: ", err)
			}
		})
	}

}

func testWebAuthnFlow(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	logReader *syslog.ChromeReader, cfg webAuthnConfig) error {
	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := cr.NewConn(ctx, "https://webauthn.io/")
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer conn.Close()

	// Perform MakeCredential on the test website.

	// Use a random username because webauthn.io keeps state for each username for a period of time.
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email').value = "%s"`, randomUsername()), nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// Select "Authenticator Type"
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('select-authenticator').value= "%s"`, cfg.authenticatorType.Value()), nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// Select "User Verification"
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('select-verification').value= "%s"`, cfg.userVerification.Value()), nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// Press "Register" button
	err = conn.Eval(ctx, `document.getElementById('register-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	ui := uiauto.New(tconn)

	// If authenticator type is "Platform", there's only platform option so we don't have to manually click "This device".
	if cfg.authenticatorType != platform {
		// Choose platform authenticator
		platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
		if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to select platform authenticator from transport selection sheet")
		}
		if err := ui.LeftClick(platformAuthenticatorButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to click button for platform authenticator")
		}
	}

	if cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return errors.Wrap(err, "ChromeOS dialog did not show up")
		}
	}
	// Perform authentication.
	if err := cfg.authCallback(); err != nil {
		return err
	}

	if err := util.AssertMakeCredentialSuccess(ctx, logReader); err != nil {
		return errors.Wrap(err, "MakeCredential did not succeed")
	}

	// Perform GetAssertion on the test website.

	// Press "Login" button.
	err = conn.Eval(ctx, `document.getElementById('login-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	if cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return errors.Wrap(err, "ChromeOS dialog did not show up")
		}
	}
	// Perform authentication.
	if err := cfg.authCallback(); err != nil {
		return err
	}

	if err := util.AssertGetAssertionSuccess(ctx, logReader); err != nil {
		return errors.Wrap(err, "GetAssertion did not succeed")
	}

	return nil
}

func (uv userVerification) Value() string {
	switch uv {
	case discouraged:
		return "discouraged"
	case preferred:
		return "preferred"
	case required:
		return "required"
	}
	return "unknown"
}

func (t authenticatorType) Value() string {
	switch t {
	case unspecified:
		return ""
	case crossPlatform:
		return "cross-platform"
	case platform:
		return "platform"
	}
	return "unknown"
}

// randomUsername returns a random username of length 10.
func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 10)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}

// setU2fdFlags sets the flags and restarts u2fd, which will re-create the u2f device.
func setU2fdFlags(ctx context.Context, helper *hwseclocal.CmdHelperLocal, u2f, g2f bool) (retErr error) {
	const (
		uf2ForcePath = "/var/lib/u2f/force/u2f.force"
		gf2ForcePath = "/var/lib/u2f/force/g2f.force"
	)

	cmd := helper.CmdRunner()
	dCtl := helper.DaemonController()

	if err := dCtl.Stop(ctx, hwsec.U2fdDaemon); err != nil {
		return errors.Wrap(err, "failed to stop u2fd")
	}
	defer func() {
		if err := dCtl.Start(ctx, hwsec.U2fdDaemon); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to restart u2fd: ", err)
			} else {
				retErr = errors.Wrap(err, "failed to restart u2fd")
			}
		}
	}()

	// Remove flags.
	if _, err := cmd.Run(ctx, "sh", "-c", "rm -f /var/lib/u2f/force/*.force"); err != nil {
		return errors.Wrap(err, "failed to remove flags")
	}
	if u2f {
		if _, err := cmd.Run(ctx, "touch", uf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set u2f flag")
		}
	}
	if g2f {
		if _, err := cmd.Run(ctx, "touch", gf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set g2f flag")
		}
	}
	return nil
}
