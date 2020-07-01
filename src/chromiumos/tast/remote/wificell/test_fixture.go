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

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
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
func TFRouter(target string) TFOption {
	return func(tf *TestFixture) {
		tf.routerTarget = target
	}
}

// TFPcap sets the pcap hostname for the test fixture.
// Format: hostname[:port]
func TFPcap(target string) TFOption {
	return func(tf *TestFixture) {
		tf.pcapTarget = target
	}
}

// TFCapture sets if the test fixture should spawn packet capturer.
func TFCapture(b bool) TFOption {
	return func(tf *TestFixture) {
		tf.packetCapture = b
	}
}

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	dut        *dut.DUT
	rpc        *rpc.Client
	wifiClient network.WifiServiceClient

	routerTarget string
	routerHost   *ssh.Conn
	router       *Router

	pcapTarget    string
	pcapHost      *ssh.Conn
	pcap          *Router
	packetCapture bool

	apID           int
	curServicePath string
	capturers      map[*APIface]*pcap.Capturer
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

	if tf.routerTarget == "" {
		tf.routerHost, err = tf.dut.DefaultWifiRouterHost(ctx)
	} else {
		tf.routerHost, err = tf.connectCompanion(ctx, tf.routerTarget)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the router")
	}
	tf.router, err = NewRouter(ctx, daemonCtx, tf.routerHost, "router")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a router object")
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

	// TODO(crbug.com/1034875): Handle the case that routerTarget and pcapTarget
	// is pointing to the same device. Current Router object does not allow this.
	if tf.pcapTarget == "" {
		tf.pcapHost, err = tf.dut.DefaultWifiPcapHost(ctx)
		if err != nil && errInvalidHost(err) {
			testing.ContextLog(ctx, "Use router as pcap because default pcap is not reachable: ", err)
			tf.pcapHost = tf.routerHost
			err = nil
		}
	} else if tf.pcapTarget == tf.routerTarget {
		err = errors.New("same target for router and pcap")
	} else {
		tf.pcapHost, err = tf.connectCompanion(ctx, tf.pcapTarget)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to pcap")
	}
	if tf.pcapHost == tf.routerHost {
		tf.pcap = tf.router
	} else {
		tf.pcap, err = NewRouter(ctx, daemonCtx, tf.pcapHost, "pcap")
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a router object for pcap")
		}
	}

	// TODO(crbug.com/1034875): set up attenuator.

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())
	return tf, nil
}

// ReserveForClose returns a shorter ctx and cancel function for tf.Close().
func (tf *TestFixture) ReserveForClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}

// Close closes the connections created by TestFixture.
func (tf *TestFixture) Close(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "tf.Close")
	defer st.End()

	var firstErr error
	if tf.pcap != nil && tf.pcap != tf.router {
		if err := tf.pcap.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap"))
		}
	}
	if tf.pcapHost != nil && tf.pcapHost != tf.routerHost {
		if err := tf.pcapHost.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close pcap ssh"))
		}
	}
	if tf.router != nil {
		if err := tf.router.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close rotuer"))
		}
	}
	if tf.routerHost != nil {
		if err := tf.routerHost.Close(ctx); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to close router ssh"))
		}
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

// getUniqueAPName returns an unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func (tf *TestFixture) getUniqueAPName() string {
	id := strconv.Itoa(tf.apID)
	tf.apID++
	return id
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
// Note that after getting an APIface, ap, the caller should defer tf.DeconfigAP(ctx, ap) and
// use tf.ReserveForClose(ctx, ap) to reserve time for the deferred call.
func (tf *TestFixture) ConfigureAP(ctx context.Context, ops []hostapd.Option, fac security.ConfigFactory) (ret *APIface, retErr error) {
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

	ap, err := tf.router.StartAPIface(ctx, name, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start APIface")
	}
	defer func() {
		if retErr != nil {
			tf.router.StopAPIface(ctx, ap)
		}
	}()

	if tf.packetCapture {
		freqOps, err := config.PcapFreqOptions()
		if err != nil {
			return nil, err
		}
		capturer, err := tf.pcap.StartCapture(ctx, name, config.Channel, freqOps)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		tf.capturers[ap] = capturer
	}
	return ap, nil
}

// ReserveForDeconfigAP returns a shorter ctx and cancel function for tf.DeconfigAP().
func (tf *TestFixture) ReserveForDeconfigAP(ctx context.Context, h *APIface) (context.Context, context.CancelFunc) {
	if tf.router == nil {
		return ctx, func() {}
	}
	return tf.router.ReserveForStopAPIface(ctx, h)
}

// DeconfigAP stops the WiFi service on router.
func (tf *TestFixture) DeconfigAP(ctx context.Context, ap *APIface) error {
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()

	var firstErr error

	capturer := tf.capturers[ap]
	delete(tf.capturers, ap)
	if capturer != nil {
		if err := tf.pcap.StopCapture(ctx, capturer); err != nil {
			collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
		}
	}
	if err := tf.router.StopAPIface(ctx, ap); err != nil {
		collectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop APIface"))
	}
	return firstErr
}

// ConnectWifi asks the DUT to connect to the specified WiFi.
func (tf *TestFixture) ConnectWifi(ctx context.Context, ssid string, hidden bool, secConf security.Config) (*network.ConnectResponse, error) {
	ctx, st := timing.Start(ctx, "tf.ConnectWifi")
	defer st.End()

	props, err := secConf.ShillServiceProperties()
	if err != nil {
		return nil, err
	}
	propsEnc, err := protoutil.EncodeToShillValMap(props)
	if err != nil {
		return nil, err
	}
	request := &network.ConnectRequest{
		Ssid:       []byte(ssid),
		Hidden:     hidden,
		Security:   secConf.Class(),
		Shillprops: propsEnc,
	}
	response, err := tf.wifiClient.Connect(ctx, request)
	if err != nil {
		return nil, err
	}
	tf.curServicePath = response.ServicePath
	return response, nil
}

// ConnectWifiAP asks the DUT to connect to the WiFi provided by the given AP.
func (tf *TestFixture) ConnectWifiAP(ctx context.Context, ap *APIface) (*network.ConnectResponse, error) {
	conf := ap.Config()
	return tf.ConnectWifi(ctx, conf.SSID, conf.Hidden, conf.SecurityConfig)
}

// DisconnectWifi asks the DUT to disconnect from current WiFi service and removes the configuration.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	if tf.curServicePath == "" {
		return errors.New("the current WiFi service path is empty")
	}
	ctx, st := timing.Start(ctx, "tf.DisconnectWifi")
	defer st.End()

	var err error
	req := &network.DisconnectRequest{ServicePath: tf.curServicePath}
	if _, err2 := tf.wifiClient.Disconnect(ctx, req); err2 != nil {
		err = errors.Wrap(err2, "failed to disconnect")
	}
	tf.curServicePath = ""
	return err
}

// AssureDisconnect assures that the WiFi service has disconnected within timeout.
func (tf *TestFixture) AssureDisconnect(ctx context.Context, timeout time.Duration) error {
	req := &network.AssureDisconnectRequest{
		ServicePath: tf.curServicePath,
		Timeout:     timeout.Nanoseconds(),
	}
	if _, err := tf.wifiClient.AssureDisconnect(ctx, req); err != nil {
		return err
	}
	tf.curServicePath = ""
	return nil
}

// QueryService queries shill service information.
func (tf *TestFixture) QueryService(ctx context.Context) (*network.QueryServiceResponse, error) {
	req := &network.QueryServiceRequest{
		Path: tf.curServicePath,
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

// PingFromServer tests the connectivity between DUT and router through currently connected WiFi service.
func (tf *TestFixture) PingFromServer(ctx context.Context, opts ...ping.Option) error {
	ctx, st := timing.Start(ctx, "tf.PingFromServer")
	defer st.End()

	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		return errors.Wrap(err, "failed to get the IP address")
	}

	pr := remoteping.NewRemoteRunner(tf.routerHost)
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
	testing.ContextLog(ctx, "arping from DUT: ", res.String())

	if res.Loss > arpingLossThreshold {
		return errors.Errorf("unexpected arping loss percentage: got %g%% want <= %g%%", res.Loss, arpingLossThreshold)
	}

	return nil
}

// ArpingFromServer tests that DUT can receive the broadcast packets from server.
func (tf *TestFixture) ArpingFromServer(ctx context.Context, serverIface string, ops ...arping.Option) error {
	ctx, st := timing.Start(ctx, "tf.ArpingFromServer")
	defer st.End()

	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		return errors.Wrap(err, "failed to get the IP address")
	}

	runner := remotearping.NewRemoteRunner(tf.routerHost)
	res, err := runner.Arping(ctx, addrs[0].String(), serverIface, ops...)
	if err != nil {
		return errors.Wrap(err, "arping failed")
	}
	testing.ContextLog(ctx, "arping from DUT: ", res.String())

	if res.Loss > arpingLossThreshold {
		return errors.Errorf("unexpected arping loss percentage: got %g%% want <= %g%%", res.Loss, arpingLossThreshold)
	}

	return nil
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

// Router returns the Router object in the fixture.
func (tf *TestFixture) Router() *Router {
	return tf.router
}

// Pcap returns the pcap Router object in the fixture.
func (tf *TestFixture) Pcap() *Router {
	return tf.pcap
}

// WifiClient returns the gRPC WifiServiceClient of the DUT.
func (tf *TestFixture) WifiClient() network.WifiServiceClient {
	return tf.wifiClient
}

// DefaultOpenNetworkAP configures the router to provide an 802.11n open network.
func (tf *TestFixture) DefaultOpenNetworkAP(ctx context.Context) (*APIface, error) {
	var secConfFac security.ConfigFactory
	return tf.ConfigureAP(ctx, []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20)}, secConfFac)
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
