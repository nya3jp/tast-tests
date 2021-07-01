// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// Timeout for methods of Tast fixture.
const (
	// Give long enough timeout for SetUp() and TearDown() as they might need
	// to reboot a broken DUT.
	setUpTimeout    = 6 * time.Minute
	tearDownTimeout = 5 * time.Minute
	resetTimeout    = 10 * time.Second
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
		Vars:            []string{"router", "pcap"},
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
		Vars:            []string{"router", "pcap"},
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
		Vars:            []string{"router", "pcap"},
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
		Vars:            []string{"routers", "pcap"},
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
		Vars:            []string{"routers", "pcap", "attenuator"},
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
	rpcClient, err := rpc.Dial(ctx, d, rpcHint, "cros")
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
func (f *tastFixtureImpl) recoverUnhealthyDUT(ctx context.Context, s *testing.FixtState) error {
	if err := f.dutHealthCheck(ctx, s.DUT(), s.RPCHint()); err != nil {
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
		if err := s.DUT().Reboot(ctx); err != nil {
			return errors.Wrap(err, "reboot failed")
		}
	}
	return nil
}

func (f *tastFixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.recoverUnhealthyDUT(ctx, s); err != nil {
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
	tf, err := NewTestFixture(ctx, s.FixtContext(), s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	f.tf = tf

	return f.tf
}

func (f *tastFixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	// Ensure DUT is healthy here again, so that we don't leave with
	// bad state to later tests/tasks.
	if err := f.recoverUnhealthyDUT(ctx, s); err != nil {
		s.Fatal("Failed to recover unhealthy DUT: ", err)
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
	var firstErr error
	// Light-weight health check here. SetUp/TearDown will try to recover
	// the DUT when anything goes wrong.
	if _, err := f.tf.WifiClient().HealthCheck(ctx, &empty.Empty{}); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, err)
	}
	if err := f.tf.Reinit(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, err)
	}
	return firstErr
}

func (f *tastFixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Nothing to do here for now.
}

func (f *tastFixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.tf.CollectLogs(ctx); err != nil {
		s.Log("Error collecting logs, err: ", err)
	}
}
