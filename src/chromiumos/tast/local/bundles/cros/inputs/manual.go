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

const helpStr = `
	This test is designed to automatically setup test environment for manual verification with given conditions.
	Supported variables:
		userType: 'guest' or 'enterprise'. Login params will be ignored if userType is provided.
		user: Username used for gaia login. Use fake user if user is not supplied.
		pass: Password used for gaia login.
		contact: Contact is required for dual auth on test account login.
		imeID: Input method to be installed and activated for testing. Default IME is xkb:us::eng.
		region: Region for device language. Default value is "us". Other region options: jp, fr.
		vk: 'true' or 'false'. Whether virtual keyboard is force enabled.
		arc: 'true' or 'false'. Whether install arc and input test app. It is disabled by default to optimize booting time.
		crd: 'true' or 'false'. Whether install RDP extension and initializing remote desktop session.
		reset: 'true' or 'false'. Existing home is cleaned every iteration by default unless reset is false.
		chromeArgs: Extra chrome args used to setup the environment. e,g. "--enable-features=LanguageSettingsUpdate2 --disable-features=DefaultWebAppInstallation".

	Example command:
	* Login guest user with virtual keyboard enabled.
		tast run -var=userType=guest -var=vk=true <dut ip> inputs.Manual
	* To test a specific input method on an android app.
		tast run -var=arc=true -var=imeID=nacl_mozc_us <dut ip> inputs.Manual
`

// envSettings represents the configurable parameters for the manual testing environment.
type envSettings struct {
	// userType can be 'guest', 'enterprise'.
	// Enterprise mode only support specific fake user.
	// Login params will be ignored if userType is provided.
	userType string
	// user is the user name for gaia login. Use fake user if user is not supplied.
	user string
	// pass is the password used for gaia login.
	pass string
	// contact is required for dual auth on test account login.
	contact string
	// imeID represents the input method to be installed and activated for testing. Default IME is xkb:us::eng.
	// More code refer to http://google3/i18n/input/javascript/chos/common/input_method_config.textproto.
	imeID string
	// region for device language. Default value is "us". Other region options: jp, fr.
	region string
	// vk indicates whether virtual keyboard is force enabled.
	vk bool
	// arc indicates whether install arc and input test app.
	arc bool
	// crd indicates whether install RDP extension and initializing remote desktop session.
	// Note: this only works in English at the moment.
	crd bool
	// reset cleans existing home every iteration unless it is false.
	reset bool
	// chromeArgs is extra chrome args used to setup the environment.
	chromeArgs []string
}

func init() {
	// Example usage:
	// $ tast run -var=user=<username> -var=pass=<password> <dut ip> inputs.Manual
	// <username> and <password> are the credentials of the test GAIA account.
	testing.AddTest(&testing.Test{
		Func:         Manual,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Login device and setup environment for manual testing purpose",
		Contacts:     []string{"shengjun@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      10 * time.Minute,
		Vars: []string{
			"userType", "user", "pass", "contact", "imeID", "region", "vk",
			"arc", "crd", "reset", "chromeArgs",
		},
		Params: []testing.Param{{
			Name: "",
			Val:  false,
		}, {
			Name: "help",
			Val:  true,
		}},
	})
}

// getVars extracts the testing parameters from testing.State.
func getVars(s *testing.State) *envSettings {
	settings := &envSettings{
		userType:   parseStringVar(s, "userType", ""),
		user:       parseStringVar(s, "user", ""),
		pass:       parseStringVar(s, "pass", ""),
		contact:    parseStringVar(s, "contact", ""),
		imeID:      parseStringVar(s, "imeID", ""),
		region:     parseStringVar(s, "region", ""),
		vk:         true,
		arc:        false,
		crd:        false,
		reset:      true,
		chromeArgs: []string{},
	}

	vkStr := parseStringVar(s, "vk", "true")
	vk, err := strconv.ParseBool(vkStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `vk`: ", err)
	}
	settings.vk = vk

	arcStr := parseStringVar(s, "arc", "false")
	arc, err := strconv.ParseBool(arcStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `arc`: ", err)
	}
	settings.arc = arc

	crdStr := parseStringVar(s, "crd", "false")
	crd, err := strconv.ParseBool(crdStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `crd`: ", err)
	}
	settings.crd = crd

	resetStr := parseStringVar(s, "reset", "true")
	reset, err := strconv.ParseBool(resetStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `reset`: ", err)
	}
	settings.reset = reset

	chromeArgs, hasChromeArgs := s.Var("chromeArgs")
	if hasChromeArgs {
		settings.chromeArgs = strings.Fields(chromeArgs)
	}

	if err := validateSettings(settings); err != nil {
		s.Fatal("Failed to validate variables: ", err)
	}

	return settings
}

// validateSettings verify setting conflicts.
// It terminate testing on unsupported / incompatible settings.
func validateSettings(settings *envSettings) error {
	if settings.crd && (settings.user == "" || settings.userType == "guest") {
		return errors.New("Remote Desktop is not supported in fake user / guest mode")
	}
	return nil
}

func Manual(ctx context.Context, s *testing.State) {
	// Print out help content and end the test.
	isHelp := s.Param().(bool)
	if isHelp {
		testing.ContextLog(ctx, helpStr)
		return
	}

	defer func() {
		if s.HasError() {
			s.Log(`Please run "tast run <dut ip> inputs.Manual.help" for more insructions of this test`)
		}
	}()

	settings := getVars(s)

	testing.ContextLogf(ctx, "Start to set up test environment with settings: %+v", settings)
	var opts []chrome.Option

	switch settings.userType {
	case "guest":
		opts = append(opts, chrome.GuestLogin())
	case "enterprise":
		// Using fakedms and login.
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

	if settings.vk {
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
			err = errors.Wrap(err, "please supply a valid contact email with -var=contact=<contact>")
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

		s.Log("Installing ArcKeyboardTest app")
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}

		act, err := arc.NewActivity(a, pkg, ".MainActivity")
		if err != nil {
			s.Fatal("Failed to create new activity: ", err)
		}
		defer act.Close()

		s.Log("Starting ArcKeyboardTest app")
		if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
			s.Fatal("Failed to start app: ", err)
		}
		s.Log("ArcKeyboardTest app started")
		s.Log("Note: First input is free text and second one is visible password")
	}

	// Add testing input method.
	if settings.imeID != "" {
		if err := ime.AddAndSetInputMethod(ctx, tconn, ime.ChromeIMEPrefix+settings.imeID); err != nil {
			s.Fatalf("Failed to set input method %q: %v", settings.imeID, err)
		}
	}

	if settings.crd {
		if err := crd.Launch(ctx, cr.Browser(), tconn); err != nil {
			s.Fatal("Failed to Launch remote desktop: ", err)
		}
		s.Log("Waiting connection")
		if err := crd.WaitConnection(ctx, tconn); err != nil {
			s.Fatal("No client connected: ", err)
		}
	}
}

//parseStringVar reads string variable. It returns default value if not exist.
func parseStringVar(s *testing.State, varName, defaultStr string) string {
	value, hasValue := s.Var(varName)
	if hasValue {
		return value
	}
	return defaultStr
}
