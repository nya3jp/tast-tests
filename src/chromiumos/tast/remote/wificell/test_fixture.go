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
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
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
		tf.packetCapture = b
	}
}

// TFAttenuator sets the attenuator hostname to use in the test fixture.
func TFAttenuator(target string) TFOption {
	return func(tf *TestFixture) {
		tf.attenuatorTarget = target
	}
}

// TFServiceName is the service needed by TestFixture.
const TFServiceName = "tast.cros.network.WifiService"

type routerData struct {
	target string
	host   *ssh.Conn
	object *Router
}

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	wifiClient network.WifiServiceClient

	routers []routerData

	pcapTarget    string
	pcapHost      *ssh.Conn
	pcap          *Router
	packetCapture bool

	attenuatorTarget string
	attenuator       *attenuator.Attenuator

	apID      int
	capturers map[*APIface]*pcap.Capturer

	// apRouterIDs is reverse map for router identification for two purposes:
	// 1) We need to know which router the APIface belongs to deconfigure
	//    AP on correct device;
	// 2) We need to know (?) a complete list of APIfaces to deconfigure all APs,
	//    which some tests require.
	apRouterIDs map[*APIface]int

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

	runner, err := hwsec.NewCmdRunner(tf.dut)
	if err != nil {
		return err
	}
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
		dut:         d,
		capturers:   make(map[*APIface]*pcap.Capturer),
		apRouterIDs: make(map[*APIface]int),
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
	tf.wifiClient = network.NewWifiServiceClient(tf.rpc.Conn)

	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.
	if _, err := tf.wifiClient.InitDUT(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to InitDUT")
	}

	if len(tf.routers) == 0 {
		testing.ContextLog(ctx, "Using default router name")
		tf.routers = append(tf.routers, routerData{target: "default-router"})
		routerHost, err := tf.dut.DefaultWifiRouterHost(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to the default router")
		}
		tf.routers[0].host = routerHost
	} else {
		for i := range tf.routers {
			router := &tf.routers[i]
			testing.ContextLogf(ctx, "Adding router %s", router.target)
			routerHost, err := tf.connectCompanion(ctx, router.target)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to connect to the router %s", router.target)
			}
			router.host = routerHost
		}
	}

	for i := range tf.routers {
		router := &tf.routers[i]
		routerObj, err := NewRouter(ctx, daemonCtx, router.host,
			strings.ReplaceAll(router.target, ":", "_"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a router object")
		}
		router.object = routerObj
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

	// TODO(crbug.com/1133855): Handle the case that routerTarget and pcapTarget
	// is pointing to the same device. Current Router object does not allow this.
	if tf.pcapTarget == "" {
		tf.pcapHost, err = tf.dut.DefaultWifiPcapHost(ctx)
		if err != nil && errInvalidHost(err) {
			testing.ContextLog(ctx, "Use router 0 as pcap because default pcap is not reachable: ", err)
			tf.pcapHost = tf.routers[0].host
			tf.pcap = tf.routers[0].object
			err = nil
		}
	} else {
		for _, router := range tf.routers {
			if tf.pcapTarget == router.target {
				return nil, errors.Errorf("failed to set up pcap: same target for router and pcap: %s", tf.pcapTarget)
			}
		}
		tf.pcapHost, err = tf.connectCompanion(ctx, tf.pcapTarget)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to pcap")
	}

	if tf.pcap == nil {
		tf.pcap, err = NewRouter(ctx, daemonCtx, tf.pcapHost, "pcap")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a router object for pcap")
		}
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
	for _, router := range tf.routers {
		err := router.object.CollectLogs(ctx)
		if err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to collect logs"))
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

	var firstErr error

	if err := tf.resetNetCertStore(ctx); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to reset the NetCertStore"))
	}

	if tf.attenuator != nil {
		tf.attenuator.Close()
	}

	// TODO(crbug.com/1133855) Handle proper closing when pcap will be able to be router.
	if tf.pcap != nil && tf.pcap != tf.routers[0].object {
		if err := tf.pcap.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap"))
		}
		if err := tf.pcapHost.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap ssh"))
		}
	}
	for i := range tf.routers {
		router := &tf.routers[i]
		if router.object != nil {
			if err := router.object.Close(ctx); err != nil {
				collectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close rotuer %s", router.target))
			}
		}
		router.object = nil
		if router.host != nil {
			if err := router.host.Close(ctx); err != nil {
				collectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close router %s ssh", router.target))
			}
		}
		router.host = nil
	}
	if tf.wifiClient != nil {
		if _, err := tf.wifiClient.TearDown(ctx, &empty.Empty{}); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
		}
	}
	if tf.rpc != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		tf.rpc.Close(ctx)
	}
	// Do not close DUT, it'll be closed by the framework.
	return firstErr
}

// Reinit reinitialize the TestFixture. This can be used in precondition or between
// testcases to guarantee a cleaner state.
func (tf *TestFixture) Reinit(ctx context.Context) error {
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

	if err := config.SecurityConfig.InstallRouterCredentials(ctx, tf.routers[idx].host,
		tf.routers[idx].object.workDir()); err != nil {
		return nil, err
	}

	var capturer *pcap.Capturer
	if tf.packetCapture {
		freqOps, err := config.PcapFreqOptions()
		if err != nil {
			return nil, err
		}
		capturer, err = tf.pcap.StartCapture(ctx, name, config.Channel, freqOps)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		defer func() {
			if retErr != nil {
				tf.pcap.StopCapture(ctx, capturer)
			}
		}()
	}

	ap, err := tf.routers[idx].object.StartAPIface(ctx, name, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start APIface")
	}
	tf.apRouterIDs[ap] = idx

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
	ctx, cancel := tf.routers[tf.apRouterIDs[ap]].object.ReserveForStopAPIface(ctx, ap)
	if capturer, ok := tf.capturers[ap]; ok {
		// Also reserve time for stopping the capturer if it exists.
		// Noted that CancelFunc returned here is dropped as we rely on its
		// parent's cancel() being called.
		ctx, _ = tf.pcap.ReserveForStopCapture(ctx, capturer)
	}
	return ctx, cancel
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *APIface) error {
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()

	var firstErr error

	capturer := tf.capturers[ap]
	delete(tf.capturers, ap)
	if err := tf.routers[tf.apRouterIDs[ap]].object.StopAPIface(ctx, ap); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop APIface"))
	}
	if capturer != nil {
		if err := tf.pcap.StopCapture(ctx, capturer); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
		}
	}
	delete(tf.apRouterIDs, ap)
	return firstErr
}

// DeconfigAllAPs facilitates deconfiguration of all APs established for
// this test fixture.
func (tf *TestFixture) DeconfigAllAPs(ctx context.Context) error {
	var firstErr error
	for ap := range tf.apRouterIDs {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to deconfig AP"))
		}
	}
	return firstErr
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
func (tf *TestFixture) ConnectWifi(ctx context.Context, ssid string, options ...ConnOption) (*network.ConnectResponse, error) {
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
	request := &network.ConnectRequest{
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
	request := &network.DiscoverBSSIDRequest{
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
	request := &network.RequestRoamRequest{
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
	_, err := tf.wifiClient.Reassociate(ctx, &network.ReassociateRequest{
		InterfaceName: iface,
		Timeout:       timeout.Nanoseconds(),
	})
	return err
}

// ConnectWifiAP asks the DUT to connect to the WiFi provided by the given AP.
func (tf *TestFixture) ConnectWifiAP(ctx context.Context, ap *APIface, options ...ConnOption) (*network.ConnectResponse, error) {
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
	req := &network.DisconnectRequest{
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
	req := &network.AssureDisconnectRequest{
		ServicePath: servicePath,
		Timeout:     timeout.Nanoseconds(),
	}
	if _, err := tf.wifiClient.AssureDisconnect(ctx, req); err != nil {
		return err
	}
	return nil
}

// QueryService queries shill information of selected service.
func (tf *TestFixture) QueryService(ctx context.Context) (*network.QueryServiceResponse, error) {
	selectedSvcResp, err := tf.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get selected service")
	}

	req := &network.QueryServiceRequest{
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

	netIface := &network.GetIPv4AddrsRequest{
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

	netIface := &network.GetHardwareAddrRequest{
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
func (tf *TestFixture) RouterByID(idx int) *Router {
	return tf.routers[idx].object
}

// Router returns Router 0 object in the fixture.
func (tf *TestFixture) Router() *Router {
	return tf.RouterByID(0)
}

// Pcap returns the pcap Router object in the fixture.
func (tf *TestFixture) Pcap() *Router {
	return tf.pcap
}

// Attenuator returns the Attenuator object in the fixture.
func (tf *TestFixture) Attenuator() *attenuator.Attenuator {
	return tf.attenuator
}

// WifiClient returns the gRPC WifiServiceClient of the DUT.
func (tf *TestFixture) WifiClient() network.WifiServiceClient {
	return tf.wifiClient
}

// DefaultOpenNetworkAPOptions returns the Options for an common 802.11n open network.
// The function is useful to allow common logic shared between the default setting
// and customized setting.
func (tf *TestFixture) DefaultOpenNetworkAPOptions() []hostapd.Option {
	return []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
}

// DefaultOpenNetworkAP configures the router to provide an 802.11n open network.
func (tf *TestFixture) DefaultOpenNetworkAP(ctx context.Context) (*APIface, error) {
	return tf.ConfigureAP(ctx, tf.DefaultOpenNetworkAPOptions(), nil)
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
	addrs, err := tf.WifiClient().GetIPv4Addrs(ctx, &network.GetIPv4AddrsRequest{InterfaceName: iface})
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

// ShillProperty holds a shill service property with it's expected and unexpected values.
type ShillProperty struct {
	Property         string
	ExpectedValues   []interface{}
	UnexpectedValues []interface{}
	Method           network.ExpectShillPropertyRequest_CheckMethod
}

// ExpectShillProperty is a wrapper for the streaming gRPC call ExpectShillProperty.
// It takes an array of ShillProperty, an array of shill properties to monitor, and
// a shill service path. It returns a function that waites for the expected property
// changes and returns the monitor results.
func (tf *TestFixture) ExpectShillProperty(ctx context.Context, objectPath string, props []*ShillProperty, monitorProps []string) (func() ([]protoutil.ShillPropertyHolder, error), error) {
	var expectedProps []*network.ExpectShillPropertyRequest_Criterion
	for _, prop := range props {
		var anyOfVals []*network.ShillVal
		for _, shillState := range prop.ExpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			anyOfVals = append(anyOfVals, state)
		}

		var noneOfVals []*network.ShillVal
		for _, shillState := range prop.UnexpectedValues {
			state, err := protoutil.ToShillVal(shillState)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert property name to ShillVal")
			}
			noneOfVals = append(noneOfVals, state)
		}

		shillPropReqCriterion := &network.ExpectShillPropertyRequest_Criterion{
			Key:    prop.Property,
			AnyOf:  anyOfVals,
			NoneOf: noneOfVals,
			Method: prop.Method,
		}
		expectedProps = append(expectedProps, shillPropReqCriterion)
	}

	req := &network.ExpectShillPropertyRequest{
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

// SuspendAssertConnect suspends the DUT for wakeUpTimeout seconds through gRPC and returns the duration from resume to connect.
func (tf *TestFixture) SuspendAssertConnect(ctx context.Context, wakeUpTimeout time.Duration) (time.Duration, error) {
	service, err := tf.wifiClient.SelectedService(ctx, &empty.Empty{})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get selected service")
	}
	resp, err := tf.wifiClient.SuspendAssertConnect(ctx, &network.SuspendAssertConnectRequest{
		WakeUpTimeout: wakeUpTimeout.Nanoseconds(),
		ServicePath:   service.ServicePath,
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to suspend and assert connection")
	}
	return time.Duration(resp.ReconnectTime), nil
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
			_, err := tf.WifiClient().SetMACRandomize(ctx, &network.SetMACRandomizeRequest{Enable: false})
			if err != nil {
				return ctx, nil, errors.Wrap(err, "failed to disable MAC randomization")
			}
			// Restore the setting when exiting.
			restore := func() error {
				cancel()
				if _, err := tf.WifiClient().SetMACRandomize(ctxRestore, &network.SetMACRandomizeRequest{Enable: true}); err != nil {
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
