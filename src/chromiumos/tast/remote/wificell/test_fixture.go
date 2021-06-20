// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// The allowed packets loss percentage for the ping command.
const pingLossThreshold float64 = 20

// The allowed packets loss percentage for the arping command.
const arpingLossThreshold float64 = 30

// TFOption is the function signature used to modify TextFixutre.
type TFOption func(*TestFixture)

// TFRouter sets the router hostname for the test fixture.
// Format: hostname[:port]
func TFRouter(targets ...string) TFOption {
	return func(tf *TestFixture) {
		tf.routers = make([]routerData, len(targets))
		for i := range targets {
			tf.routers[i].target = targets[i]
		}
	}
}

// TFPcap sets the pcap hostname for the test fixture.
// Format: hostname[:port]
func TFPcap(target string) TFOption {
	return func(tf *TestFixture) {
		tf.pcapTarget = target
	}
}

// TFCapture sets if the test fixture should spawn packet capturer in ConfigureAP.
func TFCapture(b bool) TFOption {
	return func(tf *TestFixture) {
		tf.option.packetCapture = b
	}
}

// TFAttenuator sets the attenuator hostname to use in the test fixture.
func TFAttenuator(target string) TFOption {
	return func(tf *TestFixture) {
		tf.attenuatorTarget = target
	}
}

// TFRouterAsCapture sets if the router should be used as a capturer. If there
// are multiple routers, the first one is used.
func TFRouterAsCapture() TFOption {
	return func(tf *TestFixture) {
		tf.option.routerAsCapture = true
	}
}

// TFWithUI sets if the test fixture should not skip stopping UI.
// This option is useful for tests with UI settings + basic WiFi functionality,
// where the interference of UI (e.g. trigger scans) does not matter much.
func TFWithUI() TFOption {
	return func(tf *TestFixture) {
		tf.option.withUI = true
	}
}

// TFSetLogging sets if wants
func TFSetLogging(b bool) TFOption {
	return func(tf *TestFixture) {
		tf.setLogging = b
	}
}

// TFLogTags sets the logging tags to use in the test fixture.
func TFLogTags(tags []string) TFOption {
	return func(tf *TestFixture) {
		tf.logTags = tags
	}
}

// TFLogLevel sets the logging level to use in the test fixture.
func TFLogLevel(level int) TFOption {
	return func(tf *TestFixture) {
		tf.logLevel = level
	}
}

// TFRouterType sets the router type used in the test fixture.
func TFRouterType(rtype router.Type) TFOption {
	return func(tf *TestFixture) {
		tf.routerType = rtype
	}
}

// TFPcapType sets the router type of the pcap capturing device. The pcap device in our testbeds is a router.
func TFPcapType(rtype router.Type) TFOption {
	return func(tf *TestFixture) {
		tf.pcapType = rtype
	}
}

// TFServiceName is the service needed by TestFixture.
const TFServiceName = "tast.cros.wifi.ShillService"

type routerData struct {
	target string
	host   *ssh.Conn
	object router.Base
}

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	wifiClient wifi.ShillServiceClient

	routers    []routerData
	routerType router.Type
	pcapType   router.Type

	pcapTarget string
	pcapHost   *ssh.Conn
	pcap       router.Base

	attenuatorTarget string
	attenuator       *attenuator.Attenuator

	setLogging       bool
	logLevel         int
	logTags          []string
	originalLogLevel int
	originalLogTags  []string

	// Group simple option flags here as they started to grow.
	option struct {
		packetCapture   bool
		withUI          bool
		routerAsCapture bool
	}

	apID      int
	capturers map[*APIface]*pcap.Capturer

	// aps is a set of APs useful for deconfiguring all APs, which some tests require.
	aps map[*APIface]struct{}

	// netCertStore is initialized lazily in ConnectWifi() when needed because it takes about 7 seconds to set up and only a few tests need it.
	netCertStore *netcertstore.Store
}

// connectCompanion dials SSH connection to companion device with the auth key of DUT.
func (tf *TestFixture) connectCompanion(ctx context.Context, hostname string) (*ssh.Conn, error) {
	var sopt ssh.Options
	ssh.ParseTarget(hostname, &sopt)
	sopt.KeyDir = tf.dut.KeyDir()
	sopt.KeyFile = tf.dut.KeyFile()
	sopt.ConnectTimeout = 10 * time.Second
	return ssh.New(ctx, &sopt)
}

// setupNetCertStore sets up tf.netCertStore for EAP-related tests.
func (tf *TestFixture) setupNetCertStore(ctx context.Context) error {
	if tf.netCertStore != nil {
		// Nothing to do if it was set up.
		return nil
	}

	runner := hwsec.NewCmdRunner(tf.dut)
	var err error
	tf.netCertStore, err = netcertstore.CreateStore(ctx, runner)
	return err
}

// resetNetCertStore nullifies tf.netCertStore.
func (tf *TestFixture) resetNetCertStore(ctx context.Context) error {
	if tf.netCertStore == nil {
		// Nothing to do if it was not set up.
		return nil
	}

	err := tf.netCertStore.Cleanup(ctx)
	tf.netCertStore = nil
	return err
}

// NewTestFixture creates a TestFixture.
// The TestFixture contains a gRPC connection to the DUT and a SSH connection to the router.
// The method takes two context: ctx and daemonCtx, the first one is the context for the operation and
// daemonCtx is for the spawned daemons.
// Noted that if routerHostname is empty, it uses the default router hostname based on the DUT's hostname.
// After the caller gets the TestFixture instance, it should reserve time for Close() the TestFixture:
//   tf, err := NewTestFixture(ctx, ...)
//   if err != nil {...}
//   defer tf.Close(ctx)
//   ctx, cancel := tf.ReserveForClose(ctx)
//   defer cancel()
//   ...
func NewTestFixture(fullCtx, daemonCtx context.Context, d *dut.DUT, rpcHint *testing.RPCHint, ops ...TFOption) (ret *TestFixture, retErr error) {
	fullCtx, st := timing.Start(fullCtx, "NewTestFixture")
	defer st.End()

	tf := &TestFixture{
		dut:       d,
		capturers: make(map[*APIface]*pcap.Capturer),
		aps:       make(map[*APIface]struct{}),
		// Set the router's default router type.
		routerType: router.LegacyT,
		// Set the pcap capture device's default router type.
		pcapType: router.LegacyT,
		// Set the debug values on the DUT by default.
		setLogging: true,
		// Default log level used in WiFi tests.
		logLevel: -2,
		// Default log tags used in WiFi tests. Example of other tags that can be added.
		// (connection + dbus + device + link + manager + portal + service)
		logTags: []string{"wifi"},
	}

	for _, op := range ops {
		op(tf)
	}

	defer func() {
		if retErr != nil {
			tf.Close(fullCtx)
		}
	}()

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	var err error
	tf.rpc, err = rpc.Dial(daemonCtx, tf.dut, rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect rpc")
	}
	tf.wifiClient = wifi.NewShillServiceClient(tf.rpc.Conn)

	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.
	if _, err := tf.wifiClient.InitDUT(ctx, &wifi.InitDUTRequest{WithUi: tf.option.withUI}); err != nil {
		return nil, errors.Wrap(err, "failed to InitDUT")
	}

	if tf.setLogging {
		tf.originalLogLevel, tf.originalLogTags, err = tf.getLoggingConfig(ctx)
		if err != nil {
			return nil, err
		}
		if err := tf.setLoggingConfig(ctx, tf.logLevel, tf.logTags); err != nil {
			return nil, err
		}
	}

	// Wificell precondition always provides us with router name, but we need
	// to handle case when the fixture is created from outside of the precondition.
	if len(tf.routers) == 0 {
		testing.ContextLog(ctx, "Using default router name")
		name, err := tf.dut.CompanionDeviceHostname(dut.CompanionSuffixRouter)
		if err != nil {
			return nil, errors.Wrap(err, "failed to synthesize default router name")
		}
		tf.routers = append(tf.routers, routerData{target: name})
	}
	for i := range tf.routers {
		rt := &tf.routers[i]
		testing.ContextLogf(ctx, "Adding router %s", rt.target)
		routerHost, err := tf.connectCompanion(ctx, rt.target)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to the router %s", rt.target)
		}
		rt.host = routerHost
		routerObj, err := router.NewRouter(ctx, daemonCtx, rt.host,
			strings.ReplaceAll(rt.target, ":", "_"), tf.routerType)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a router object")
		}
		rt.object = routerObj
	}
	if tf.option.routerAsCapture {
		testing.ContextLog(ctx, "Using router as pcap")
		tf.pcapTarget = tf.routers[0].target
	}

	// errInvalidHost checks if the error is a wrapped "no such host" error.
	errInvalidHost := func(err error) bool {
		if err == dut.ErrCompanionHostname {
			return true
		}
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return true
		}
		return false
	}

	useDefaultPcap := false
	if tf.pcapTarget == "" {
		testing.ContextLog(ctx, "Using default pcap name")
		tf.pcapTarget, err = tf.dut.CompanionDeviceHostname(dut.CompanionSuffixPcap)
		if err != nil {
			// DUT might be specified with IP. As the routers are available,
			// fallback to use router as pcap in this case.
			tf.pcapTarget = ""
		} else {
			useDefaultPcap = true
		}
	}

	// Check for pcap duplicates on router list.
	for _, router := range tf.routers {
		// We're checking only hostnames, as these should be autogenerated by precondition.
		// If hostnames are supplied manually, testing lab should guarantee that
		// no two devices names point to the same device.
		// Otherwise we'd need to open a nasty can of worms and e.g. check if two SSH tunnels
		// anchored on our side on different ip/port pairs don't lead to the same device.
		if tf.pcapTarget == router.target {
			testing.ContextLog(ctx, "Supplied pcap name already on router list")
			tf.pcapHost = router.host
			tf.pcap = router.object
		}
	}

	// If pcap name is available and unique, try to connect it.
	if tf.pcapHost == nil && tf.pcapTarget != "" {
		tf.pcapHost, err = tf.connectCompanion(ctx, tf.pcapTarget)
		if err != nil {
			// We want to fallback to use router as pcap iff the default
			// pcap hostname is invalid. Fail here if it's not the case.
			if !useDefaultPcap || !errInvalidHost(err) {
				return nil, errors.Wrap(err, "failed to connect to pcap")
			}
		} else {
			tf.pcap, err = router.NewRouter(ctx, daemonCtx, tf.pcapHost, "pcap", tf.pcapType)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create a router object for pcap")
			}
		}
	}

	// Finally, fallback to use the first router as pcap if needed.
	if tf.pcapHost == nil {
		testing.ContextLog(ctx, "Fallback to use router 0 as pcap")
		tf.pcapHost = tf.routers[0].host
		tf.pcap = tf.routers[0].object
	}

	if tf.attenuatorTarget != "" {
		testing.ContextLog(ctx, "Opening Attenuator: ", tf.attenuatorTarget)
		// Router #0 should always be present, thus we use it as a proxy.
		tf.attenuator, err = attenuator.Open(ctx, tf.attenuatorTarget, tf.routers[0].host)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open attenuator")
		}
	}

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, nil
}

// ReserveForClose returns a shorter ctx and cancel function for tf.Close().
func (tf *TestFixture) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}

// CollectLogs downloads related log files to OutDir.
func (tf *TestFixture) CollectLogs(ctx context.Context) error {
	var firstErr error
	for _, rt := range tf.routers {
		// Assert router can collect logs
		obj, ok := rt.object.(router.SupportLogs)
		if !ok {
			return errors.New("Router does not support log collection")
		}
		err := obj.CollectLogs(ctx)
		if err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect logs"))
		}
	}
	return firstErr
}

// ReserveForCollectLogs returns a shorter ctx and cancel function for tf.CollectLogs.
func (tf *TestFixture) ReserveForCollectLogs(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, time.Second)
}

// Close closes the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "tf.Close")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var firstErr error
	if err := tf.resetNetCertStore(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to reset the NetCertStore"))
	}

	if tf.attenuator != nil {
		tf.attenuator.Close()
		tf.attenuator = nil
	}

	// Check if one of routers was used in dual-purpose (router&pcap) mode.
	if tf.pcap != nil {
		for i := range tf.routers {
			rt := &tf.routers[i]
			if tf.pcap == rt.object {
				// Don't close it, it will be closed while handling routers.
				tf.pcap = nil
				tf.pcapHost = nil
				break
			}
		}
	}
	// If pcap was created specifically for this purpose, close it.
	if tf.pcap != nil {
		if err := tf.pcap.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap"))
		}
		if err := tf.pcapHost.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap ssh"))
		}
		tf.pcap = nil
	}
	// Close all created routers.
	for i := range tf.routers {
		router := &tf.routers[i]
		if router.object != nil {
			if err := router.object.Close(ctx); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close rotuer %s", router.target))
			}
		}
		router.object = nil
		if router.host != nil {
			if err := router.host.Close(ctx); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close router %s ssh", router.target))
			}
		}
		router.host = nil
	}
	if tf.wifiClient != nil {
		if tf.setLogging {
			if err := tf.setLoggingConfig(ctx, tf.originalLogLevel, tf.originalLogTags); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
			}
		}
		if _, err := tf.wifiClient.TearDown(ctx, &empty.Empty{}); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
		}
		tf.wifiClient = nil
	}
	if tf.rpc != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		tf.rpc.Close(ctx)
		tf.rpc = nil
	}
	// Do not close DUT, it'll be closed by the framework.
	return firstErr
}

// Reinit reinitialize the TestFixture. This can be used in precondition or between
// testcases to guarantee a cleaner state.
func (tf *TestFixture) Reinit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := tf.WifiClient().ReinitTestState(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reinit DUT")
	}
	return nil
}

// getUniqueAPName returns an unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func (tf *TestFixture) getUniqueAPName() string {
	id := strconv.Itoa(tf.apID)
	tf.apID++
	return id
}

// ConfigureAPOnRouterID is an extended version of ConfigureAP, allowing to chose router
// to establish the AP on.
func (tf *TestFixture) ConfigureAPOnRouterID(ctx context.Context, idx int, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
	ctx, st := timing.Start(ctx, "tf.ConfigureAP")
	defer st.End()
	r, ok := tf.routers[idx].object.(router.LegacyOpenWrtShared)
	if !ok {
		return nil, errors.Errorf("router device of type %v does not have legacy/openwrt support", tf.routers[idx].object.GetRouterType())
	}

	name := tf.getUniqueAPName()

	if fac != nil {
		// Defer the securityConfig generation from test's init() to here because the step may emit error and that's not allowed in test init().
		securityConfig, err := fac.Gen()
		if err != nil {
			return nil, err
		}
		ops = append([]hostapd.Option{hostapd.SecurityConfig(securityConfig)}, ops...)
	}
	config, err := hostapd.NewConfig(ops...)
	if err != nil {
		return nil, err
	}

	if len(tf.routers) <= idx {
		return nil, errors.Errorf("Router index (%d) out of range [0, %d)", idx, len(tf.routers))
	}

	var capturer *pcap.Capturer
	if tf.option.packetCapture {
		freqOps, err := config.PcapFreqOptions()
		if err != nil {
			return nil, err
		}
		p, ok := tf.pcap.(router.SupportCapture)
		if !ok {
			return nil, errors.Errorf("pcap device of type %v does not have log capture support", tf.pcap.GetRouterType())
		}
		capturer, err = p.StartCapture(ctx, name, config.Channel, freqOps)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		defer func() {
			if retErr != nil {
				p.StopCapture(ctx, capturer)
			}
		}()
	}

	ap, err := StartAPIface(ctx, r, name, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start APIface")
	}
	tf.aps[ap] = struct{}{}

	if capturer != nil {
		tf.capturers[ap] = capturer
	}

	return ap, nil
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
// Note that after getting an APIface, ap, the caller should defer tf.DeconfigAP(ctx, ap) and
// use tf.ReserveForClose(ctx, ap) to reserve time for the deferred call.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
	return tf.ConfigureAPOnRouterID(ctx, 0, ops, fac)
}

// ReserveForDeconfigAP returns a shorter ctx and cancel function for tf.DeconfigAP().
func (tf *TestFixture) ReserveForDeconfigAP(ctx context.Context, ap *APIface) (context.Context, context.CancelFunc) {
	if len(tf.routers) == 0 {
		return ctx, func() {}
	}

	ctx, cancel := ap.ReserveForStop(ctx)
	if capturer, ok := tf.capturers[ap]; ok {
		// Stop the call if the router does not support ResrveForStopCapture.
		if _, ok := tf.pcap.(router.SupportCapture); !ok {
			return ctx, func() {}
		}
		// Also reserve time for stopping the capturer if it exists.
		// Noted that CancelFunc returned here is dropped as we rely on its
		// parent's cancel() being called.
		if p, ok := tf.pcap.(router.SupportCapture); ok {
			ctx, _ = p.ReserveForStopCapture(ctx, capturer)
		} else {
			return ctx, func() {}
		}

	}
	return ctx, cancel
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *APIface) error {
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()
	p, ok := tf.pcap.(router.SupportCapture)
	if !ok {
		return errors.Errorf("pcap device of type %v does not have log capture support", tf.pcap.GetRouterType())
	}
	var firstErr error

	capturer := tf.capturers[ap]
	delete(tf.capturers, ap)
	if err := ap.Stop(ctx); err != nil {
		wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop APIface"))
	}
	if capturer != nil {
		if err := p.StopCapture(ctx, capturer); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
		}
	}
	delete(tf.aps, ap)
	return firstErr
}

// DeconfigAllAPs facilitates deconfiguration of all APs established for
// this test fixture.
func (tf *TestFixture) DeconfigAllAPs(ctx context.Context) error {
	var firstErr error
	for ap := range tf.aps {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to deconfig AP"))
		}
	}
	return firstErr
}

const wpaMonitorStopTimeout = 10 * time.Second

// StartWPAMonitor configures and starts wpa_supplicant events monitor
// newCtx is ctx shortened for the stop function, which should be deferred by the caller.
func (tf *TestFixture) StartWPAMonitor(ctx context.Context) (wpaMonitor *WPAMonitor, stop func(), newCtx context.Context, retErr error) {
	wpaMonitor = new(WPAMonitor)
	if err := wpaMonitor.Start(ctx, tf.dut.Conn()); err != nil {
		return nil, nil, ctx, err
	}
	sCtx, sCancel := ctxutil.Shorten(ctx, wpaMonitorStopTimeout)
	return wpaMonitor, func() {
		sCancel()
		timeoutCtx, cancel := context.WithTimeout(ctx, wpaMonitorStopTimeout)
		defer cancel()
		if err := wpaMonitor.Stop(timeoutCtx); err != nil {
			testing.ContextLog(ctx, "Failed to wait for wpa monitor stop: ", err)
		}
		testing.ContextLog(ctx, "Wpa monitor stopped")
	}, sCtx, nil
}

// Capturer returns the auto-spawned Capturer for the APIface instance.
func (tf *TestFixture) Capturer(ap *APIface) (*pcap.Capturer, bool) {
	capturer, ok := tf.capturers[ap]
	return capturer, ok
}

type connConfig struct {
	ssid    string
	hidden  bool
	secConf security.Config
	props   map[string]interface{}
}

// ConnOption is the function signature used to modify ConnectWifi.
type ConnOption func(*connConfig)

// ConnHidden returns a ConnOption which sets the hidden property.
func ConnHidden(h bool) ConnOption {
	return func(c *connConfig) {
		c.hidden = h
	}
}

// ConnSecurity returns a ConnOption which sets the security configuration.
func ConnSecurity(s security.Config) ConnOption {
	return func(c *connConfig) {
		c.secConf = s
	}
}

// ConnProperties returns a ConnOption which sets the service properties.
func ConnProperties(p map[string]interface{}) ConnOption {
	return func(c *connConfig) {
		c.props = make(map[string]interface{})
		for k, v := range p {
			c.props[k] = v
		}
	}
}

// ConnectWifi asks the DUT to connect to the specified WiFi.
func (tf *TestFixture) ConnectWifi(ctx context.Context, ssid string, options ...ConnOption) (*wifi.ConnectResponse, error) {
	c := &connConfig{
		ssid:    ssid,
		secConf: &base.Config{},
	}
	for _, op := range options {
		op(c)
	}
	ctx, st := timing.Start(ctx, "tf.ConnectWifi")
	defer st.End()

	// Setup the NetCertStore only for EAP-related tests.
	if c.secConf.NeedsNetCertStore() {
		if err := tf.setupNetCertStore(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to set up the NetCertStore")
		}

		if err := c.secConf.InstallClientCredentials(ctx, tf.netCertStore); err != nil {
			return nil, errors.Wrap(err, "failed to install client credentials")
		}
	}

	secProps, err := c.secConf.ShillServiceProperties()
	if err != nil {
		return nil, err
	}

	props := make(map[string]interface{})
	for k, v := range c.props {
		props[k] = v
	}
	for k, v := range secProps {
		props[k] = v
	}

	propsEnc, err := protoutil.EncodeToShillValMap(props)
	if err != nil {
		return nil, err
	}
	request := &wifi.ConnectRequest{
		Ssid:       []byte(c.ssid),
		Hidden:     c.hidden,
		Security:   c.secConf.Class(),
		Shillprops: propsEnc,
	}
	response, err := tf.wifiClient.Connect(ctx, request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DiscoverBSSID discovers a service with the given properties.
func (tf *TestFixture) DiscoverBSSID(ctx context.Context, bssid, iface string, ssid []byte) error {
	ctx, st := timing.Start(ctx, "tf.DiscoverBSSID")
	defer st.End()
	request := &wifi.DiscoverBSSIDRequest{
		Bssid:     bssid,
		Interface: iface,
		Ssid:      ssid,
	}
	if _, err := tf.wifiClient.DiscoverBSSID(ctx, request); err != nil {
		return err
	}

	return nil
}

// RequestRoam requests DUT to roam to the specified BSSID and waits until the DUT has roamed.
func (tf *TestFixture) RequestRoam(ctx context.Context, iface, bssid string, timeout time.Duration) error {
	request := &wifi.RequestRoamRequest{
		InterfaceName: iface,
		Bssid:         bssid,
		Timeout:       timeout.Nanoseconds(),
	}
	if _, err := tf.wifiClient.RequestRoam(ctx, request); err != nil {
		return err
	}

	return nil
}

// Reassociate triggers reassociation with the current AP and waits until it has reconnected or the timeout expires.
func (tf *TestFixture) Reassociate(ctx context.Context, iface string, timeout time.Duration) error {
	_, err := tf.wifiClient.Reassociate(ctx, &wifi.ReassociateRequest{
		InterfaceName: iface,
		Timeout:       timeout.Nanoseconds(),
	})
	return err
}

// FlushBSS flushes BSS entries over the specified age from wpa_supplicant's cache.
func (tf *TestFixture) FlushBSS(ctx context.Context, iface string, age time.Duration) error {
	req := &wifi.FlushBSSRequest{
		InterfaceName: iface,
		Age:           age.Nanoseconds(),
	}
	_, err := tf.wifiClient.FlushBSS(ctx, req)
	return err
}

// ConnectWifiAP asks the DUT to connect to the WiFi provided by the given AP.
func (tf *TestFixture) ConnectWifiAP(ctx context.Context, ap *APIface, options ...ConnOption) (*wifi.ConnectResponse, error) {
	conf := ap.Config()
	opts := append([]ConnOption{ConnHidden(conf.Hidden), ConnSecurity(conf.SecurityConfig)}, options...)
	return tf.ConnectWifi(ctx, conf.SSID, opts...)
}

func (tf *TestFixture) disconnectWifi(ctx context.Context, removeProfile bool) error {
	ctx, st := timing.Start(ctx, "tf.disconnectWifi")
	defer st.End()

	resp, err := tf.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get selected service")
	}

	// Note: It is possible that selected service changed after SelectService call,
	// but we are usually in a stable state when calling this. If not, the Disconnect
	// call will also fail and caller usually leaves hint for this.
	// In Close and Reinit, we pop + remove related profiles so it should still be
	// safe for next test if this case happened in clean up.
	req := &wifi.DisconnectRequest{
		ServicePath:   resp.ServicePath,
		RemoveProfile: removeProfile,
	}
	if _, err := tf.wifiClient.Disconnect(ctx, req); err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}

// DisconnectWifi asks the DUT to disconnect from current WiFi service.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	return tf.disconnectWifi(ctx, false)
}

// CleanDisconnectWifi asks the DUT to disconnect from current WiFi service and removes the configuration.
func (tf *TestFixture) CleanDisconnectWifi(ctx context.Context) error {
	return tf.disconnectWifi(ctx, true)
}

// ReserveForDisconnect returns a shorter ctx and cancel function for tf.DisconnectWifi.
func (tf *TestFixture) ReserveForDisconnect(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

// AssureDisconnect assures that the WiFi service has disconnected within timeout.
func (tf *TestFixture) AssureDisconnect(ctx context.Context, servicePath string, timeout time.Duration) error {
	req := &wifi.AssureDisconnectRequest{
		ServicePath: servicePath,
		Timeout:     timeout.Nanoseconds(),
	}
	if _, err := tf.wifiClient.AssureDisconnect(ctx, req); err != nil {
		return err
	}
	return nil
}

// QueryService queries shill information of selected service.
func (tf *TestFixture) QueryService(ctx context.Context) (*wifi.QueryServiceResponse, error) {
	selectedSvcResp, err := tf.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get selected service")
	}

	req := &wifi.QueryServiceRequest{
		Path: selectedSvcResp.ServicePath,
	}
	resp, err := tf.wifiClient.QueryService(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the service information")
	}

	return resp, nil
}

// PingFromDUT tests the connectivity between DUT and target IP.
func (tf *TestFixture) PingFromDUT(ctx context.Context, targetIP string, opts ...ping.Option) error {
	ctx, st := timing.Start(ctx, "tf.PingFromDUT")
	defer st.End()

	pr := remoteping.NewRemoteRunner(tf.dut.Conn())
	res, err := pr.Ping(ctx, targetIP, opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)

	if res.Loss > pingLossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	return nil
}

// PingFromRouterID tests the connectivity between DUT and router through currently connected WiFi service.
func (tf *TestFixture) PingFromRouterID(ctx context.Context, idx int, opts ...ping.Option) error {
	ctx, st := timing.Start(ctx, "tf.PingFromServer")
	defer st.End()

	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		return errors.Wrap(err, "failed to get the IP address")
	}

	pr := remoteping.NewRemoteRunner(tf.routers[idx].host)
	res, err := pr.Ping(ctx, addrs[0].String(), opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)

	if res.Loss > pingLossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	return nil
}

// PingFromServer calls PingFromRouterID for router 0.
// Kept for backwards-compatibility.
func (tf *TestFixture) PingFromServer(ctx context.Context, opts ...ping.Option) error {
	return tf.PingFromRouterID(ctx, 0, opts...)
}

// ArpingFromDUT tests that DUT can send the broadcast packets to server.
func (tf *TestFixture) ArpingFromDUT(ctx context.Context, serverIP string, ops ...arping.Option) error {
	ctx, st := timing.Start(ctx, "tf.ArpingFromDUT")
	defer st.End()

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the client WiFi interface")
	}

	runner := remotearping.NewRemoteRunner(tf.dut.Conn())
	res, err := runner.Arping(ctx, serverIP, iface, ops...)
	if err != nil {
		return errors.Wrap(err, "arping failed")
	}

	if res.Loss > arpingLossThreshold {
		return errors.Errorf("unexpected arping loss percentage: got %g%% want <= %g%%", res.Loss, arpingLossThreshold)
	}

	return nil
}

// ArpingFromRouterID tests that DUT can receive the broadcast packets from server.
func (tf *TestFixture) ArpingFromRouterID(ctx context.Context, idx int, serverIface string, ops ...arping.Option) error {
	ctx, st := timing.Start(ctx, "tf.ArpingFromServer")
	defer st.End()

	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		return errors.Wrap(err, "failed to get the IP address")
	}

	runner := remotearping.NewRemoteRunner(tf.routers[idx].host)
	res, err := runner.Arping(ctx, addrs[0].String(), serverIface, ops...)
	if err != nil {
		return errors.Wrap(err, "arping failed")
	}

	if res.Loss > arpingLossThreshold {
		return errors.Errorf("unexpected arping loss percentage: got %g%% want <= %g%%", res.Loss, arpingLossThreshold)
	}

	return nil
}

// ArpingFromServer tests that DUT can receive the broadcast packets from server.
// Kept for backwards-compatibility.
func (tf *TestFixture) ArpingFromServer(ctx context.Context, serverIface string, ops ...arping.Option) error {
	return tf.ArpingFromRouterID(ctx, 0, serverIface, ops...)
}

// ClientIPv4Addrs returns the IPv4 addresses for the network interface.
func (tf *TestFixture) ClientIPv4Addrs(ctx context.Context) ([]net.IP, error) {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetIPv4AddrsRequest{
		InterfaceName: iface,
	}
	addrs, err := tf.WifiClient().GetIPv4Addrs(ctx, netIface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the IPv4 addresses")
	}

	ret := make([]net.IP, len(addrs.Ipv4))
	for i, a := range addrs.Ipv4 {
		ret[i], _, err = net.ParseCIDR(a)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse IP address %s", a)
		}
	}

	return ret, nil
}

// ClientHardwareAddr returns the HardwareAddr for the network interface.
func (tf *TestFixture) ClientHardwareAddr(ctx context.Context) (string, error) {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return "", errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetHardwareAddrRequest{
		InterfaceName: iface,
	}
	resp, err := tf.WifiClient().GetHardwareAddr(ctx, netIface)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the HardwareAddr")
	}

	return resp.HwAddr, nil
}

// AssertNoDisconnect runs the given routine and verifies that no disconnection event
// is captured in the same duration.
func (tf *TestFixture) AssertNoDisconnect(ctx context.Context, f func(context.Context) error) error {
	ctx, st := timing.Start(ctx, "tf.AssertNoDisconnect")
	defer st.End()

	el, err := iw.NewEventLogger(ctx, tf.dut)
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventLogger")
	}
	errf := f(ctx)
	if err := el.Stop(ctx); err != nil {
		// Let's also keep errf if available. Wrapf is equivalent to Errorf when errf==nil.
		return errors.Wrapf(errf, "failed to stop iw.EventLogger, err=%s", err.Error())
	}
	if errf != nil {
		return errf
	}
	evs := el.EventsByType(iw.EventTypeDisconnect)
	if len(evs) != 0 {
		return errors.Errorf("disconnect events captured: %v", evs)
	}
	return nil
}

// RouterByID returns the respective Router object in the fixture.
func (tf *TestFixture) RouterByID(idx int) router.Base {
	return tf.routers[idx].object
}

// LegacyRouter returns Router 0 object in the fixture.
func (tf *TestFixture) LegacyRouter() (router.Legacy, error) {
	r, ok := tf.RouterByID(0).(router.Legacy)
	if !ok {
		return nil, errors.New("router is not a legacy router")
	}
	return r, nil

}

// Router returns Router 0 object in the fixture.
func (tf *TestFixture) Router() router.Base {
	return tf.RouterByID(0)
}

// LegacyPcap returns the pcap Router object in the fixture.
func (tf *TestFixture) LegacyPcap() (router.Legacy, error) {
	p, ok := tf.pcap.(router.Legacy)
	if !ok {
		return nil, errors.New("pcap is not a legacy pcap device")
	}
	return p, nil
}

// Pcap returns the pcap Router object in the fixture.
func (tf *TestFixture) Pcap() router.Base {
	return tf.pcap
}

// Attenuator returns the Attenuator object in the fixture.
func (tf *TestFixture) Attenuator() *attenuator.Attenuator {
	return tf.attenuator
}

// WifiClient returns the gRPC ShillServiceClient of the DUT.
func (tf *TestFixture) WifiClient() wifi.ShillServiceClient {
	return tf.wifiClient
}

// DefaultOpenNetworkAPOptions returns the Options for an common 802.11n open wifi.
// The function is useful to allow common logic shared between the default setting
// and customized setting.
func DefaultOpenNetworkAPOptions() []hostapd.Option {
	return []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
}

// DefaultOpenNetworkAP configures the router to provide an 802.11n open wifi.
func (tf *TestFixture) DefaultOpenNetworkAP(ctx context.Context) (*APIface, error) {
	return tf.ConfigureAP(ctx, DefaultOpenNetworkAPOptions(), nil)
}

// ClientInterface returns the client interface name.
func (tf *TestFixture) ClientInterface(ctx context.Context) (string, error) {
	netIf, err := tf.wifiClient.GetInterface(ctx, &empty.Empty{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return netIf.Name, nil
}

// VerifyConnection verifies that the AP is reachable by pinging, and we have the same frequency and subnet as AP's.
func (tf *TestFixture) VerifyConnection(ctx context.Context, ap *APIface) error {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get interface from the DUT")
	}

	// Check frequency.
	service, err := tf.QueryService(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query shill service information")
	}
	clientFreq := service.Wifi.Frequency
	serverFreq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		return errors.Wrap(err, "failed to get server frequency")
	}
	if clientFreq != uint32(serverFreq) {
		return errors.Errorf("frequency does not match, got %d want %d", clientFreq, serverFreq)
	}

	// Check subnet.
	addrs, err := tf.WifiClient().GetIPv4Addrs(ctx, &wifi.GetIPv4AddrsRequest{InterfaceName: iface})
	if err != nil {
		return errors.Wrap(err, "failed to get client ipv4 addresses")
	}
	serverSubnet := ap.ServerSubnet().String()
	foundSubnet := false
	for _, a := range addrs.Ipv4 {
		_, ipnet, err := net.ParseCIDR(a)
		if err != nil {
			return errors.Wrapf(err, "failed to parse IP address %s", a)
		}
		if ipnet.String() == serverSubnet {
			foundSubnet = true
			break
		}
	}
	if !foundSubnet {
		return errors.Errorf("subnet does not match, got addrs %v want %s", addrs.Ipv4, serverSubnet)
	}

	// Perform ping.
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		return errors.Wrap(err, "failed to ping from the DUT")
	}

	return nil
}

// CurrentClientTime returns the current time on DUT.
func (tf *TestFixture) CurrentClientTime(ctx context.Context) (time.Time, error) {
	res, err := tf.WifiClient().GetCurrentTime(ctx, &empty.Empty{})
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to get the current DUT time")
	}
	currentTime := time.Unix(res.NowSecond, res.NowNanosecond)
	return currentTime, nil
}

// ClearBSSIDIgnoreDUT clears the BSSID_IGNORE list on DUT.
func (tf *TestFixture) ClearBSSIDIgnoreDUT(ctx context.Context) error {
	wpa := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: tf.dut.Conn()})

	err := wpa.ClearBSSIDIgnore(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to clear WPA BSSID_IGNORE")
	}

	return nil
}

// ShillProperty holds a shill service property with it's expected and unexpected values.
type ShillProperty struct {
	Property         string
	ExpectedValues   []interface{}
	UnexpectedValues []interface{}
	Method           wifi.ExpectShillPropertyRequest_CheckMethod
}

// ExpectShillProperty is a wrapper for the streaming gRPC call ExpectShillProperty.
// It takes an array of ShillProperty, an array of shill properties to monitor, and
// a shill service path. It returns a function that waites for the expected property
// changes and returns the monitor results.
func (tf *TestFixture) ExpectShillProperty(ctx context.Context, objectPath string, props []*ShillProperty, monitorProps []string) (func() ([]protoutil.ShillPropertyHolder, error), error) {
	var expectedProps []*wifi.ExpectShillPropertyRequest_Criterion
	for _, prop := range props {
		var anyOfVals []*wifi.ShillVal
		for _, shillState := range prop.ExpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			anyOfVals = append(anyOfVals, state)
		}

		var noneOfVals []*wifi.ShillVal
		for _, shillState := range prop.UnexpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			noneOfVals = append(noneOfVals, state)
		}

		shillPropReqCriterion := &wifi.ExpectShillPropertyRequest_Criterion{
			Key:    prop.Property,
			AnyOf:  anyOfVals,
			NoneOf: noneOfVals,
			Method: prop.Method,
		}
		expectedProps = append(expectedProps, shillPropReqCriterion)
	}

	req := &wifi.ExpectShillPropertyRequest{
		ObjectPath:   objectPath,
		Props:        expectedProps,
		MonitorProps: monitorProps,
	}

	stream, err := tf.WifiClient().ExpectShillProperty(ctx, req)
	if err != nil {
		return nil, err
	}

	ready, err := stream.Recv()
	if err != nil || ready.Key != "" {
		// Error due to expecting an empty response as ready signal.
		return nil, errors.New("failed to get the ready signal")
	}

	// Get the expected properties and values.
	waitForProperties := func() ([]protoutil.ShillPropertyHolder, error) {
		for {
			resp, err := stream.Recv()
			if err != nil {
				return nil, errors.Wrap(err, "failed to get the expected properties")
			}

			if resp.MonitorDone {
				return protoutil.DecodeFromShillPropertyChangedSignalList(resp.Props)
			}

			// Now we get the matched state change in resp.
			stateVal, err := protoutil.FromShillVal(resp.Val)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			testing.ContextLogf(ctx, "The current WiFi service %s: %v", resp.Key, stateVal)
		}
	}

	return waitForProperties, nil
}

// EAPAuthSkipped is a wrapper for the streaming gRPC call EAPAuthSkipped.
// It returns a function that waits and verifies the EAP authentication is skipped or not in the next connection.
func (tf *TestFixture) EAPAuthSkipped(ctx context.Context) (func() (bool, error), error) {
	recv, err := tf.WifiClient().EAPAuthSkipped(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	s, err := recv.Recv()
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive ready signal from EAPAuthSkipped")
	}
	if s.Skipped {
		return nil, errors.New("unexpected ready signal: got true, want false")
	}
	return func() (bool, error) {
		resp, err := recv.Recv()
		if err != nil {
			return false, errors.Wrap(err, "failed to receive from EAPAuthSkipped")
		}
		return resp.Skipped, nil
	}, nil
}

// DisconnectReason is a wrapper for the streaming gRPC call DisconnectReason.
// It returns a function that waits for the wpa_supplicant DisconnectReason
// property change, and returns the disconnection reason code.
func (tf *TestFixture) DisconnectReason(ctx context.Context) (func() (int32, error), error) {
	recv, err := tf.WifiClient().DisconnectReason(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	ready, err := recv.Recv()
	if err != nil || ready.Reason != 0 {
		// Error due to expecting an empty response as ready signal.
		return nil, errors.New("failed to get the ready signal")
	}
	return func() (int32, error) {
		resp, err := recv.Recv()
		if err != nil {
			return 0, errors.Wrap(err, "failed to receive from DisconnectReason")
		}
		return resp.Reason, nil
	}, nil
}

// SuspendAssertConnect suspends the DUT for wakeUpTimeout seconds through gRPC and returns the duration from resume to connect.
func (tf *TestFixture) SuspendAssertConnect(ctx context.Context, wakeUpTimeout time.Duration) (time.Duration, error) {
	service, err := tf.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get selected service")
	}
	resp, err := tf.wifiClient.SuspendAssertConnect(ctx, &wifi.SuspendAssertConnectRequest{
		WakeUpTimeout: wakeUpTimeout.Nanoseconds(),
		ServicePath:   service.ServicePath,
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to suspend and assert connection")
	}
	return time.Duration(resp.ReconnectTime), nil
}

// Suspend suspends the DUT for wakeUpTimeout seconds through gRPC.
// This call will fail when the DUT wake up early. If the caller expects the DUT to
// wake up early, please use the Suspend gRPC to specify the detailed options.
func (tf *TestFixture) Suspend(ctx context.Context, wakeUpTimeout time.Duration) error {
	req := &wifi.SuspendRequest{
		WakeUpTimeout:  wakeUpTimeout.Nanoseconds(),
		CheckEarlyWake: true,
	}
	_, err := tf.wifiClient.Suspend(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to suspend")
	}
	return nil
}

// DisableMACRandomize disables MAC randomization on DUT if supported, this
// is useful for tests verifying probe requests from DUT.
// On success, a shortened context and cleanup function is returned.
func (tf *TestFixture) DisableMACRandomize(ctx context.Context) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	// If MAC randomization setting is supported, disable MAC randomization
	// as we're filtering the packets with MAC address.
	if supResp, err := tf.WifiClient().MACRandomizeSupport(ctx, &empty.Empty{}); err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get if MAC randomization is supported")
	} else if supResp.Supported {
		resp, err := tf.WifiClient().GetMACRandomize(ctx, &empty.Empty{})
		if err != nil {
			return ctx, nil, errors.Wrap(err, "failed to get MAC randomization setting")
		}
		if resp.Enabled {
			ctxRestore := ctx
			ctx, cancel := ctxutil.Shorten(ctx, time.Second)
			_, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: false})
			if err != nil {
				return ctx, nil, errors.Wrap(err, "failed to disable MAC randomization")
			}
			// Restore the setting when exiting.
			restore := func() error {
				cancel()
				if _, err := tf.WifiClient().SetMACRandomize(ctxRestore, &wifi.SetMACRandomizeRequest{Enable: true}); err != nil {
					return errors.Wrap(err, "failed to re-enable MAC randomization")
				}
				return nil
			}
			return ctx, restore, nil
		}
	}
	// Not supported or not enabled. No-op for these cases.
	return ctx, func() error { return nil }, nil
}

// SetWifiEnabled persistently enables/disables Wifi via shill.
func (tf *TestFixture) SetWifiEnabled(ctx context.Context, enabled bool) error {
	req := &wifi.SetWifiEnabledRequest{Enabled: enabled}
	_, err := tf.WifiClient().SetWifiEnabled(ctx, req)
	return err
}

// TurnOffBgscan turns off the DUT's background scan, and returns a shortened ctx and a restoring function.
func (tf *TestFixture) TurnOffBgscan(ctx context.Context) (context.Context, func() error, error) {
	ctxForRestoreBgConfig := ctx
	ctx, cancel := ctxutil.Shorten(ctxForRestoreBgConfig, 2*time.Second)

	testing.ContextLog(ctx, "Disable the DUT's background scan")
	bgscanResp, err := tf.wifiClient.GetBgscanConfig(ctx, &empty.Empty{})
	if err != nil {
		return ctxForRestoreBgConfig, nil, err
	}
	oldBgConfig := bgscanResp.Config

	turnOffBgConfig := *bgscanResp.Config
	turnOffBgConfig.Method = shillconst.DeviceBgscanMethodNone
	if _, err := tf.wifiClient.SetBgscanConfig(ctx, &wifi.SetBgscanConfigRequest{Config: &turnOffBgConfig}); err != nil {
		return ctxForRestoreBgConfig, nil, err
	}

	return ctx, func() error {
		cancel()
		testing.ContextLog(ctxForRestoreBgConfig, "Restore the DUT's background scan config: ", oldBgConfig)
		_, err := tf.wifiClient.SetBgscanConfig(ctxForRestoreBgConfig, &wifi.SetBgscanConfigRequest{Config: oldBgConfig})
		return err
	}, nil
}

// SendChannelSwitchAnnouncement sends a CSA frame and waits for Client_Disconnection, or Channel_Switch event.
func (tf *TestFixture) SendChannelSwitchAnnouncement(ctx context.Context, ap *APIface, maxRetry, alternateChannel int) error {
	ctxForCloseFrameSender := ctx
	r, ok := tf.Router().(router.SupportFrameSender)
	if !ok {
		return errors.Errorf("router device of type %v does not have management frame support", tf.Router().GetRouterType())
	}
	ctx, cancel := r.ReserveForCloseFrameSender(ctx)
	defer cancel()
	sender, err := r.NewFrameSender(ctx, ap.Interface())
	if err != nil {
		return errors.Wrap(err, "failed to create frame sender")
	}
	defer func(ctx context.Context) error {
		if err := r.CloseFrameSender(ctx, sender); err != nil {
			return errors.Wrap(err, "failed to close frame sender")
		}
		return nil
	}(ctxForCloseFrameSender)

	ew, err := iw.NewEventWatcher(ctx, tf.dut)
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventWatcher")
	}
	defer ew.Stop(ctx)

	// Action frame might be lost, give it some retries.
	for i := 0; i < maxRetry; i++ {
		testing.ContextLogf(ctx, "Try sending channel switch frame %d", i)
		if err := sender.Send(ctx, framesender.TypeChannelSwitch, alternateChannel); err != nil {
			return errors.Wrap(err, "failed to send channel switch frame")
		}
		// The frame might need some time to reach DUT, wait for a few seconds.
		wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		// TODO(b/154879577): Find some way to know if DUT supports
		// channel switch, and only wait for the proper event.
		_, err := ew.WaitByType(wCtx, iw.EventTypeChanSwitch, iw.EventTypeDisconnect)
		if err == context.DeadlineExceeded {
			// Retry if deadline exceeded.
			continue
		}
		if err != nil {
			return errors.Wrap(err, "failed to wait for iw event")
		}
		// Channel switch or client disconnection detected.
		return nil
	}

	return errors.New("failed to disconnect client or switch channel")
}

// DisablePowersaveMode disables power saving mode (if it's enabled) and return a function to restore it's initial mode.
func (tf *TestFixture) DisablePowersaveMode(ctx context.Context) (shortenCtx context.Context, restore func() error, err error) {
	iwr := iw.NewRemoteRunner(tf.dut.Conn())
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get the client interface")
	}

	ctxForResetingPowersaveMode := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get the powersave mode")
	}
	if psMode {
		restore := func() error {
			cancel()
			testing.ContextLogf(ctxForResetingPowersaveMode, "Restoring power save mode to %t", psMode)
			if err := iwr.SetPowersaveMode(ctxForResetingPowersaveMode, iface, psMode); err != nil {
				return errors.Wrapf(err, "failed to restore powersave mode to %t", psMode)
			}
			return nil
		}
		testing.ContextLog(ctx, "Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			return ctx, nil, errors.Wrap(err, "failed to turn off powersave")
		}
		return ctx, restore, nil
	}

	// Power saving mode already disabled.
	return ctxForResetingPowersaveMode, func() error { return nil }, nil
}

// setLoggingConfig configures the logging setting with the specified values (level and tags).
func (tf *TestFixture) setLoggingConfig(ctx context.Context, level int, tags []string) error {
	testing.ContextLogf(ctx, "Configuring logging level: %d, tags: %v", level, tags)
	_, err := tf.wifiClient.SetLoggingConfig(ctx, &wifi.SetLoggingConfigRequest{DebugLevel: int32(level), DebugTags: tags})
	return err
}

// getLoggingConfig returns the current DUT's logging setting (level and tags).
func (tf *TestFixture) getLoggingConfig(ctx context.Context) (int, []string, error) {
	currentConfig, err := tf.wifiClient.GetLoggingConfig(ctx, &empty.Empty{})
	if err != nil {
		return 0, nil, err
	}
	return int(currentConfig.DebugLevel), currentConfig.DebugTags, err
}

// SetWakeOnWifiOption is the type of options of SetWakeOnWifi method of TestFixture.
type SetWakeOnWifiOption func(*wifi.WakeOnWifiConfig)

// WakeOnWifiFeatures returns a option for SetWakeOnWifi to modify the
// WakeOnWiFiFeaturesEnabled property.
func WakeOnWifiFeatures(features string) SetWakeOnWifiOption {
	return func(config *wifi.WakeOnWifiConfig) {
		config.Features = features
	}
}

// WakeOnWifiNetDetectScanPeriod returns an option for SetWakeOnWifi to modify
// the NetDetectScanPeriodSeconds property.
func WakeOnWifiNetDetectScanPeriod(seconds uint32) SetWakeOnWifiOption {
	return func(config *wifi.WakeOnWifiConfig) {
		config.NetDetectScanPeriod = seconds
	}
}

// SetWakeOnWifi sets properties related to wake on WiFi.
func (tf *TestFixture) SetWakeOnWifi(ctx context.Context, ops ...SetWakeOnWifiOption) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	resp, err := tf.WifiClient().GetWakeOnWifi(ctx, &empty.Empty{})
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to get WoWiFi setting")
	}

	origConfig := resp.Config
	newConfig := *origConfig // Copy so we won't modify the original one.

	// Allow WakeOnWiFi.
	newConfig.Allowed = true
	for _, op := range ops {
		op(&newConfig)
	}

	ctxRestore := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	req := &wifi.SetWakeOnWifiRequest{
		Config: &newConfig,
	}
	if _, err := tf.WifiClient().SetWakeOnWifi(ctx, req); err != nil {
		return ctx, nil, errors.Wrap(err, "failed to set WoWiFi features")
	}
	restore := func() error {
		cancel()
		req := &wifi.SetWakeOnWifiRequest{
			Config: origConfig,
		}
		if _, err := tf.WifiClient().SetWakeOnWifi(ctxRestore, req); err != nil {
			return errors.Wrapf(err, "failed to restore WoWiFi features to %v", origConfig)
		}
		return nil
	}
	return ctx, restore, nil
}
