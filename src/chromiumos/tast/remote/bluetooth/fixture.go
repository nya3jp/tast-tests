// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"

	btc "chromiumos/tast/common/bluetooth"
	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dbus"
	"chromiumos/tast/remote/wificell/fileutil"
	"chromiumos/tast/rpc"
	bts "chromiumos/tast/services/cros/bluetooth"
	chromeService "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Fixture variable keys.
const (
	// fixtureVarBTPeers is the name of the tast var that specifies a
	// comma-separated list of btpeer host addresses.
	//
	// This is an optional override to the usual btpeer addresses which are normally
	// resolved based on the DUT hostname.
	fixtureVarBTPeers = "btpeers"

	fixtureVarSigninKey = "ui.signinProfileTestExtensionManifestKey"

	fixtureVarChromeUsername = "chrome_username"
	fixtureVarChromePassword = "chrome_password"
)

const (
	defaultChromeUsername = "testuser@gmail.com"
	defaultChromePassword = "testpass"
)

const (
	serviceDepBTTestService = "tast.cros.bluetooth.BTTestService"
	serviceDepChromeService = "tast.cros.browser.ChromeService"
)

const (
	setUpTimeout    = 80 * time.Second
	resetTimeout    = 65 * time.Second
	tearDownTimeout = 70 * time.Second
	postTestTimeout = 1 * time.Second

	// btpeerTimeoutBuffer is added to fixture per btpeer expected to give
	// additional time to manage each btpeer.
	btpeerTimeoutBuffer = 15 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWithBluetoothEnabled",
		Desc: "Logs into a user session and enables Bluetooth during set up and disables it during tear down",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_FAKE_LOGIN,
		}),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: tearDownTimeout,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWith1BTPeer",
		Desc: "Logs into a user session, enables Bluetooth, and connects to 1 btpeer",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             1,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_FAKE_LOGIN,
		}),
		Vars:            []string{fixtureVarBTPeers},
		SetUpTimeout:    setUpTimeout + btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWith2BTPeers",
		Desc: "Logs into a user session, enables Bluetooth, and connects to 2 btpeers",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             2,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_FAKE_LOGIN,
		}),
		Vars:            []string{fixtureVarBTPeers},
		SetUpTimeout:    setUpTimeout + 2*btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + 2*btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + 2*btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWith3BTPeers",
		Desc: "Logs into a user session, enables Bluetooth, and connects to 3 btpeers",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             3,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_FAKE_LOGIN,
		}),
		Vars:            []string{fixtureVarBTPeers},
		SetUpTimeout:    setUpTimeout + 3*btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + 3*btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + 3*btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInWith4BTPeers",
		Desc: "Logs into a user session, enables Bluetooth, and connects to 4 btpeers",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             4,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_FAKE_LOGIN,
		}),
		Vars:            []string{fixtureVarBTPeers},
		SetUpTimeout:    setUpTimeout + 4*btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + 4*btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + 4*btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeOobeWith1BTPeer",
		Desc: "Puts the DUT into OOBE, enables Bluetooth, and connects to 1 btpeer",
		Contacts: []string{
			"chadduffin@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             1,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{"OobeHidDetectionRevamp"},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_NO_LOGIN,
			EnableHidScreenOnOobe:   true,
		}),
		Vars:            []string{fixtureVarBTPeers, fixtureVarSigninKey},
		SetUpTimeout:    setUpTimeout + btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "chromeLoggedInAsUserWithFastPairAnd1BTPeer",
		Desc: "Logs into a chrome as a specific user and enables Bluetooth, FastPair, and connects to 1 btpeer",
		Contacts: []string{
			"jaredbennett@chromium.org",
			"cros-connectivity@google.com",
		},
		Impl: newFixture(&fixtureFeatures{
			BTPeerCount:             1,
			BluetoothAdapterEnabled: true,
			EnableFeatures:          []string{"FastPair"},
			DisableFeatures:         []string{},
			LoginMode:               chromeService.LoginMode_LOGIN_MODE_GAIA_LOGIN,
			RequireChromeUserVars:   true,
			EnableFastPairVars:      true,
		}),
		Vars: []string{
			fixtureVarBTPeers,
			fixtureVarChromeUsername,
			fixtureVarChromePassword,
		},
		SetUpTimeout:    setUpTimeout + btpeerTimeoutBuffer,
		ResetTimeout:    resetTimeout + btpeerTimeoutBuffer,
		TearDownTimeout: tearDownTimeout + btpeerTimeoutBuffer,
		PostTestTimeout: postTestTimeout,
		ServiceDeps:     []string{serviceDepBTTestService, serviceDepChromeService},
	})
}

type fixtureFeatures struct {
	// BTPeerCount requires the specified amount of btpeers to exist in the
	// testbed and connects to them during setup. A testbed can have more btpeers
	// than the BTPeerCount, but only that many connections are configured.
	BTPeerCount int

	// BluetoothAdapterEnabled being true will cause the DUT bluetooth adapter
	// to be enabled during fixture.SetUp and fixture.Reset and disabled during
	// fixture.TearDown.
	BluetoothAdapterEnabled bool

	// EnableFeatures is the list of features that will be enabled when starting Chrome.
	EnableFeatures []string

	// DisableFeatures is the list of features that will be enabled when starting Chrome.
	DisableFeatures []string

	// LoginMode is what the resulting login mode should be after starting Chrome.
	LoginMode chromeService.LoginMode

	// EnableHidScreenOnOobe enables HID detection screen when in OOBE.
	EnableHidScreenOnOobe bool

	// RequireChromeUserVars enables retrieving chrome user credentials from
	// fixture vars, and requires that they are provided.
	RequireChromeUserVars bool

	// EnableFastPairVars enabled to retrieval of fast pair fixture vars.
	EnableFastPairVars bool
}

// FixtValue is the value of the test fixture accessible within a test. All
// variables are configured in fixture.SetUp so that tests can use them without
// any further configuration.
type FixtValue struct {
	// BTPeers is a list of chameleond clients that are connected to each btpeer
	// available to the test fixture.
	BTPeers []chameleon.Chameleond

	// DUTRPCClient is a gRPC client that remains connected to the DUT throughout
	// the life of the test fixture. This can be used to create clients to
	// additional local tast services.
	DUTRPCClient *rpc.Client

	// BTS is a client of the BTTestService that uses the DUTRPCClient connection.
	BTS bts.BTTestServiceClient

	// ChromeService is a client of the ChromeService that is used to start Chrome.
	ChromeService chromeService.ChromeServiceClient
}

type fixture struct {
	features                     *fixtureFeatures
	fv                           *FixtValue
	dbusMonitorBluetoothServices *dbus.Monitor
}

func newFixture(features *fixtureFeatures) *fixture {
	return &fixture{
		features: features,
		fv:       &FixtValue{},
	}
}

// SetUp preforms fixture setup actions. All fixtureFeatures are configured.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Connect to btpeers and reset them to a fresh state.
	if err := tf.setUpBTPeers(ctx, s, tf.features.BTPeerCount); err != nil {
		s.Fatal("Failed to set up btpeers: ", err)
	}
	if err := tf.resetBTPeers(ctx); err != nil {
		s.Fatal("Failed to reset all btpeers: ", err)
	}

	// Connect to local gRPC services, and keep connection alive until after
	// TearDown is called by using the fixture context.
	if rpcClient, err := rpc.Dial(s.FixtContext(), s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to connect to the local gRPC service on the DUT: ", err)
	} else {
		tf.fv.DUTRPCClient = rpcClient
	}
	tf.fv.BTS = bts.NewBTTestServiceClient(tf.fv.DUTRPCClient.Conn)

	tf.fv.ChromeService = chromeService.NewChromeServiceClient(tf.fv.DUTRPCClient.Conn)

	var signinProfileTestExtensionID string
	if tf.features.EnableHidScreenOnOobe {
		var ok bool
		signinProfileTestExtensionID, ok = s.Var(fixtureVarSigninKey)
		if !ok {
			s.Fatal("Failed to get sign-in key variable required for OOBE tests")
		}
	}

	// Start Chrome with the features and login mode provided by the test fixture.
	var chromeUsername, chromePassword string
	if tf.features.RequireChromeUserVars {
		chromeUsername = s.RequiredVar(fixtureVarChromeUsername)
		chromePassword = s.RequiredVar(fixtureVarChromePassword)
	} else {
		chromeUsername = defaultChromeUsername
		chromePassword = defaultChromePassword
	}
	if _, err := tf.fv.ChromeService.New(ctx, &chromeService.NewRequest{
		LoginMode:       tf.features.LoginMode,
		EnableFeatures:  tf.features.EnableFeatures,
		DisableFeatures: tf.features.DisableFeatures,
		Credentials: &chromeService.NewRequest_Credentials{
			Username: chromeUsername,
			Password: chromePassword,
		},
		EnableHidScreenOnOobe:        tf.features.EnableHidScreenOnOobe,
		SigninProfileTestExtensionId: signinProfileTestExtensionID,
	}); err != nil {
		s.Fatal("Failed to log into chrome on DUT: ", err)
	}

	// Start capturing bluez and floss D-Bus messages.
	dbusMonitorBluetoothServices, err := dbus.StartMonitor(
		ctx,
		s.DUT().Conn(),
		"--system",
		"destination='org.bluez'",
		"destination='org.chromium.bluetooth'",
	)
	if err != nil {
		s.Fatal("Failed to start dbus-monitor listening to bluez and floss service messages: ", err)
	}
	tf.dbusMonitorBluetoothServices = dbusMonitorBluetoothServices

	// Enable bluetooth adapter and clear devices.
	if tf.features.BluetoothAdapterEnabled {
		if _, err := tf.fv.BTS.EnableBluetoothAdapter(ctx, &emptypb.Empty{}); err != nil {
			s.Fatal("Failed to enable bluetooth adapter on DUT: ", err)
		}
		if err := tf.clearDutBluetoothDevices(ctx); err != nil {
			s.Error("Failed to clear DUT bluetooth devices: ", err)
		}
	}

	// Save collected bluez D-Bus messages collected thus far.
	if err := tf.logDBusMonitorBluetoothMessages(ctx, "SetUp"); err != nil {
		s.Fatal("Failed to collect dbus-monitor bluez logs: ", err)
	}
	return tf.fv
}

// Reset is called by the framework after each test (except for the last one) to
// do a light-weight reset of the environment to the original state.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) Reset(ctx context.Context) (retErr error) {
	if tf.features.BluetoothAdapterEnabled {
		if _, err := tf.fv.BTS.EnableBluetoothAdapter(ctx, &emptypb.Empty{}); err != nil {
			return errors.Wrap(err, "failed to enable bluetooth adapter on DUT")
		}
		if err := tf.clearDutBluetoothDevices(ctx); err != nil {
			return errors.Wrap(err, "failed to clear DUT bluetooth devices")
		}
	}
	if err := tf.resetBTPeers(ctx); err != nil {
		return errors.Wrap(err, "failed to reset all btpeers")
	}
	if err := tf.logDBusMonitorBluetoothMessages(ctx, "Reset"); err != nil {
		return errors.Wrap(err, "failed to collect dbus-monitor bluez logs")
	}
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set
// up for the test.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

// PostTest is called by the framework after each test to tear down changes
// PreTest made.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Save any new dbus logs that occurred during the test.
	if err := tf.logDBusMonitorBluetoothMessages(ctx, "PostTest"); err != nil {
		s.Fatal("Failed to collect dbus-monitor bluez logs: ", err)
	}
}

// TearDown is called by the framework to tear down the environment SetUp set
// up.
//
// This is necessary to implement testing.FixtureImpl.
func (tf *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Turn bluetooth adapter back off.
	if tf.features.BluetoothAdapterEnabled {
		if err := tf.clearDutBluetoothDevices(ctx); err != nil {
			s.Error("Failed to clear DUT bluetooth devices: ", err)
		}
		if _, err := tf.fv.BTS.DisableBluetoothAdapter(ctx, &emptypb.Empty{}); err != nil {
			s.Error("Failed to disable bluetooth adapter on DUT: ", err)
		}
	}

	// Clean up chrome login state.
	if _, err := tf.fv.ChromeService.Close(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to close Chrome on the DUT: ", err)
	}

	// Close gRPC connection to DUT.
	if err := tf.fv.DUTRPCClient.Close(ctx); err != nil {
		s.Error("Failed to close gRPC connection to DUT: ", err)
	}

	// Reset btpeers to original state.
	if err := tf.resetBTPeers(ctx); err != nil {
		s.Error("Failed to reset all btpeers: ", err)
	}

	// Stop dbus-monitor.
	if err := tf.logDBusMonitorBluetoothMessages(ctx, "TearDown"); err != nil {
		s.Error("Failed to collect dbus-monitor bluez logs: ", err)
	}
	if err := tf.dbusMonitorBluetoothServices.Close(); err != nil {
		s.Error("Failed to close dbus-monitor: ", err)
	}
}

func (tf *fixture) setUpBTPeers(ctx context.Context, s *testing.FixtState, requiredBTPeers int) error {
	ctx, st := timing.Start(ctx, fmt.Sprintf("setUpBTPeers_%d", requiredBTPeers))
	defer st.End()
	if requiredBTPeers <= 0 {
		return nil
	}
	var btpeerAddresses []string
	if btpeersVar, isSet := s.Var(fixtureVarBTPeers); isSet && btpeersVar != "" {
		btpeerAddresses = strings.Split(btpeersVar, ",")
		if len(btpeerAddresses) < requiredBTPeers {
			return errors.Errorf("fixture requires at least %d btpeers, but "+
				"only %d were provided in the %s tast var (%q)",
				requiredBTPeers, len(btpeerAddresses),
				fixtureVarBTPeers, btpeersVar)
		}
		btpeerAddresses = btpeerAddresses[:requiredBTPeers]
	} else {
		// Imply btpeer hostnames based on DUT hostname.
		btpeerAddresses = make([]string, requiredBTPeers)
		dutHostname := strings.Split(s.DUT().HostName(), ":")[0]
		if dutHostname == "localhost" || dutHostname == "" || dutHostname == "127.0.0.1" {
			for i := 0; i < requiredBTPeers; i++ {
				btpeerAddresses[i] = fmt.Sprintf("localhost:%d", 2201+i)
			}
			exampleTastCall := fmt.Sprintf("tast run --var=%s=%s %s <test>",
				fixtureVarBTPeers, strings.Join(btpeerAddresses, ","),
				s.DUT().HostName())
			return errors.Errorf("btpeer hostname resolution not supported "+
				"when DUT hostname is %q. If tast is being run in a local "+
				"development environment outside of the lab, ssh tunnel to the "+
				"btpeers outside of the chroot and provide the local forwarded"+
				" address to tast using the %q tast var (e.g. %q)",
				dutHostname, fixtureVarBTPeers, exampleTastCall)
		}
		for i := 0; i < requiredBTPeers; i++ {
			btpeerAddresses[i] = fmt.Sprintf("%s-btpeer%d", dutHostname, i+1)
		}
	}
	testing.ContextLogf(ctx, "Connecting to %d btpeers: %s",
		len(btpeerAddresses), strings.Join(btpeerAddresses, ", "))
	btpeers, err := btc.ConnectToBTPeers(ctx, btpeerAddresses)
	if err != nil {
		return err
	}
	tf.fv.BTPeers = btpeers
	testing.ContextLogf(ctx, "Successfully connected to %d btpeers",
		len(tf.fv.BTPeers))
	return nil
}

// resetBTPeers resets each configured btpeer to return them to their normal
// state and clear any changes a test may have made to them.
// Each btpeer is reset in parallel to save time. If any reset fails, the first
// error is returned and any pending resets are cancelled.
func (tf *fixture) resetBTPeers(ctx context.Context) error {
	ctx, st := timing.Start(ctx, fmt.Sprintf("resetBTPeers_%d", len(tf.fv.BTPeers)))
	defer st.End()
	if len(tf.fv.BTPeers) == 0 {
		return nil
	}
	testing.ContextLogf(ctx, "Resetting %d btpeers", len(tf.fv.BTPeers))
	resetCtx, cancelResetCtx := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelResetCtx()
	resetGroup, resetCtx := errgroup.WithContext(resetCtx)
	for i, btpeer := range tf.fv.BTPeers {
		resetGroup.Go(func() error {
			return tf.resetBTPeer(resetCtx, i, btpeer)
		})
	}
	if err := resetGroup.Wait(); err != nil {
		return errors.Wrap(err, "failed to reset btpeers")
	}
	return nil
}

func (tf *fixture) resetBTPeer(ctx context.Context, btpeerNum int, btpeer chameleon.Chameleond) error {
	// Reset the base chameleond service state.
	if err := btpeer.Reset(ctx); err != nil {
		return errors.Wrapf(err, "failed to reset chameleond on btpeer[%d] at %q", btpeerNum, btpeer.Host())
	}
	// Reset the bluetooth service state, through the keyboard device interface
	// since this method is not exposed at a higher level.
	if err := btpeer.BluetoothKeyboardDevice().ResetStack(ctx, ""); err != nil {
		return errors.Wrapf(err, "failed to reset bluetooth stack on btpeer[%d] at %q", btpeerNum, btpeer.Host())
	}
	return nil
}

func (tf *fixture) logDBusMonitorBluetoothMessages(ctx context.Context, logName string) error {
	ctx, st := timing.Start(ctx, "logDBusMonitorBluetoothMessages")
	defer st.End()
	// Prepare output file.
	dstLogFilename := tf.buildLogFilename(logName)
	dstFilePath := filepath.Join("dbus_monitor_bluetooth", dstLogFilename)
	f, err := fileutil.PrepareOutDirFile(ctx, dstFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare output dir file %q", dstFilePath)
	}
	// Dump buffer of collected logs to file.
	if err := tf.dbusMonitorBluetoothServices.Dump(f); err != nil {
		return errors.Wrapf(err, "failed to dump dbus-monitor logs to %q", dstFilePath)
	}
	return nil
}

// buildLogFilename builds a log filename with a minimal timestamp prefix, all
// the name parts in the middle delimited by "_" with non-word characters
// replaced with underscores, and a ".log" file extension.
//
// This not only communicates the time of the log to users, but keeps similar
// files in chronological order within the same directory when displayed sorted
// by name (alphanumerical order) by most programs.
//
// Example result: "20220523-122753_dbus_bluetooth_PostTest"
func (tf *fixture) buildLogFilename(nameParts ...string) string {
	// Build timestamp prefix.
	timestamp := time.Now().Format("20060102-150405")
	// Join and sanitize name parts.
	name := strings.Join(nameParts, "_")
	name = regexp.MustCompile("\\W").ReplaceAllString(name, "_")
	name = regexp.MustCompile("_+").ReplaceAllString(name, "_")
	// Combine timestamp, name, and extension.
	return fmt.Sprintf("%s_%s.log", timestamp, name)
}

// clearDutBluetoothDevices disconnects the DUT from any connected bluetooth
// devices and also removes all known bluetooth devices from the DUT.
func (tf *fixture) clearDutBluetoothDevices(ctx context.Context) error {
	testing.ContextLog(ctx, "Disconnecting all bluetooth devices from DUT")
	if _, err := tf.fv.BTS.DisconnectAllDevices(ctx, &emptypb.Empty{}); err != nil {
		return errors.Wrap(err, "failed to disconnected all bluetooth devices")
	}
	testing.ContextLog(ctx, "Removing all bluetooth devices from DUT")
	if _, err := tf.fv.BTS.RemoveAllDevices(ctx, &emptypb.Empty{}); err != nil {
		return errors.Wrap(err, "failed to remove all bluetooth devices")
	}
	return nil
}
