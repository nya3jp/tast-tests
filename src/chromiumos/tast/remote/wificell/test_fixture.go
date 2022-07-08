// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"math/rand"
	"net"
	"strconv"
	"time"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/router/ax"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/router/legacy"
	"chromiumos/tast/remote/wificell/router/openwrt"
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

// TFRouter sets the router hostname(s) for the test fixture.
// Format: hostname[:port]
func TFRouter(targets ...string) TFOption {
	return func(tf *TestFixture) {
		tf.routers = make([]WiFiRouter, len(targets))
		for i := range targets {
			tf.routers[i] = &WiFiRouterImpl{hostName: targets[i]}
		}
	}
}

// TFPcap sets the pcap hostname for the test fixture.
// Format: hostname[:port]
func TFPcap(target string) TFOption {
	return func(tf *TestFixture) {
		tf.pcap = &WiFiRouterImpl{hostName: target}
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
func TFRouterType(rtype support.RouterType) TFOption {
	return func(tf *TestFixture) {
		// tf.routerType = rtype
	}
}

// TFPcapType sets the router type of the pcap capturing device. The pcap device in our testbeds is a router.
func TFPcapType(rtype support.RouterType) TFOption {
	return func(tf *TestFixture) {
		// tf.pcapType = rtype
	}
}

// TFHostUsers saves the mapping of hostname to username. This is used to figure
// out what username to log in with for a given hostname. If a username entry is
// not found for a given hostname, the default username, root, will be used.
func TFHostUsers(hostUsers map[string]string) TFOption {
	return func(tf *TestFixture) {
		tf.hostUsers = hostUsers
	}
}

// TFCompanionDUT sets the companion DUT to use in the test fixture.
func TFCompanionDUT(cd *dut.DUT) TFOption {
	return func(tf *TestFixture) {
		tf.duts = append(tf.duts, &WiFiDUTImpl{dut: cd})
	}
}

const (
	// TFServiceName is the service needed by TestFixture.
	TFServiceName = "tast.cros.wifi.ShillService"
	// DefaultDUT is the default DUT index (0).
	DefaultDUT = 0
)

// TODO(b/234845693): make that an independent structure.
// type routerData struct {
// 	target string
// 	host   *ssh.Conn
// 	object router.Base
// }

// DutIdx is the type used for DUT Index.
type DutIdx int

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	hostUsers map[string]string
	duts      []WiFiDUT
	routers   []WiFiRouter
	pcap      WiFiRouter
	// routerType support.RouterType
	// pcapType support.RouterType

	// pcapHostName string
	// pcapConn     *ssh.Conn
	// pcap         router.Base

	attenuatorTarget string
	attenuator       *attenuator.Attenuator

	setLogging bool
	logLevel   int
	logTags    []string

	// Group simple option flags here as they started to grow.
	option struct {
		packetCapture   bool
		withUI          bool
		routerAsCapture bool
	}

	capturers map[*APIface]*pcap.Capturer

	// aps is a set of APs useful for deconfiguring all APs, which some tests require.
	aps map[*APIface]struct{}
}

var apID int

// NewTestFixture creates a TestFixture.
// The TestFixture contains a gRPC connection to the DUT and a SSH connection to the router.
// The method takes two context: ctx and daemonCtx, the first one is the context for the operation and
// daemonCtx is for the spawned daemons.
// Noted that if routerHostname is empty, it uses the default router hostname based on the DUT's hostname.
// After the caller gets the TestFixture instance, it should reserve time for Close() the TestFixture:
//
//	tf, err := NewTestFixture(ctx, ...)
//	if err != nil {...}
//	defer tf.Close(ctx)
//	ctx, cancel := tf.ReserveForClose(ctx)
//	defer cancel()
//	...
func NewTestFixture(fullCtx, daemonCtx context.Context, d *dut.DUT, rpcHint *testing.RPCHint, ops ...TFOption) (ret *TestFixture, retErr error) {
	fullCtx, st := timing.Start(fullCtx, "NewTestFixture")
	defer st.End()

	tf := &TestFixture{
		capturers: make(map[*APIface]*pcap.Capturer),
		aps:       make(map[*APIface]struct{}),
		// Set the router's default router type.
		// routerType: support.LegacyT,
		// Set the pcap capture device's default router type.
		// pcapType: support.LegacyT,
		// Set the debug values on the DUT by default.
		setLogging: true,
		// Default log level used in WiFi tests.
		logLevel: -2,
		// Default log tags used in WiFi tests. Example of other tags that can be added.
		// (connection + dbus + device + link + manager + portal + service)
		logTags: []string{"wifi"},
	}
	tf.duts = append(tf.duts, &WiFiDUTImpl{dut: d})

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

	for i := range tf.duts {
		var lCfg *loggingConfig
		if tf.setLogging {
			lCfg = &loggingConfig{logLevel: tf.logLevel, logTags: tf.logTags}
		}
		if err := tf.duts[i].Connect(fullCtx, daemonCtx, rpcHint, lCfg, tf.option.withUI); err != nil {
			return nil, err // TODO: better err
		}
	}
	for i := range tf.duts {
		if tf.duts[i].WiFiClient() == nil {
			panic("WiFiClient null")
		}
	}

	// Wificell precondition always provides us with router name, but we need
	// to handle case when the fixture is created from outside of the precondition.
	if len(tf.routers) == 0 {
		testing.ContextLog(ctx, "Using default router name")
		name, err := tf.DUT(DefaultDUT).CompanionDeviceHostName(dut.CompanionSuffixRouter)
		if err != nil {
			return nil, errors.Wrap(err, "failed to synthesize default router name")
		}
		tf.routers = append(tf.routers, &WiFiRouterImpl{hostName: name})
	}
	for _, rt := range tf.routers {
		if err := rt.Connect(fullCtx, daemonCtx, tf.DUT(DefaultDUT), tf.hostUsers); err != nil {
			return nil, err // TODO: better err
		}
	}
	if tf.option.routerAsCapture {
		testing.ContextLog(ctx, "Using router as pcap")
		tf.pcap = tf.routers[0]
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
	var pcapHostName string
	if tf.pcap == nil {
		var err error
		testing.ContextLog(ctx, "Using default pcap name")
		name, err := tf.DUT(DefaultDUT).CompanionDeviceHostName(dut.CompanionSuffixPcap)
		if err != nil {
			// DUT might be specified with IP. As the routers are available,
			// fallback to use router as pcap in this case.
			pcapHostName = name
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
		if pcapHostName == router.Name() {
			testing.ContextLog(ctx, "Supplied pcap name already on router list")
			tf.pcap = router
		}
	}

	// If pcap name is available and unique, try to connect it.
	if tf.pcap == nil && pcapHostName != "" {
		var err error
		tf.pcap = &WiFiRouterImpl{hostName: pcapHostName}
		err = tf.pcap.Connect(ctx, daemonCtx, tf.DUT(DefaultDUT), tf.hostUsers)
		if err != nil {
			// We want to fallback to use router as pcap iff the default
			// pcap hostname is invalid. Fail here if it's not the case.
			if !useDefaultPcap || !errInvalidHost(err) {
				return nil, errors.Wrap(err, "failed to connect to pcap")
			}
		} else {
			testing.ContextLogf(ctx, "Successfully instantiated %s router controller for pcap", tf.pcap.RouterType().String())
			// Validate that the pcap router actually supports pcap
			if tf.pcap.SupportsCapture() {
				return nil, errors.Errorf("router type %q does not support Capture", tf.pcap.RouterType().String())
			}
		}
	}

	// Finally, fallback to use the first router as pcap if needed.
	if tf.pcap == nil {
		testing.ContextLog(ctx, "Fallback to use router 0 as pcap")
		tf.pcap = tf.routers[0]
	}

	if tf.attenuatorTarget != "" {
		testing.ContextLog(ctx, "Opening Attenuator: ", tf.attenuatorTarget)
		var err error
		// openWrtRouter #0 should always be present, thus we use it as a proxy.
		tf.attenuator, err = attenuator.Open(ctx, tf.attenuatorTarget, tf.routers[0].Conn())
		if err != nil {
			return nil, errors.Wrap(err, "failed to open attenuator")
		}
	}

	// Seed the random as we have some randomization. e.g. default SSID.
	seed := time.Now().UnixNano()
	testing.ContextLog(ctx, "Random seed: ", seed)
	rand.Seed(seed)
	return tf, nil
}

// NumberOfDUTs returns number of DUTs handled by this fixture.
func (tf *TestFixture) NumberOfDUTs() int {
	return len(tf.duts)
}

// DUT returns particular DUT.
func (tf *TestFixture) DUT(dutIdx DutIdx) WiFiDUT {
	return tf.duts[dutIdx]
}

// DUTConn returns connection object to particular DUT.
func (tf *TestFixture) DUTConn(dutIdx DutIdx) *ssh.Conn {
	return tf.DUT(dutIdx).Conn()
}

// ReserveForClose returns a shorter ctx and cancel function for tf.Close().
func (tf *TestFixture) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}

// CollectLogs downloads related log files to OutDir.
func (tf *TestFixture) CollectLogs(ctx context.Context) error {
	var firstErr error
	for _, rt := range tf.routers {
		err := rt.CollectLogs(ctx)
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
	for _, d := range tf.duts {
		if err := d.ResetNetCertStore(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to reset the NetCertStore"))
		}
	}

	if tf.attenuator != nil {
		tf.attenuator.Close()
		tf.attenuator = nil
	}

	// Check if one of routers was used in dual-purpose (router&pcap) mode.
	if tf.pcap != nil {
		for _, rt := range tf.routers {
			if tf.pcap == rt {
				// Don't close it, it will be closed while handling routers.
				tf.pcap = nil
				break
			}
		}
	}
	// If pcap was created specifically for this purpose, close it.
	if tf.pcap != nil {
		if err := tf.pcap.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap"))
		}
		tf.pcap = nil
	}
	// Close all created routers.
	for _, rt := range tf.routers {
		if err := rt.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close router %s", rt.Name()))
		}
	}
	for _, d := range tf.duts {
		d.Close(ctx)
	}

	// Do not close DUT, it'll be closed by the framework.
	return firstErr
}

// Reinit reinitialize the TestFixture. This can be used in precondition or between
// testcases to guarantee a cleaner state.
func (tf *TestFixture) Reinit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for _, d := range tf.duts {
		if err := d.Reinit(ctx); err != nil {
			// No need to reinit others
			return err
		}
	}
	return nil
}

// UniqueAPName returns a unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func UniqueAPName() string {
	id := strconv.Itoa(apID)
	apID++
	return id
}

func (tf *TestFixture) configureAP(ctx context.Context, idx int, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
	var cap WiFiRouter
	if tf.option.packetCapture {
		cap = tf.pcap
	}

	ap, capturer, err := tf.routers[idx].ConfigureAP(ctx, ops, fac, cap)
	if err == nil {
		tf.aps[ap] = struct{}{}

		if cap != nil {
			tf.capturers[ap] = capturer
		}
	}
	return ap, err
}

// ConfigureAPOnRouterID is an extended version of ConfigureAP, allowing to choose router
// to establish the AP on.
func (tf *TestFixture) ConfigureAPOnRouterID(ctx context.Context, idx int, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
	return tf.configureAP(ctx, idx, ops, fac)
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
// Note that after getting an APIface, ap, the caller should defer tf.DeconfigAP(ctx, ap) and
// use tf.ReserveForClose(ctx, ap) to reserve time for the deferred call.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
	return tf.configureAP(ctx, 0, ops, fac)
}

// ReserveForDeconfigAP returns a shorter ctx and cancel function for tf.DeconfigAP().
func (tf *TestFixture) ReserveForDeconfigAP(ctx context.Context, ap *APIface) (context.Context, context.CancelFunc) {
	if len(tf.routers) == 0 {
		return ctx, func() {}
	}

	ctx, cancel := ap.ReserveForStop(ctx)
	if capturer, ok := tf.capturers[ap]; ok {
		// Also reserve time for stopping the capturer if it exists.
		// Noted that CancelFunc returned here is dropped as we rely on its
		// parent's cancel() being called.
		if p, ok := tf.pcap.obj().(support.Capture); !ok {
			ctx, _ = p.ReserveForStopCapture(ctx, capturer)
		} else {
			// Stop the call if the router does not support.Capture.
			return ctx, func() {}
		}
	}
	return ctx, cancel
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *APIface) error {
	return tf.DeconfigAP(ctx, ap)
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
func (tf *TestFixture) StartWPAMonitor(ctx context.Context, dutIdx DutIdx) (wpaMonitor *WPAMonitor, stop func(), newCtx context.Context, retErr error) {
	return tf.DUT(dutIdx).StartWPAMonitor(ctx)
}

// Capturer returns the auto-spawned Capturer for the APIface instance.
func (tf *TestFixture) Capturer(ap *APIface) (*pcap.Capturer, bool) {
	capturer, ok := tf.capturers[ap]
	return capturer, ok
}

// ConnectWifi is backwards-compatible version of ConnectWifiFromDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ConnectWifi(ctx context.Context, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.DUT(DefaultDUT).ConnectSSID(ctx, ssid, options...)
}

// ConnectWifiFromDUT asks the DUT #dutIdx to connect to the specified WiFi.
func (tf *TestFixture) ConnectWifiFromDUT(ctx context.Context, dutIdx DutIdx, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.DUT(dutIdx).ConnectSSID(ctx, ssid, options...)
}

// ConnectWifiAPFromDUT asks the given DUT to connect to the WiFi provided by the given AP.
func (tf *TestFixture) ConnectWifiAPFromDUT(ctx context.Context, dutIdx DutIdx, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.DUT(dutIdx).ConnectAP(ctx, ap, options...)
}

// ConnectWifiAP is backwards-compatible version of ConnectWifiAPFromDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ConnectWifiAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.DUT(DefaultDUT).ConnectAP(ctx, ap, options...)
}

func (tf *TestFixture) disconnectWifi(ctx context.Context, dutIdx DutIdx, removeProfile bool) error {
	return tf.DUT(dutIdx).DisconnectWiFi(ctx, removeProfile)
}

// DisconnectWifi is backwards-compatible version of DisconnectDUTFromWifi. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	return tf.DUT(DefaultDUT).DisconnectWiFi(ctx, false)
}

// CleanDisconnectWifi is backwards-compatible version of CleanDisconnectDUTFromWifi. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) CleanDisconnectWifi(ctx context.Context) error {
	return tf.DUT(DefaultDUT).DisconnectWiFi(ctx, true)
}

// DisconnectDUTFromWifi asks the given DUT to disconnect from current WiFi service.
func (tf *TestFixture) DisconnectDUTFromWifi(ctx context.Context, dutIdx DutIdx) error {
	return tf.DUT(dutIdx).DisconnectWiFi(ctx, false)
}

// CleanDisconnectDUTFromWifi asks the given DUT to disconnect from current WiFi service and removes the configuration.
func (tf *TestFixture) CleanDisconnectDUTFromWifi(ctx context.Context, dutIdx DutIdx) error {
	return tf.DUT(dutIdx).DisconnectWiFi(ctx, true)
}

// ReserveForDisconnect returns a shorter ctx and cancel function for tf.DisconnectWifi.
func (tf *TestFixture) ReserveForDisconnect(ctx context.Context) (context.Context, context.CancelFunc) {
	return tf.DUT(DefaultDUT).ReserveForDisconnectWiFi(ctx)
}

// PingFromDUT is backwards-compatible version of PingFromSpecificDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) PingFromDUT(ctx context.Context, targetIP string, opts ...ping.Option) error {
	res, err := tf.PingFromSpecificDUT(ctx, DefaultDUT, targetIP, opts...)
	if err != nil {
		return err
	}
	return VerifyPingResults(res, pingLossThreshold)
}

// PingFromSpecificDUT tests the connectivity between the given DUT and a target IP.
func (tf *TestFixture) PingFromSpecificDUT(ctx context.Context, dutIdx DutIdx, targetIP string, opts ...ping.Option) (*ping.Result, error) {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	// Bind ping used in all WiFi Tests to WiFiInterface. Otherwise if the
	// WiFi interface is not up yet they will be routed through the Ethernet
	// interface. Also see b/225205611 for details.
	opts = append(opts, ping.BindAddress(true), ping.SourceIface(iface))

	ctx, st := timing.Start(ctx, "tf.PingFromDUT")
	defer st.End()

	pr := remoteping.NewRemoteRunner(tf.DUT(dutIdx).Conn())
	res, err := pr.Ping(ctx, targetIP, opts...)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	return res, nil
}

// VerifyPingResults checks if ping results are within acceptable range.
func VerifyPingResults(res *ping.Result, lossThreshold float64) error {

	if res.Loss > lossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	return nil
}

// PingFromRouterID tests the connectivity between DUT and router through currently connected WiFi service.
func (tf *TestFixture) PingFromRouterID(ctx context.Context, idx int, opts ...ping.Option) error {
	ctx, st := timing.Start(ctx, "tf.PingFromRouterID")
	defer st.End()

	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		return errors.Wrap(err, "failed to get the IP address")
	}

	pr := remoteping.NewRemoteRunner(tf.routers[idx].Conn())
	res, err := pr.Ping(ctx, addrs[0].String(), opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)

	return VerifyPingResults(res, pingLossThreshold)
}

// PingFromServer calls PingFromRouterID for router 0.
// Kept for backwards-compatibility.
func (tf *TestFixture) PingFromServer(ctx context.Context, opts ...ping.Option) error {
	return tf.PingFromRouterID(ctx, 0, opts...)
}

// ArpingFromDUT is backwards-compatible version of ArpingFromSpecificDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ArpingFromDUT(ctx context.Context, serverIP string, ops ...arping.Option) error {
	return tf.ArpingFromSpecificDUT(ctx, DefaultDUT, serverIP, ops...)
}

// ArpingFromSpecificDUT tests that the given DUT can send the broadcast packets to server.
func (tf *TestFixture) ArpingFromSpecificDUT(ctx context.Context, dutIdx DutIdx, serverIP string, ops ...arping.Option) error {
	ctx, st := timing.Start(ctx, "tf.ArpingFromDUT")
	defer st.End()

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the client WiFi interface")
	}

	runner := remotearping.NewRemoteRunner(tf.DUT(dutIdx).Conn())
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

	runner := remotearping.NewRemoteRunner(tf.routers[idx].Conn())
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

// ClientIPv4Addrs is backwards-compatible version of DUTIPv4Addrs. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ClientIPv4Addrs(ctx context.Context) ([]net.IP, error) {
	return tf.DUT(DefaultDUT).IPv4Addrs(ctx)
}

// DUTIPv4Addrs returns the IPv4 addresses for the network interface.
func (tf *TestFixture) DUTIPv4Addrs(ctx context.Context, dutIdx DutIdx) ([]net.IP, error) {
	return tf.DUT(dutIdx).IPv4Addrs(ctx)
}

// ClientHardwareAddr is backwards-compatible version of DUTHardwareAddr. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ClientHardwareAddr(ctx context.Context) (string, error) {
	mac, err := tf.DUTHardwareAddr(ctx, DefaultDUT)
	return mac.String(), err
}

// DUTHardwareAddr returns the HardwareAddr for the network interface.
func (tf *TestFixture) DUTHardwareAddr(ctx context.Context, dutIdx DutIdx) (net.HardwareAddr, error) {
	iface, err := tf.DUTClientInterface(ctx, dutIdx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetHardwareAddrRequest{
		InterfaceName: iface,
	}
	resp, err := tf.DUTWifiClient(dutIdx).GetHardwareAddr(ctx, netIface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the HardwareAddr")
	}

	return net.ParseMAC(resp.HwAddr)
}

// AssertNoDisconnect runs the given routine and verifies that no disconnection event
// is captured in the same duration.
func (tf *TestFixture) AssertNoDisconnect(ctx context.Context, dutIdx DutIdx, f func(context.Context) error) error {
	return tf.DUT(dutIdx).AssertNoDisconnect(ctx, f)
}

// RouterByID returns the respective router object in the fixture.
func (tf *TestFixture) RouterByID(idx int) router.Base {
	return tf.routers[idx].obj()
}

// Router returns the router with id 0 in the fixture as the generic router.Base.
func (tf *TestFixture) Router() router.Base {
	return tf.RouterByID(0)
}

// StandardRouter returns the Router as a router.Standard.
func (tf *TestFixture) StandardRouter() (router.Standard, error) {
	r, ok := tf.Router().(router.Standard)
	if !ok {
		return nil, errors.New("router is not a standard router")
	}
	return r, nil
}

// StandardRouterWithFrameSenderSupport returns the Router as a router.StandardWithFrameSender.
func (tf *TestFixture) StandardRouterWithFrameSenderSupport() (router.StandardWithFrameSender, error) {
	r, ok := tf.Router().(router.StandardWithFrameSender)
	if !ok {
		return nil, errors.New("router is not a standard router with frame sender support")
	}
	return r, nil
}

// StandardRouterWithBridgeAndVethSupport returns the Router as a router.StandardWithBridgeAndVeth.
func (tf *TestFixture) StandardRouterWithBridgeAndVethSupport() (router.StandardWithBridgeAndVeth, error) {
	r, ok := tf.Router().(router.StandardWithBridgeAndVeth)
	if !ok {
		return nil, errors.New("router is not a standard router with bridge and veth support")
	}
	return r, nil
}

// LegacyRouter returns the Router as a legacy.Router.
func (tf *TestFixture) LegacyRouter() (*legacy.Router, error) {
	r, ok := tf.Router().(*legacy.Router)
	if !ok {
		return nil, errors.New("router is not a legacy router")
	}
	return r, nil
}

// AxRouter returns the Router as an ax.Router.
func (tf *TestFixture) AxRouter() (*ax.Router, error) {
	r, ok := tf.Router().(*ax.Router)
	if !ok {
		return nil, errors.New("router is not an ax router")
	}
	return r, nil
}

// OpenWrtRouter returns the Router as an openwrt.Router.
func (tf *TestFixture) OpenWrtRouter() (*openwrt.Router, error) {
	r, ok := tf.Router().(*openwrt.Router)
	if !ok {
		return nil, errors.New("router is not an OpenWrt router")
	}
	return r, nil
}

// Pcap returns the pcap device in the fixture.
func (tf *TestFixture) Pcap() router.Base {
	return tf.pcap.obj()
}

// LegacyPcap returns the Pcap as a legacy.Router.
func (tf *TestFixture) LegacyPcap() (*legacy.Router, error) {
	p, ok := tf.Pcap().(*legacy.Router)
	if !ok {
		return nil, errors.New("pcap is not a legacy router")
	}
	return p, nil
}

// OpenWrtPcap returns the Pcap as an openwrt.Router.
func (tf *TestFixture) OpenWrtPcap() (*openwrt.Router, error) {
	p, ok := tf.Pcap().(*openwrt.Router)
	if !ok {
		return nil, errors.New("pcap is not an OpenWrt router")
	}
	return p, nil
}

// Attenuator returns the Attenuator object in the fixture.
func (tf *TestFixture) Attenuator() *attenuator.Attenuator {
	return tf.attenuator
}

// WifiClient is a backwards-compatible version of DUTWifiClient. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) WifiClient() *WifiClient {
	return tf.DUTWifiClient(DefaultDUT)
}

// DUTWifiClient returns the gRPC ShillServiceClient of the given DUT.
func (tf *TestFixture) DUTWifiClient(dutIdx DutIdx) *WifiClient {
	return tf.DUT(dutIdx).WiFiClient()
}

// RPC returns the gRPC connection of the default DUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) RPC() *rpc.Client {
	return tf.DUTRPC(DefaultDUT)
}

// DUTRPC returns the gRPC connection of the given DUT.
func (tf *TestFixture) DUTRPC(dutIdx DutIdx) *rpc.Client {
	return tf.DUT(dutIdx).RPC()
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

// ClientInterface is a backwards-compatible version of DUTClientInterface. Deprecated.
func (tf *TestFixture) ClientInterface(ctx context.Context) (string, error) {
	return tf.DUTWifiClient(DefaultDUT).Interface(ctx)
}

// DUTClientInterface returns the client interface name of the given DUT.
func (tf *TestFixture) DUTClientInterface(ctx context.Context, dutIdx DutIdx) (string, error) {
	return tf.DUTWifiClient(dutIdx).Interface(ctx)
}

// VerifyConnection is backwards-compatible version of VerifyConnectionFromDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) VerifyConnection(ctx context.Context, ap *APIface) error {
	return tf.VerifyConnectionFromDUT(ctx, 0, ap)
}

// VerifyConnectionFromDUT verifies that the AP is reachable from a specific DUT by pinging, and we have the same frequency and subnet as AP's.
func (tf *TestFixture) VerifyConnectionFromDUT(ctx context.Context, dutIdx DutIdx, ap *APIface) error {
	iface, err := tf.DUTClientInterface(ctx, dutIdx)
	if err != nil {
		return errors.Wrap(err, "failed to get interface from the DUT")
	}

	// Check frequency.
	service, err := tf.DUTWifiClient(dutIdx).QueryService(ctx)
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
	addrs, err := tf.DUTWifiClient(dutIdx).GetIPv4Addrs(ctx, &wifi.GetIPv4AddrsRequest{InterfaceName: iface})
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
	res, err := tf.PingFromSpecificDUT(ctx, dutIdx, ap.ServerIP().String())
	if err != nil {
		return errors.Wrap(err, "failed to ping from the DUT")
	}
	if err := VerifyPingResults(res, pingLossThreshold); err != nil {
		return errors.Wrap(err, "ping verification failed")
	}

	return nil
}

// ClearBSSIDIgnoreDUT clears the BSSID_IGNORE list on DUT.
func (tf *TestFixture) ClearBSSIDIgnoreDUT(ctx context.Context, dutIdx DutIdx) error {
	return tf.DUT(dutIdx).ClearBSSIDIgnore(ctx)
}

// SendChannelSwitchAnnouncement sends a CSA frame and waits for Client_Disconnection, or Channel_Switch event.
func (tf *TestFixture) SendChannelSwitchAnnouncement(ctx context.Context, dutIdx DutIdx, ap *APIface, maxRetry, alternateChannel int) error {
	ctxForCloseFrameSender := ctx
	r, ok := tf.Router().(support.FrameSender)
	if !ok {
		return errors.Errorf("router type %q does not support FrameSender", tf.Router().RouterType().String())
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

	ew, err := iw.NewEventWatcher(ctx, tf.DUT(dutIdx).DUT())
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventWatcher")
	}
	defer ew.Stop()

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
func (tf *TestFixture) DisablePowersaveMode(ctx context.Context, dutIdx DutIdx) (shortenCtx context.Context, restore func() error, err error) {
	return tf.DUT(dutIdx).DisablePowersaveMode(ctx)
}

// setLoggingConfig configures the logging setting with the specified values (level and tags).
// func setLoggingConfig(ctx context.Context, wc *WifiClient, level int, tags []string) error {
// 	testing.ContextLogf(ctx, "Configuring logging level: %d, tags: %v", level, tags)
// 	_, err := wc.SetLoggingConfig(ctx, &wifi.SetLoggingConfigRequest{DebugLevel: int32(level), DebugTags: tags})
// 	return err
// }

// getLoggingConfig returns the current DUT's logging setting (level and tags).
// func (tf *TestFixture) getLoggingConfig(ctx context.Context, wc *WifiClient) (int, []string, error) {
// 	currentConfig, err := wc.GetLoggingConfig(ctx, &empty.Empty{})
// 	if err != nil {
// 		return 0, nil, err
// 	}
// 	return int(currentConfig.DebugLevel), currentConfig.DebugTags, err
// }

// SetWakeOnWifi sets properties related to wake on WiFi.
// DEPRECATED: Use tf.WifiClient().SetWakeOnWifi instead.
func (tf *TestFixture) SetWakeOnWifi(ctx context.Context, ops ...SetWakeOnWifiOption) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	return tf.WifiClient().SetWakeOnWifi(ctx, ops...)
}

// WaitWifiConnected waits until WiFi is connected to the SHILL profile with specific GUID.
func (tf *TestFixture) WaitWifiConnected(ctx context.Context, dutIdx DutIdx, guid string) error {
	return tf.DUT(dutIdx).WaitWifiConnected(ctx, guid)
}
