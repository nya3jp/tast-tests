// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	policyBlob "chromiumos/tast/common/policy"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	// Give long enough timeout for SetUp() and TearDown() as they might need
	// to reboot a broken DUT. SetUp() and Reset() have additional time allotted
	// to reboot routers as well.
	setUpTimeout    = 17 * time.Minute
	tearDownTimeout = 5 * time.Minute
	resetTimeout    = 11 * time.Minute
	postTestTimeout = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixt",
		Desc: "Default wificell setup with router and pcap object. Note that pcap and router can point to the same Access Point. Also, unlike wificellFixtWithCapture, the fixture won't spawn Capturer. Users may spawn Capturer with customize configuration when needed",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesNone),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap", "routertype", "pcaptype"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtWithCapture",
		Desc: "Wificell setup with Capturer on pcap for each configured AP",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesCapture),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap", "routertype", "pcaptype"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtRouterAsPcap",
		Desc: "Wificell setup with default capturer on router instead of pcap",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesCapture | TFFeaturesRouterAsCapture),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap", "routertype", "pcaptype"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtRouters",
		Desc: "Wificell setup with multiple routers",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesRouters),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"routers", "pcap", "routertype", "pcaptype"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtRoaming",
		Desc: "WiFi romaing setup with multiple routers and attenuators",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesRouters | TFFeaturesAttenuator),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"routers", "pcap", "routertype", "pcaptype", "attenuator"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtEnrolled",
		Desc: "Wificell setup with router and pcap object and chrome enrolled",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesEnroll),
		SetUpTimeout:    10 * time.Minute,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: 8 * time.Minute,
		ServiceDeps: []string{
			TFServiceName,
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
		},
		Vars: []string{"router", "pcap", "routertype", "pcaptype"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "wificellFixtCompanionDut",
		Desc: "Wificell setup with companion Chromebook DUT",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Impl:            newTastFixture(TFFeaturesCompanionDUT),
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: tearDownTimeout,
		ServiceDeps:     []string{TFServiceName},
		Vars:            []string{"router", "pcap", "routertype", "pcaptype"},
	})
}

// TFFeatures is an enum type for extra features needed for Tast fixture.
// Note that features can be combined using bitwise OR, e.g. TFFeaturesCapture | TFFeaturesRouters.
type TFFeatures uint8

const (
	// TFFeaturesNone represents a default value.
	TFFeaturesNone TFFeatures = 0
	// TFFeaturesCapture is a feature that spawns packet capturer in TestFixture.
	TFFeaturesCapture = 1 << iota
	// TFFeaturesRouters allows to configure more than one router.
	TFFeaturesRouters
	// TFFeaturesAttenuator feature facilitates attenuator handling.
	TFFeaturesAttenuator
	// TFFeaturesRouterAsCapture configures the router as a capturer as well.
	TFFeaturesRouterAsCapture
	// TFFeaturesEnroll enrolls Chrome.
	TFFeaturesEnroll
	// TFFeaturesCompanionDUT is a feature that spawns companion DUT in TestFixture.
	TFFeaturesCompanionDUT
)

// String returns name component corresponding to enum value(s).
func (enum TFFeatures) String() string {
	if enum == 0 {
		return "default"
	}
	var ret []string
	if enum&TFFeaturesCapture != 0 {
		ret = append(ret, "capture")
		// Punch out the bit to check for weird values later.
		enum ^= TFFeaturesCapture
	}
	if enum&TFFeaturesRouters != 0 {
		ret = append(ret, "routers")
		enum ^= TFFeaturesRouters
	}
	if enum&TFFeaturesAttenuator != 0 {
		ret = append(ret, "attenuator")
		enum ^= TFFeaturesAttenuator
	}
	if enum&TFFeaturesRouterAsCapture != 0 {
		ret = append(ret, "routerAsCapture")
		enum ^= TFFeaturesRouterAsCapture
	}
	// Catch weird cases. Like when somebody extends enum, but forgets to extend this.
	if enum != 0 {
		panic(fmt.Sprintf("Invalid TFFeatures enum, residual bits :%d", enum))
	}

	return strings.Join(ret, "&")
}

// tastFixtureImpl is the Tast implementation of the Wificell fixture.
// Notice the difference between tastFixtureImpl and TestFixture objects.
// The former is the one in the Tast framework; the latter is for
// wificell fixture.
type tastFixtureImpl struct {
	features TFFeatures
	tf       *TestFixture
}

// newTastFixture creates a Tast fixture with given features.
func newTastFixture(features TFFeatures) *tastFixtureImpl {
	return &tastFixtureImpl{
		features: features,
	}
}

// companionName returns the hostname of a companion device.
func (f *tastFixtureImpl) companionName(s *testing.FixtState, suffix string) string {
	name, err := s.DUT().CompanionDeviceHostname(suffix)
	if err != nil {
		s.Fatal("Unable to synthesize name, err: ", err)
	}
	return name
}

// dutHealthCheck checks if the DUT is healthy.
func (f *tastFixtureImpl) dutHealthCheck(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// We create a new gRPC session here to exclude broken gRPC case and save reboots when
	// the DUT is healthy but the gRPC is broken.
	rpcClient, err := rpc.Dial(ctx, d, rpcHint)
	if err != nil {
		return errors.Wrap(err, "cannot create gRPC client")
	}
	defer rpcClient.Close(ctx)

	wifiClient := wifi.NewShillServiceClient(rpcClient.Conn)
	if _, err := wifiClient.HealthCheck(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "health check failed")
	}
	return nil
}

// recoverUnhealthyDUT checks if the DUT is healthy. If not, try to recover it
// with reboot.
func (f *tastFixtureImpl) recoverUnhealthyDUT(ctx context.Context, d *dut.DUT, s *testing.FixtState) error {
	if !d.Connected(ctx) {
		testing.ContextLog(ctx, "DUT found to not be connected before health check; reconnecting to DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := d.WaitConnect(ctx); err != nil {
				return errors.Wrap(err, "failed to connect to DUT")
			}
			return nil
		}, &testing.PollOptions{
			Timeout: 1 * time.Minute,
		}); err != nil {
			return errors.Wrap(err, "failed to wait for DUT to connect")
		}
		testing.ContextLog(ctx, "Successfully reconnected to DUT")
	}
	if err := f.dutHealthCheck(ctx, d, s.RPCHint()); err != nil {
		testing.ContextLog(ctx, "Rebooting the DUT due to health check err: ", err)
		// As reboot will at least break tf.rpc, no reason to keep
		// the existing p.tf. Close it before reboot.
		if f.tf != nil {
			testing.ContextLog(ctx, "Close TestFixture before reboot")
			if err := f.tf.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close TestFixture before DUT reboot recovery: ", err)
			}
			f.tf = nil
		}
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "reboot failed")
		}
	}
	return nil
}

func (f *tastFixtureImpl) enrollChrome(ctx context.Context, s *testing.FixtState, dutIdx int) error {
	pc := policy.NewPolicyServiceClient(f.tf.duts[dutIdx].rpc.Conn)
	pJSON, err := json.Marshal(policyBlob.NewBlob())
	if err != nil {
		return errors.Wrap(err, "failed to serialize policies")
	}

	if _, err := pc.EnrollUsingChrome(ctx, &policy.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		SkipLogin:  true,
	}); err != nil {
		return errors.Wrap(err, "failed to enroll using Chrome")
	}

	if _, err = pc.StopChrome(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to close Chrome instance")
	}

	return nil
}

func (f *tastFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if f.features&TFFeaturesEnroll != 0 {
		// Do this before NewTestFixture as DUT might be rebooted which will break tf.rpc.
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	}

	if err := f.recoverUnhealthyDUT(ctx, s.DUT(), s); err != nil {
		s.Fatal("Failed to recover unhealthy DUT: ", err)
	}

	// Create TestFixture.
	var ops []TFOption
	// Read router/pcap variable. If not available or empty, NewTestFixture
	// will fall back to Default{Router,Pcap}Host.
	if f.features&TFFeaturesRouters != 0 {
		if routers, ok := s.Var("routers"); ok && routers != "" {
			testing.ContextLog(ctx, "routers: ", routers)
			slice := strings.Split(routers, ",")
			if len(slice) < 2 {
				s.Fatal("Must provide at least two router names when Routers feature is enabled")
			}
			ops = append(ops, TFRouter(slice...))
		} else {
			var routers []string
			for _, suffix := range []string{dut.CompanionSuffixRouter, dut.CompanionSuffixPcap} {
				routers = append(routers, f.companionName(s, suffix))

			}
			testing.ContextLog(ctx, "companion routers: ", routers)
			ops = append(ops, TFRouter(routers...))
		}
	} else {
		router, ok := s.Var("router")
		if ok && router != "" {
			testing.ContextLog(ctx, "router: ", router)
			ops = append(ops, TFRouter(router))
		} // else: let TestFixture resolve the name.
	}
	pcap, ok := s.Var("pcap")
	if ok && pcap != "" {
		testing.ContextLog(ctx, "pcap: ", pcap)
		ops = append(ops, TFPcap(pcap))
	} // else: let TestFixture resolve the name.
	if f.features&TFFeaturesRouterAsCapture != 0 {
		testing.ContextLog(ctx, "using router as pcap")
		ops = append(ops, TFRouterAsCapture())
	}
	// Read attenuator variable.
	if f.features&TFFeaturesAttenuator != 0 {
		atten, ok := s.Var("attenuator")
		if !ok || atten == "" {
			// Attenuator is not typical companion, so we synthesize its name here.
			atten = f.companionName(s, "-attenuator")
		}
		testing.ContextLog(ctx, "attenuator: ", atten)
		ops = append(ops, TFAttenuator(atten))
	}
	// Enable capturing.
	if f.features&TFFeaturesCapture != 0 {
		ops = append(ops, TFCapture(true))
	}

	// Allow for setting router type
	var routerType support.RouterType
	if rTypeStr, ok := s.Var("routertype"); !ok || rTypeStr == "" {
		// Default to unknown so that it may be automatically determined with host
		routerType = support.UnknownT
	} else {
		var err error
		routerType, err = support.ParseRouterType(rTypeStr)
		if err != nil {
			s.Fatalf("Failed to parse routertype %q: ", err)
		}
	}
	testing.ContextLog(ctx, "routertype: ", routerType.String())
	ops = append(ops, TFRouterType(routerType))

	// Allow for setting pcap type
	var pcapType support.RouterType
	if rTypeStr, ok := s.Var("pcaptype"); !ok || rTypeStr == "" {
		// Default to unknown so that it may be automatically determined with host
		pcapType = support.UnknownT
	} else {
		var err error
		pcapType, err = support.ParseRouterType(rTypeStr)
		if err != nil {
			s.Fatalf("Failed to parse pcaptype %q: ", err)
		}
	}
	testing.ContextLog(ctx, "pcaptype: ", pcapType.String())
	ops = append(ops, TFPcapType(pcapType))

	// Read companion DUT.
	if f.features&TFFeaturesCompanionDUT != 0 {
		cd := s.CompanionDUT("cd1")
		if cd == nil {
			s.Fatal("Failed to get companion DUT cd1")
		}
		ops = append(ops, TFRouterRequired(false))
		ops = append(ops, TFCompanionDUT(cd))
		if err := f.recoverUnhealthyDUT(ctx, cd, s); err != nil {
			s.Fatal("Failed to recover unhealthy DUT: ", err)
		}
	}

	tf, err := NewTestFixture(ctx, s.FixtContext(), s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	f.tf = tf

	if f.features&TFFeaturesEnroll != 0 {
		for i := range f.tf.duts {
			if err := f.enrollChrome(ctx, s, i); err != nil {
				s.Fatal("Failed to enroll Chrome: ", err)
			}
		}
	}

	return f.tf
}

func (f *tastFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	duts := f.tf.duts // Make a copy of the slice to iterate over.
	for _, d := range duts {
		if f.features&TFFeaturesEnroll != 0 {
			pc := policy.NewPolicyServiceClient(d.rpc.Conn)

			if _, err := pc.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
				s.Error("Failed to close Chrome instance and Fake DMS: ", err)
			}

			// Reset DUT TPM and system state to leave it in a good state post test.
			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, d.dut, s.RPCHint()); err != nil {
				s.Error("Failed to reset TPM: ", err)
			}
		}
		// Ensure DUT is healthy here again, so that we don't leave with
		// bad state to later tests/tasks.
		if err := f.recoverUnhealthyDUT(ctx, d.dut, s); err != nil {
			s.Fatal("Failed to recover unhealthy DUT: ", err)
		}
	}

	if f.tf == nil {
		return
	}

	if err := f.tf.Close(ctx); err != nil {
		s.Log("Failed to tear down test fixture, err: ", err)
	}
	f.tf = nil
}

func (f *tastFixtureImpl) Reset(ctx context.Context) error {
	if err := f.tf.Reinit(ctx); err != nil {
		return errors.Wrap(err, "failed to reinit test fixture")
	}
	return nil
}

func (f *tastFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// No-op.
}

func (f *tastFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.tf.CollectLogs(ctx); err != nil {
		s.Log("Error collecting logs, err: ", err)
	}
}
