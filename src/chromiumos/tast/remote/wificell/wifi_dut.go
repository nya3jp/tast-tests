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

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	"chromiumos/tast/remote/network/iw"
	remotewpacli "chromiumos/tast/remote/network/wpacli"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type WiFiDUT interface {
	WiFiHost
	Connect(fullCtx, daemonCtx context.Context, rpcHint *testing.RPCHint, lCfg *loggingConfig, withUI bool) error
	CompanionDeviceHostName(suffix string) (string, error)
	SetupNetCertStore(ctx context.Context) error
	ResetNetCertStore(ctx context.Context) error
	NetCertStore() *netcertstore.Store
	RpcConn() *grpc.ClientConn
	DUT() *dut.DUT
	Reinit(ctx context.Context) error
	StartWPAMonitor(ctx context.Context) (wpaMonitor *WPAMonitor, stop func(), newCtx context.Context, retErr error)
	ConnectSSID(ctx context.Context, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error)
	ConnectAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error)
	DisconnectWiFi(ctx context.Context, removeProfile bool) error
	// CleanDisconnectWiFi()
	ReserveForDisconnectWiFi(ctx context.Context) (context.Context, context.CancelFunc)
	AssertNoDisconnect(ctx context.Context, f func(context.Context) error) error
	WiFiClient() *WifiClient
	RPC() *rpc.Client
	// VerifyConnection()
	ClearBSSIDIgnore(ctx context.Context) error
	// SendChannelSwitchAnnouncement(*WiFiAP)
	DisablePowersaveMode(ctx context.Context) (shortenCtx context.Context, restore func() error, err error)
	// SetWakeOnWifi()
	WaitWifiConnected(ctx context.Context, guid string) error
	KeyData() (string, string)
	connectCompanion(ctx context.Context, hostname string, hostUsers map[string]string, retryDNSNotFound bool) (*ssh.Conn, error)
}

type WiFiDUTImpl struct {
	WiFiHost
	dut                   *dut.DUT
	rpc                   *rpc.Client
	wifiClient            *WifiClient
	originalLoggingConfig *loggingConfig

	// netCertStore is initialized lazily in ConnectWifi() when needed because it takes about 7 seconds to set up and only a few tests need it.
	netCertStore *netcertstore.Store
}

type loggingConfig struct {
	logLevel int
	logTags  []string
}

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

func (d *WiFiDUTImpl) ClearBSSIDIgnore(ctx context.Context) error {
	wpar := remotewpacli.NewRemoteRunner(d.Conn())

	err := wpar.ClearBSSIDIgnore(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to clear WPA BSSID_IGNORE")
	}

	return nil
}

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

func (d *WiFiDUTImpl) CompanionDeviceHostName(suffix string) (string, error) {
	return d.dut.CompanionDeviceHostname(suffix)
}

func (d *WiFiDUTImpl) Conn() *ssh.Conn {
	return d.dut.Conn()
}

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

func (d *WiFiDUTImpl) ConnectAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	conf := ap.Config()
	opts := append([]dutcfg.ConnOption{dutcfg.ConnHidden(conf.Hidden), dutcfg.ConnSecurity(conf.SecurityConfig)}, options...)
	return d.ConnectSSID(ctx, conf.SSID, opts...)
}

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

func (d *WiFiDUTImpl) DUT() *dut.DUT {
	return d.dut
}

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

func (d *WiFiDUTImpl) KeyData() (string, string) {
	return d.dut.KeyDir(), d.dut.KeyFile()
}

func (d *WiFiDUTImpl) Name() string {
	return d.dut.HostName()
}

func (d *WiFiDUTImpl) NetCertStore() *netcertstore.Store {
	return d.netCertStore
}

func (d *WiFiDUTImpl) Reinit(ctx context.Context) error {
	if _, err := d.WiFiClient().ReinitTestState(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reinit DUT")
	}
	return nil
}

func (d *WiFiDUTImpl) ReserveForDisconnectWiFi(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}

func (d *WiFiDUTImpl) ResetNetCertStore(ctx context.Context) error {
	if d.netCertStore == nil {
		// Nothing to do if it was not set up.
		return nil
	}

	err := d.netCertStore.Cleanup(ctx)
	d.netCertStore = nil
	return err
}

func (d *WiFiDUTImpl) RPC() *rpc.Client {
	return d.rpc
}

func (d *WiFiDUTImpl) RpcConn() *grpc.ClientConn {
	return d.rpc.Conn
}

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

func (d *WiFiDUTImpl) WiFiClient() *WifiClient {
	return d.wifiClient
}

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
