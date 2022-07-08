// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	remotewpacli "chromiumos/tast/remote/network/wpacli"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// WiFiDUT defines interface for wificell's DUT device.
type WiFiDUT interface {
	WiFiHost
	// Connect establishes connection to DUT.
	Connect(fullCtx, daemonCtx context.Context, rpcHint *testing.RPCHint, lCfg *loggingConfig, withUI bool) error
	// CompanionDeviceHostname derives the hostname of companion device from test target
	// with the convention in Autotest.
	CompanionDeviceHostName(suffix string) (string, error)
	// SetupNetCertStore sets up netCertStore for EAP-related tests.
	SetupNetCertStore(ctx context.Context) error
	// ResetNetCertStore nullifies netCertStore.
	ResetNetCertStore(ctx context.Context) error
	// NetCertStore returns the current netCertStore.
	NetCertStore() *netcertstore.Store
	// RpcConn returns GRPC connection object.
	RPCConn() *grpc.ClientConn
	// DUT returns underlying DUT object.
	DUT() *dut.DUT
	// Reinit reinits test state fror this DUT.
	Reinit(ctx context.Context) error
	// StartWPAMonitor starts WPA Monitor on the DUT.
	StartWPAMonitor(ctx context.Context) (wpaMonitor *WPAMonitor, stop func(), newCtx context.Context, retErr error)
	// ConnectSSID connects DUT to manually-configured SSID.
	ConnectSSID(ctx context.Context, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error)
	// ConnectAP connects DUT to already configured AP, given by APIface with extra options.
	ConnectAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error)
	// DisconnectWiFi disconnects DUT from the connected AP.
	DisconnectWiFi(ctx context.Context, removeProfile bool) error
	// ReserveForDisconnectWiFi reserves time for disconnection from AP.
	ReserveForDisconnectWiFi(ctx context.Context) (context.Context, context.CancelFunc)
	// AssertNoDisconnect runs the given routine and verifies that no disconnection event
	// is captured in the same duration.
	AssertNoDisconnect(ctx context.Context, f func(context.Context) error) error
	// VerifyConnection verifies that the AP is reachable from this DUT by pinging, and we have the same frequency and subnet as AP's.
	VerifyConnection(ctx context.Context, ap *APIface) error
	// WiFiClient returns WifiClient object.
	WiFiClient() *WifiClient
	// RPC returns RPC Client object.
	RPC() *rpc.Client
	// ClearBSSIDIgnoreDUT clears the BSSID_IGNORE list on DUT.
	ClearBSSIDIgnore(ctx context.Context) error
	// DisablePowersaveMode disables power saving mode (if it's enabled) and return a function to restore its initial mode.
	DisablePowersaveMode(ctx context.Context) (shortenCtx context.Context, restore func() error, err error)
	// WaitWifiConnected waits until WiFi is connected to the Shill profile with specific GUID.
	WaitWifiConnected(ctx context.Context, guid string) error
	// KeyData returns directory and file where connection keys are located.
	KeyData() (string, string)
	// connectCompanion dials SSH connection to companion device with the auth key of DUT.
	connectCompanion(ctx context.Context, hostname string, hostUsers map[string]string, retryDNSNotFound bool) (*ssh.Conn, error)
}

// WiFiDUTImpl implements WiFiDUT interface.
type WiFiDUTImpl struct {
	dut                   *dut.DUT
	rpc                   *rpc.Client
	wifiClient            *WifiClient
	originalLoggingConfig *loggingConfig

	// netCertStore is initialized lazily in ConnectWifi() when needed because it takes about 7 seconds to set up and only a few tests need it.
	netCertStore *netcertstore.Store
}

// loggingConfig contains logging configuration data.
type loggingConfig struct {
	logLevel int
	logTags  []string
}

// ArPingIP sends ARP ping from host to target IP.
func (d *WiFiDUTImpl) ArPingIP(ctx context.Context, targetIP string, opts ...arping.Option) (*arping.Result, error) {
	iface, err := d.WiFiClient().Interface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	ctx, st := timing.Start(ctx, "WiFiDUT.ArPingIP")
	defer st.End()

	pr := remotearping.NewRemoteRunner(d.Conn())
	res, err := pr.Arping(ctx, targetIP, iface, opts...)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	return res, nil

}

// AssertNoDisconnect runs the given routine and verifies that no disconnection event
// is captured in the same duration.
func (d *WiFiDUTImpl) AssertNoDisconnect(ctx context.Context, f func(context.Context) error) error {
	ctx, st := timing.Start(ctx, "tf.AssertNoDisconnect")
	defer st.End()

	el, err := iw.NewEventLogger(ctx, d.DUT())
	if err != nil {
		return errors.Wrap(err, "failed to start iw.EventLogger")
	}
	errf := f(ctx)
	if err := el.Stop(); err != nil {
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

// ClearBSSIDIgnore clears the BSSID_IGNORE list on DUT.
func (d *WiFiDUTImpl) ClearBSSIDIgnore(ctx context.Context) error {
	wpar := remotewpacli.NewRemoteRunner(d.Conn())

	err := wpar.ClearBSSIDIgnore(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to clear WPA BSSID_IGNORE")
	}

	return nil
}

// Close closes connection to DUT.
func (d *WiFiDUTImpl) Close(ctx context.Context) error {
	var firstErr error
	if d.wifiClient != nil {
		if d.originalLoggingConfig != nil {
			if err := d.setLoggingConfig(ctx, *d.originalLoggingConfig); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
			}
		}
		if _, err := d.wifiClient.TearDown(ctx, &empty.Empty{}); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
		}
		d.wifiClient = nil
	}
	if d.RPC() != nil {
		// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
		d.rpc.Close(ctx)
		d.rpc = nil
	}

	return firstErr
}

// CompanionDeviceHostName derives the hostname of companion device from test target
// with the convention in Autotest.
func (d *WiFiDUTImpl) CompanionDeviceHostName(suffix string) (string, error) {
	return d.dut.CompanionDeviceHostname(suffix)
}

// Conn returns SSH connection object to a given DUT.
func (d *WiFiDUTImpl) Conn() *ssh.Conn {
	return d.dut.Conn()
}

// Connect establishes connection to DUT.
func (d *WiFiDUTImpl) Connect(fullCtx, daemonCtx context.Context, rpcHint *testing.RPCHint, lCfg *loggingConfig, withUI bool) error {
	var err error
	d.rpc, err = rpc.Dial(daemonCtx, d.dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect rpc")
	}
	d.wifiClient = &WifiClient{
		ShillServiceClient: wifi.NewShillServiceClient(d.rpc.Conn),
	}

	// TODO(crbug.com/728769): Make sure if we need to turn off powersave.
	if _, err := d.wifiClient.InitDUT(fullCtx, &wifi.InitDUTRequest{WithUi: withUI}); err != nil {
		return errors.Wrap(err, "failed to InitDUT")
	}

	if lCfg != nil {
		cfg, err := d.getLoggingConfig(fullCtx)
		if err != nil {
			return err
		}
		d.originalLoggingConfig = &cfg
		if err := d.setLoggingConfig(fullCtx, *lCfg); err != nil {
			return err
		}
	}
	testing.ContextLogf(fullCtx, "DUT %s connected", d.Name())
	return nil
}

// ConnectAP connects DUT to already configured AP, given by APIface with extra options.
func (d *WiFiDUTImpl) ConnectAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	conf := ap.Config()
	opts := append([]dutcfg.ConnOption{dutcfg.ConnHidden(conf.Hidden), dutcfg.ConnSecurity(conf.SecurityConfig)}, options...)
	return d.ConnectSSID(ctx, conf.SSID, opts...)
}

// ConnectSSID connects DUT to manually-configured SSID.
func (d *WiFiDUTImpl) ConnectSSID(ctx context.Context, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	c := &dutcfg.ConnConfig{
		Ssid:    ssid,
		SecConf: &base.Config{},
	}
	for _, op := range options {
		op(c)
	}
	ctx, st := timing.Start(ctx, "tf.ConnectWifi")
	defer st.End()

	// Setup the NetCertStore only for EAP-related tests.
	if c.SecConf.NeedsNetCertStore() {
		if err := d.SetupNetCertStore(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to set up the NetCertStore")
		}

		if err := c.SecConf.InstallClientCredentials(ctx, d.NetCertStore()); err != nil {
			return nil, errors.Wrap(err, "failed to install client credentials")
		}
	}

	secProps, err := c.SecConf.ShillServiceProperties()
	if err != nil {
		return nil, err
	}

	props := make(map[string]interface{})
	for k, v := range c.Props {
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
		Ssid:       []byte(c.Ssid),
		Hidden:     c.Hidden,
		Security:   c.SecConf.Class(),
		Shillprops: propsEnc,
	}
	if d.WiFiClient() == nil {
		return nil, errors.New("WiFiClient uninitialized")
	}
	response, err := d.WiFiClient().Connect(ctx, request)
	if err != nil {
		return nil, errors.Wrapf(err, "client failed to connect to WiFi network with SSID %q", c.Ssid)
	}
	return response, nil
}

// DisablePowersaveMode disables power saving mode (if it's enabled) and return a function to restore its initial mode.
func (d *WiFiDUTImpl) DisablePowersaveMode(ctx context.Context) (shortenCtx context.Context, restore func() error, err error) {
	iwr := iw.NewRemoteRunner(d.Conn())
	iface, err := d.WiFiClient().Interface(ctx)
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

// DisconnectWiFi disconnects DUT from the connected AP.
func (d *WiFiDUTImpl) DisconnectWiFi(ctx context.Context, removeProfile bool) error {
	ctx, st := timing.Start(ctx, "tf.disconnectWifi")
	defer st.End()

	resp, err := d.WiFiClient().SelectedService(ctx, &empty.Empty{})
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
	if _, err := d.WiFiClient().Disconnect(ctx, req); err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}

// DUT returns underlying DUT object.
func (d *WiFiDUTImpl) DUT() *dut.DUT {
	return d.dut
}

// HwAddr returns Hardware address of the wifi-related interface.
func (d *WiFiDUTImpl) HwAddr(ctx context.Context) (net.HardwareAddr, error) {
	iface, err := d.WiFiClient().Interface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetHardwareAddrRequest{
		InterfaceName: iface,
	}
	resp, err := d.WiFiClient().GetHardwareAddr(ctx, netIface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the HardwareAddr")
	}

	return net.ParseMAC(resp.HwAddr)
}

// IPv4Addrs returns IPv4 addresses of the wifi-related interface.
func (d *WiFiDUTImpl) IPv4Addrs(ctx context.Context) ([]net.IP, error) {
	iface, err := d.WiFiClient().Interface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetIPv4AddrsRequest{
		InterfaceName: iface,
	}
	addrs, err := d.WiFiClient().GetIPv4Addrs(ctx, netIface)
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

// KeyData returns directory and file where connection keys are located.
func (d *WiFiDUTImpl) KeyData() (string, string) {
	return d.dut.KeyDir(), d.dut.KeyFile()
}

// Name returns host name.
func (d *WiFiDUTImpl) Name() string {
	return d.dut.HostName()
}

// NetCertStore returns the current netCertStore.
func (d *WiFiDUTImpl) NetCertStore() *netcertstore.Store {
	return d.netCertStore
}

// PingIP sends ICMP ping from host to target IP.
func (d *WiFiDUTImpl) PingIP(ctx context.Context, targetIP string, opts ...ping.Option) (*ping.Result, error) {
	iface, err := d.WiFiClient().Interface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	// Bind ping used in all WiFi Tests to WiFiInterface. Otherwise if the
	// WiFi interface is not up yet they will be routed through the Ethernet
	// interface. Also see b/225205611 for details.
	opts = append(opts, ping.BindAddress(true), ping.SourceIface(iface))

	ctx, st := timing.Start(ctx, "WiFiDUT.PingIP")
	defer st.End()

	pr := remoteping.NewRemoteRunner(d.Conn())
	res, err := pr.Ping(ctx, targetIP, opts...)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	return res, nil

}

// Reinit reinits test state fror this DUT.
func (d *WiFiDUTImpl) Reinit(ctx context.Context) error {
	if _, err := d.WiFiClient().ReinitTestState(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reinit DUT")
	}
	return nil
}

// ReserveForDisconnectWiFi reserves time for disconnection from AP.
func (d *WiFiDUTImpl) ReserveForDisconnectWiFi(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

// ResetNetCertStore nullifies netCertStore.
func (d *WiFiDUTImpl) ResetNetCertStore(ctx context.Context) error {
	if d.netCertStore == nil {
		// Nothing to do if it was not set up.
		return nil
	}

	err := d.netCertStore.Cleanup(ctx)
	d.netCertStore = nil
	return err
}

// RPC returns RPC Client object.
func (d *WiFiDUTImpl) RPC() *rpc.Client {
	return d.rpc
}

// RPCConn returns GRPC connection object.
func (d *WiFiDUTImpl) RPCConn() *grpc.ClientConn {
	return d.rpc.Conn
}

// SetupNetCertStore sets up netCertStore for EAP-related tests.
func (d *WiFiDUTImpl) SetupNetCertStore(ctx context.Context) error {
	if d.netCertStore != nil {
		// Nothing to do if it was set up.
		return nil
	}

	runner := hwsec.NewCmdRunner(d.dut)
	var err error
	d.netCertStore, err = netcertstore.CreateStore(ctx, runner)
	return err
}

// StartWPAMonitor starts WPA Monitor on the DUT.
func (d *WiFiDUTImpl) StartWPAMonitor(ctx context.Context) (wpaMonitor *WPAMonitor, stop func(), newCtx context.Context, retErr error) {
	wpaMonitor = new(WPAMonitor)
	if err := wpaMonitor.Start(ctx, d.Conn()); err != nil {
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

// VerifyConnection verifies that the AP is reachable from this DUT by pinging, and we have the same frequency and subnet as AP's.
func (d *WiFiDUTImpl) VerifyConnection(ctx context.Context, ap *APIface) error {
	// Check frequency.
	service, err := d.WiFiClient().QueryService(ctx)
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
	addrs, err := d.IPv4Addrs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get client ipv4 addresses")
	}
	serverSubnet := ap.ServerSubnet()
	foundSubnet := false
	for _, a := range addrs {
		if serverSubnet.Contains(a) {
			foundSubnet = true
			break
		}
	}
	if !foundSubnet {
		return errors.Errorf("subnet does not match, got addrs %v want %s", addrs, serverSubnet.String())
	}

	// Perform ping.
	res, err := d.PingIP(ctx, ap.ServerIP().String(), ping.Interval(0.1))
	if err != nil {
		return errors.Wrap(err, "failed to ping from the DUT")
	}

	if err := VerifyPingResults(res, pingLossThreshold); err != nil {
		return errors.Wrap(err, "ping verification failed")
	}

	return nil
}

// WaitWifiConnected waits until WiFi is connected to the Shill profile with specific GUID.
func (d *WiFiDUTImpl) WaitWifiConnected(ctx context.Context, guid string) error {
	testing.ContextLogf(ctx, "Waiting for WiFi to be connected from DUT #%v to profile with GUID: %s", d.Name(), guid)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		req := &wifi.RequestScansRequest{Count: 1}
		if _, err := d.WiFiClient().RequestScans(ctx, req); err != nil {
			errors.Wrap(err, "failed to request scan")
		}

		serInfo, err := d.WiFiClient().QueryService(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the WiFi service information from DUT #%v", d.Name())
		}

		if guid == serInfo.Guid && serInfo.IsConnected {
			iface, err := d.WiFiClient().Interface(ctx)
			if err != nil {
				return errors.Wrapf(err, "failed to get interface from the DUT #%v", d.Name())
			}

			addrs, err := d.WiFiClient().GetIPv4Addrs(ctx, &wifi.GetIPv4AddrsRequest{InterfaceName: iface})
			if err != nil {
				return errors.Wrapf(err, "failed to get client #%v IPv4 addresses", d.Name())
			}
			if len(addrs.Ipv4) > 0 {
				return nil
			}
			return errors.New("IPv4 address not assigned yet")
		} else if guid != serInfo.Guid {
			return errors.New("GUID does not match, current service is: " + serInfo.Guid)
		}
		return errors.New("Service is not connected")
	}, &testing.PollOptions{
		Timeout:  time.Minute,
		Interval: time.Second,
	}); err != nil {
		return errors.Wrap(err, "no matching GUID service selected")
	}

	testing.ContextLog(ctx, "WiFi connected")
	return nil
}

// WiFiClient returns WifiClient object.
func (d *WiFiDUTImpl) WiFiClient() *WifiClient {
	return d.wifiClient
}

// getLoggingConfig returns current logging configuration.
func (d *WiFiDUTImpl) getLoggingConfig(ctx context.Context) (loggingConfig, error) {
	currentConfig, err := d.wifiClient.GetLoggingConfig(ctx, &empty.Empty{})
	if err != nil {
		return loggingConfig{0, nil}, err
	}
	return loggingConfig{int(currentConfig.DebugLevel), currentConfig.DebugTags}, err
}

// setLoggingConfig configures the logging setting with the specified values (level and tags).
func (d *WiFiDUTImpl) setLoggingConfig(ctx context.Context, cfg loggingConfig) error {
	testing.ContextLogf(ctx, "Configuring logging level: %d, tags: %v for %s", cfg.logLevel, cfg.logTags, d.Name())
	_, err := d.wifiClient.SetLoggingConfig(ctx, &wifi.SetLoggingConfigRequest{DebugLevel: int32(cfg.logLevel), DebugTags: cfg.logTags})
	return err
}

// connectCompanion dials SSH connection to companion device with the auth key of DUT.
func (d *WiFiDUTImpl) connectCompanion(ctx context.Context, hostname string, hostUsers map[string]string, retryDNSNotFound bool) (*ssh.Conn, error) {
	var sopt ssh.Options
	ssh.ParseTarget(hostname, &sopt)
	// Assumption is, that the key will be shared between DUTs.
	sopt.KeyDir, sopt.KeyFile = d.KeyData()
	sopt.ConnectTimeout = 10 * time.Second

	var conn *ssh.Conn

	if hostUsers != nil {
		if username, ok := hostUsers[hostname]; ok {
			testing.ContextLogf(ctx, "Using ssh username override %q for host %q", username, hostname)
			sopt.User = username
		}
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		var dnsErr *net.DNSError

		conn, err = ssh.New(ctx, &sopt)
		if !retryDNSNotFound && errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			// Don't retry DNS not found case.
			return testing.PollBreak(err)
		}
		return err
	}, &testing.PollOptions{
		Timeout: time.Minute,
	}); err != nil {
		return nil, err
	}

	return conn, nil
}
