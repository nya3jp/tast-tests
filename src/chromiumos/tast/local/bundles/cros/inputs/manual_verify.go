// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/crd"
	policyFixt "chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

// envSettings represents the configurable parameters for the manual testing environment.
type envSettings struct {
	// Profile can be 'guest', 'enterprise'.
	// Enterprise mode only support specific fake user.
	// Login params will be igored if profile is valid.
	profile string
	// Username used for gaia login. Use fake user if user is not supplied.
	user string
	// Password used for gaia login.
	pass string
	// Contact is required for dual auth on test account login.
	contact string
	// Input method to be installed and activated for testing. Default IME is xkb:us::eng
	// More code refer to http://google3/i18n/input/javascript/chos/common/input_method_config.textproto
	imeCode string
	// Region for device language. Default value is "US". Other region options: jp, fr.
	region string
	// Whether virtual keyboard is force enabled.
	vkEnabled bool
	// Whether install arc and input test app.
	arc bool
	//Whether install RDP extension and initializing remote desktop session.
	// TODO: only works in English.
	rdp bool
	// Existing home is cleaned every iteration by default unless reset is false.
	reset bool
	// Extra chrome args used to setup the environment.
	chromeArgs []string
}

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> inputs.ManualVerify
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         ManualVerify,
		Desc:         "Login device and setup environment for manual testing purpose",
		Contacts:     []string{"shengjun@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"profile", "user", "pass", "contact", "imeCode", "region", "vkEnabled",
			"arc", "rdp", "reset", "chromeArgs",
		},
	})
}

// getVars extracts the testing parameters from testing.State.
func getVars(s *testing.State) *envSettings {
	settings := &envSettings{
		profile:    "",
		user:       "",
		pass:       "",
		contact:    "",
		imeCode:    "",
		region:     "",
		vkEnabled:  true,
		arc:        false,
		rdp:        false,
		reset:      true,
		chromeArgs: []string{},
	}
	profile, hasProfile := s.Var("profile")
	if hasProfile {
		settings.profile = strings.ToLower(profile)
	}
	user, hasUser := s.Var("user")
	if hasUser {
		settings.user = strings.ToLower(user)
	}

	pass, hasPass := s.Var("pass")
	if hasPass {
		settings.pass = pass
	}

	contact, hasContact := s.Var("contact")
	if hasContact {
		settings.contact = strings.ToLower(contact)
	}

	imeCode, ok := s.Var("imeCode")
	if ok {
		settings.imeCode = imeCode
	}

	region, ok := s.Var("region")
	if ok {
		settings.region = strings.ToLower(region)
	}

	vkEnabledStr, ok := s.Var("vkEnabled")
	if !ok {
		vkEnabledStr = "true"
	}
	vkEnabled, err := strconv.ParseBool(vkEnabledStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `vkEnabled`: ", err)
	}
	settings.vkEnabled = vkEnabled

	_, arcInstalled := s.Var("arc")
	settings.arc = arcInstalled

	_, rdp := s.Var("rdp")
	settings.rdp = rdp

	resetStr, ok := s.Var("reset")
	if !ok {
		resetStr = "true"
	}
	reset, err := strconv.ParseBool(resetStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `reset`: ", err)
	}
	settings.reset = reset

	chromeArgs, hasChromeArgs := s.Var("chromeArgs")
	if hasChromeArgs {
		settings.chromeArgs = strings.Fields(chromeArgs)
	}

	return settings
}

// validateSettings verify setting conflicts.
// It terminate testing on unsupported / incompatible settings.
func validateSettings(settings *envSettings) error {
	if settings.rdp && (settings.user == "" || settings.profile == "guest") {
		return errors.New("Remote Desktop is not supported in fake user / guest mode")
	}
	return nil
}

func ManualVerify(ctx context.Context, s *testing.State) {
	settings := getVars(s)

	testing.ContextLogf(ctx, "Start to set up test environment with settings: %+v", settings)
	var opts []chrome.Option

	switch settings.profile {
	case "guest":
		opts = append(opts, chrome.GuestLogin())
	case "enterprise":
		// Using fakedms and login
		// Start FakeDMS.
		tmpdir, err := ioutil.TempDir("", "fdms-")
		if err != nil {
			s.Fatal("Failed to create fdms temp dir: ", err)
		}
		defer os.RemoveAll(tmpdir)

		testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
		fdms, err := fakedms.New(ctx, tmpdir)
		if err != nil {
			s.Fatal("Failed to start FakeDMS: ", err)
		}
		defer fdms.Stop(ctx)

		if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
			s.Fatal("Failed to write policies to FakeDMS: ", err)
		}
		opts = append(opts,
			chrome.FakeLogin(chrome.Creds{User: policyFixt.Username, Pass: policyFixt.Password}),
			chrome.DMSPolicy(fdms.URL))
	default:
		if settings.user != "" {
			opts = append(opts, chrome.GAIALogin(chrome.Creds{
				User:    settings.user,
				Pass:    settings.pass,
				Contact: settings.contact,
			}))
		}
	}

	if settings.region != "" {
		opts = append(opts, chrome.Region(settings.region))
	}

	if settings.vkEnabled {
		opts = append(opts, chrome.VKEnabled())
	}

	if !settings.reset {
		opts = append(opts, chrome.KeepState())
	}

	chromeARCOpt := chrome.ARCDisabled()
	if settings.arc {
		if arc.Supported() {
			chromeARCOpt = chrome.ARCEnabled()
		} else {
			s.Fatal("The DUT board does not support ARC")
		}
	}
	opts = append(opts, chromeARCOpt)

	if len(settings.chromeArgs) > 0 {
		opts = append(opts, chrome.ExtraArgs(settings.chromeArgs...))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		// In case of authentication error, provide a more informative message to the user.
		if strings.Contains(err.Error(), "chrome.Auth") {
			err = errors.Wrap(err, "please supply a password with -var=pass=<password>")
		} else if strings.Contains(err.Error(), "chrome.Contact") {
			err = errors.Wrap(err, "please supply a contact email with -var=contact=<contact>")
		}
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	var a *arc.ARC
	if settings.arc {
		a, err = arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Add testing input method.
	if settings.imeCode != "" {
		if err := ime.AddAndSetInputMethod(ctx, tconn, ime.IMEPrefix+settings.imeCode); err != nil {
			s.Fatalf("Failed to set input method %q: %v", settings.imeCode, err)
		}
	}

	// Launch e14s-test test page for non-arc environment.
	const e14sTestPage = "https://sites.google.com/corp/view/e14s-test"
	if !settings.arc {
		_, err := cr.NewConn(ctx, e14sTestPage)
		if err != nil {
			s.Fatal("Failed to create renderer: ", err)
		}
	} else {
		// Launch android app for arc testing.
		const (
			apk = "ArcKeyboardTest.apk"
			pkg = "org.chromium.arc.testapp.keyboard"

			fieldID = "org.chromium.arc.testapp.keyboard:id/text"
		)

		s.Log("Starting ArcKeyboardTest app")
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}

		act, err := arc.NewActivity(a, pkg, ".MainActivity")
		if err != nil {
			s.Fatal("Failed to create new activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start app: ", err)
		}
		s.Log("ArcKeyboardTest app started")
		s.Log("Note: First input is free text and second one is visible password")
	}

	if settings.rdp {
		if err := crd.Launch(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to Launch: ", err)
		}
		s.Log("Waiting connection")
		if err := crd.WaitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	}
}
