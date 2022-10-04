// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/testing"
)

const (
	setUpTimeout    = time.Minute
	tearDownTimeout = time.Minute
	preTestTimeout  = time.Minute
	postTestTimeout = time.Minute
)

func ashNoNudgesExtraArg() chrome.Option {
	return chrome.ExtraArgs("--ash-no-nudges")
}

var (
	chameleonHostname = testing.RegisterVarString(
		"assistant.chameleon_host",
		"localhost",
		"Hostname for Chameleon")

	chameleonSSHPort = testing.RegisterVarString(
		"assistant.chameleon_ssh_port",
		"22",
		"SSH port for Chameleon")

	chameleonPort = testing.RegisterVarString(
		"assistant.chameleon_port",
		"9992",
		"Port for chameleond on Chameleon")
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "assistantBase",
		Desc: "Chrome session for assistant testing",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				VerboseLogging(),
				ashNoNudgesExtraArg(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithStartAudioDecoderOnDemand",
		Desc: "Chrome session for assistant testing with StartAssistantAudioDecoderOnDemand flag",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				VerboseLogging(),
				ashNoNudgesExtraArg(),
				chrome.EnableFeatures("StartAssistantAudioDecoderOnDemand"),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// Assistant fixtures use assistant test GAIA for tests with Arc++ feature
	// as we have to make sure that necessary bits are enabled to run our tests,
	// e.g. device apps.
	//
	// Assistant Android support (e.g. open local Android app) requires Play
	// Store opt-in and device apps bit.
	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithPlayStore",
		Desc: "Assistant test GAIA chrome session with Play Store",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Vars: []string{"assistant.username", "assistant.password"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("assistant.username"),
					Pass: s.RequiredVar("assistant.password"),
				}),
				VerboseLogging(),
				ashNoNudgesExtraArg(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		PostTestTimeout: arc.PostTestTimeout,
		ResetTimeout:    arc.ResetTimeout,
		TearDownTimeout: arc.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantOOBEUsedVMReady",
		Desc: "Assistant OOBE screen with a GAIA which has used Assistant before and whose Voice Match is ready",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Vars:            []string{"assistant.username", "assistant.password", "ui.signinProfileTestExtensionManifestKey"},
		Impl:            NewOOBEFixture(),
		SetUpTimeout:    30 * time.Second,
		PostTestTimeout: 30 * time.Second,
		PreTestTimeout:  chrome.GAIALoginTimeout + 3*time.Minute,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantBaseWithHotword",
		Desc: "Chrome session for assistant testing with Hotword enabled",
		Contacts: []string{
			"yawano@google.com",
			"assitive-eng@google.com",
		},
		Vars: []string{"assistant.username", "assistant.password"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(chrome.Creds{
					User: s.RequiredVar("assistant.username"),
					Pass: s.RequiredVar("assistant.password"),
				}),
				VerboseLogging(),
				ashNoNudgesExtraArg(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistant",
		Desc: "Assistant is enabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBase",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithStartAudioDecoderOnDemand",
		Desc: "Assistant is enabled",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithStartAudioDecoderOnDemand",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantClamshell",
		Desc: "Assistant is enabled in Clamshell mode",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:          "assistant",
		Impl:            newTabletFixture(false),
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithArc",
		Desc: "Assistant is enabled with Arc",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithPlayStore",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			preData := s.ParentValue().(*arc.PreData)
			return FixtData{
				Chrome: preData.Chrome,
				ARC:    preData.ARC,
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantClamshellPerf",
		Desc: "Assistant clamshell fixture for running performance test",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:         "assistantClamshell",
		Impl:           newPerfFixture(),
		PreTestTimeout: perfFixturePreTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantPerf",
		Desc: "Assistant fixture for running performance test",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent:         "assistant",
		Impl:           newPerfFixture(),
		PreTestTimeout: perfFixturePreTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithHotword",
		Desc: "Assistant is enabled with Hotword support",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantBaseWithHotword",
		Impl: NewAssistantFixture(func(s *testing.FixtState) FixtData {
			return FixtData{
				Chrome: s.ParentValue().(*chrome.Chrome),
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "assistantWithAudioBox",
		Desc: "Assistant is enabled with Hotword support and Chameleon access",
		Contacts: []string{
			"yawano@google.com",
			"assistive-eng@google.com",
		},
		Parent: "assistantWithHotword",
		Impl: NewAudioBoxFixture(func(s *testing.FixtState) AudioBoxFixtData {
			fixtData := s.ParentValue().(*FixtData)
			return AudioBoxFixtData{
				FixtData: *fixtData,
			}
		}),
		PreTestTimeout:  preTestTimeout,
		PostTestTimeout: postTestTimeout,
	})
}

type tabletFixture struct {
	enabled bool
	cleanup func(ctx context.Context) error
}

func newTabletFixture(e bool) testing.FixtureImpl {
	return &tabletFixture{
		enabled: e,
	}
}

func (f *tabletFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fixtData := s.ParentValue().(*FixtData)
	cr := fixtData.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, f.enabled)
	if err != nil {
		s.Fatal("Failed to put into specified mode: ", err)
	}
	f.cleanup = cleanup

	// If a DUT switches from Tablet mode to Clamshell mode, it can take a while
	// until launcher gets settled down.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
		s.Fatal("Failed to wait the launcher state Closed: ", err)
	}

	return fixtData
}

func (f *tabletFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.cleanup != nil {
		f.cleanup(ctx)
	}
}

func (f *tabletFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *tabletFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *tabletFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

type parentFixtDataCallback func(s *testing.FixtState) FixtData

type enabledFixture struct {
	cr *chrome.Chrome
	cb parentFixtDataCallback
}

// FixtData is fixture data of assistant fixture.
type FixtData struct {
	Chrome *chrome.Chrome
	ARC    *arc.ARC
}

// NewAssistantFixture returns new assistant fixture.
func NewAssistantFixture(cb parentFixtDataCallback) testing.FixtureImpl {
	return &enabledFixture{
		cb: cb,
	}
}

func (f *enabledFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fixtData := f.cb(s)
	f.cr = fixtData.Chrome

	return &fixtData
}

func (f *enabledFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *enabledFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *enabledFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
}

func (f *enabledFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Run Cleanup in PostTest instead of TearDown as we want to capture a
	// screenshot if a test fails. Also a previous test might leave the launcher
	// open if it failed by missing an expected response. It can cause a
	// following test to fail. Disabling assistant will close the launcher.
	if err := Cleanup(ctx, s.HasError, f.cr, tconn); err != nil {
		s.Fatal("Failed to disable Assistant: ", err)
	}
}

type perfFixture struct{}

// 2 mins is coming from waitIdleCPUTimeout in cpu.WaitUntilIdle.
const perfFixturePreTestTimeout = 2 * time.Minute

func newPerfFixture() testing.FixtureImpl {
	return &perfFixture{}
}

func (f *perfFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return s.ParentValue()
}

func (f *perfFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

func (f *perfFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *perfFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// We don't want to include noises from cpu busy state.
	// As a best practice, wait cpu idle time before running a performance related test.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for cpu idle time: ", err)
	}
}

func (f *perfFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

type parentAudioBoxFixtDataCallback func(s *testing.FixtState) AudioBoxFixtData

type audioBoxFixture struct {
	audioBoxFixtData *AudioBoxFixtData
	cb               parentAudioBoxFixtDataCallback
}

// AudioBoxFixtData is fixture data of assistant fixture with chameleon support.
type AudioBoxFixtData struct {
	FixtData
	Chameleon         chameleon.Chameleond
	ChameleonHostname string
	ChameleonPort     int
	ChameleonSSHPort  int
}

// NewAudioBoxFixture returns new fixture.
func NewAudioBoxFixture(cb parentAudioBoxFixtDataCallback) testing.FixtureImpl {
	return &audioBoxFixture{
		cb: cb,
	}
}

func (f *audioBoxFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var err error
	audioBoxFixtData := f.cb(s)

	// Verify that port numbers are integers.
	audioBoxFixtData.ChameleonPort, err = strconv.Atoi(chameleonPort.Value())
	if err != nil {
		s.Fatalf("Failed to convert assistant.chameleon_port with value: %s to integer: %v", chameleonPort.Value(), err)
	}
	audioBoxFixtData.ChameleonSSHPort, err = strconv.Atoi(chameleonSSHPort.Value())
	if err != nil {
		s.Fatalf("Failed to convert assistant.chameleon_ssh_port with value: %s to integer: %v", chameleonSSHPort.Value(), err)
	}
	audioBoxFixtData.ChameleonHostname = chameleonHostname.Value()

	// Setup Chameleon
	// In Skylab, DUT and chameleon follow the naming convention: <dut> and <dut>-chameleon
	// While DUT and chamelon can ssh directly though IPs against each other, they cannot
	// resolve machine names to IPs and IP resolution has to be done outside of the local test.
	// Drone keeps the metadata of DUT and chameleon and can help resolve hostname to IP.
	// Drone will pass information like chameleon host, chameleon host_port, ssh_port as
	// tast input through the autotest control file.
	chameleonAddr := fmt.Sprintf("%s:%d", chameleonHostname.Value(), audioBoxFixtData.ChameleonPort)

	// Connect to chameleon with retries.
	err = action.Retry(5, func(ctx context.Context) error {
		s.Logf("Connect to Chameleon:%s", chameleonAddr)
		audioBoxFixtData.Chameleon, err = chameleon.NewChameleond(ctx, chameleonAddr)
		return err
	}, time.Second)(ctx)

	if err != nil {
		s.Fatal("Failed to connect to chameleon board: ", err)
	}

	analogAudioLineOutPortID, err := audioBoxFixtData.Chameleon.FetchSupportedPortIDByType(ctx, chameleon.PortTypeAnalogAudioLineOut, 0)
	if err != nil {
		s.Fatal("Failed to get port id of audio line out port: ", err)
	}
	if hasAudioSupport, err := audioBoxFixtData.Chameleon.HasAudioSupport(ctx, analogAudioLineOutPortID); !hasAudioSupport || err != nil {
		s.Fatalf("Chameleon has no audio support for %v: %v", analogAudioLineOutPortID, err)
	}
	f.audioBoxFixtData = &audioBoxFixtData

	return &audioBoxFixtData
}

func (f *audioBoxFixture) TearDown(ctx context.Context, s *testing.FixtState) {

}

func (f *audioBoxFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *audioBoxFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Reset Chameleon to ensure a consistent state for testing.
	if f.audioBoxFixtData.Chameleon != nil {
		if err := f.audioBoxFixtData.Chameleon.Reset(ctx); err != nil {
			s.Fatal("Failed to reset Chameleon: ", err)
		}
	}
}

func (f *audioBoxFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

// oobeFixture is a fixture for testing OOBE flow. Currently it only supports a case where a GAIA
// has already used Assistant before and its voice match is ready.
type oobeFixture struct {
	// fixtData pointer might be copied by a framework and shared between tests. Do not change a
	// pointer in PreTest/Reset/PostTest.
	fixtData *OOBEFixtData
	// chrome options used to start a Chrome. chromeOpts is filled in SetUp while a Chrome is
	// started in PreTest. Some of chrome options require an access to testing.FixtState and it's
	// not available in PreTest.
	chromeOpts []chrome.Option
}

// OOBEFixtData is a fixture data for Assistant OOBE.
type OOBEFixtData struct {
	OOBECtx OOBEContext
}

// NewOOBEFixture returns an Assistant OOBE fixture.
func NewOOBEFixture() testing.FixtureImpl {
	return &oobeFixture{}
}

func (f *oobeFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.chromeOpts = []chrome.Option{
		VerboseLogging(),
		ashNoNudgesExtraArg(),
		chrome.NoLogin(),
		chrome.DeferLogin(),
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("assistant.username"),
			Pass: s.RequiredVar("assistant.password"),
		}),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	}
	oobeFixtData := OOBEFixtData{}
	f.fixtData = &oobeFixtData

	return f.fixtData
}

func (f *oobeFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Go through OOBE in PreTest as we cannot re-use this instance after a test.
	cr, err := chrome.New(ctx, f.chromeOpts...)
	if err != nil {
		s.Fatal("Failed to create a Chrome: ", err)
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create an OOBE connection: ", err)
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to obtain a test API Conn: ", err)
	}

	oobeCtx := OOBEContext{
		OOBEConn: oobeConn,
		Chrome:   cr,
		TConn:    tconn,
	}
	f.fixtData.OOBECtx = oobeCtx

	if err := GoThroughOOBEScreen(ctx, &WelcomeScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through welcome screen: ", err)
	}
	if err := GoThroughOOBEScreen(ctx, &UserCreationScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through user creation screen: ", err)
	}
	if err := GoThroughOOBEScreen(ctx, &GAIAScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through GAIA screen: ", err)
	}
	if err := GoThroughOOBEScreen(ctx, &ConsolidatedConsentScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through consolidated consent screen: ", err)
	}
	if err := GoThroughOOBEScreen(ctx, &SyncScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through sync screen: ", err)
	}

	// FingerprintScreen appears only on a device which has a fingerprint scanner.
	skipFingerprintScreen, err := shouldSkip(ctx, &FingerprintScreen, &oobeCtx)
	if err != nil {
		s.Fatal("Failed to evaluate whether to skip fingerprint screen or not: ", err)
	}
	if !skipFingerprintScreen {
		if err := GoThroughOOBEScreen(ctx, &FingerprintScreen, &oobeCtx); err != nil {
			s.Fatal("Failed to go through fingerprint screen: ", err)
		}
	}

	if err := GoThroughOOBEScreen(ctx, &PinSetupScreen, &oobeCtx); err != nil {
		s.Fatal("Failed to go through pin setup screen: ", err)
	}

	// Wait Assistant screen before go into a test body.
	if err := isVisible(ctx, &OOBEScreen{oobeAPIName: "AssistantScreen"}, &oobeCtx); err != nil {
		s.Fatal("Failed to wait assistant screen: ", err)
	}
}

func (f *oobeFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *oobeFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	defer func() {
		// Reset fixtData. Do not reset fixtData pointer as it can be copied and shared between
		// tests by a framework.
		f.fixtData.OOBECtx = OOBEContext{}
	}()

	if f.fixtData.OOBECtx.OOBEConn != nil {
		if err := f.fixtData.OOBECtx.OOBEConn.Close(); err != nil {
			s.Fatal("Failed to close OOBEConn: ", err)
		}
	}

	if f.fixtData.OOBECtx.Chrome != nil {
		if err := f.fixtData.OOBECtx.Chrome.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome: ", err)
		}
	}
}

func (f *oobeFixture) TearDown(ctx context.Context, s *testing.FixtState) {}
