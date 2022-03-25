// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
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
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/hwsec"
	remotearping "chromiumos/tast/remote/network/arping"
	remoteip "chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	remotewpacli "chromiumos/tast/remote/network/wpacli"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/router/ax"
	"chromiumos/tast/remote/wificell/router/common"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/router/legacy"
	"chromiumos/tast/remote/wificell/router/openwrt"
	"chromiumos/tast/remote/wificell/tethering"
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

// The amount of time to wait before reconnecting after a router reboot.
const routerPostRebootWaitTime = 2 * time.Minute

// TODO(b/232150137): Using a different subnet than other ip addrs in Tast.
// Move all hardcoded ip addresses to one file to avoid collision.
const (
	p2pGOIPAddress     string = "192.160.0.1"
	p2pClientIPAddress string = "192.160.0.2"
)

// TFOption is the function signature used to modify TextFixutre.
type TFOption func(*TestFixture)

// TFRouter sets the router hostname for the test fixture.
// Format: hostname[:port]
func TFRouter(targets ...string) TFOption {
	return func(tf *TestFixture) {
		tf.routers = make([]*routerData, len(targets))
		for i := range targets {
			tf.routers[i] = &routerData{
				target: targets[i],
			}
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
func TFRouterType(rtype support.RouterType) TFOption {
	return func(tf *TestFixture) {
		tf.routerType = rtype
	}
}

// TFPcapType sets the router type of the pcap capturing device. The pcap device in our testbeds is a router.
func TFPcapType(rtype support.RouterType) TFOption {
	return func(tf *TestFixture) {
		tf.pcapType = rtype
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
		tf.duts = append(tf.duts, &dutData{dut: cd})
	}
}

const (
	// TFServiceName is the service needed by TestFixture.
	TFServiceName = "tast.cros.wifi.ShillService"
	// DefaultDUT is the default DUT index (0).
	DefaultDUT = 0
	// PeerDUT is the peer DUT index (1).
	PeerDUT = 1
)

// TODO(b/234845693): make that an independent structure.
type routerData struct {
	target string
	host   *ssh.Conn
	object router.Base
}

// TODO(b/234845693): make that an independent structure.
type dutData struct {
	dut              *dut.DUT
	rpc              *rpc.Client
	wifiClient       *WifiClient
	originalLogLevel int
	originalLogTags  []string

	// netCertStore is initialized lazily in ConnectWifi() when needed because it takes about 7 seconds to set up and only a few tests need it.
	netCertStore *netcertstore.Store
}

// P2PDevice is used as p2p device type.
type P2PDevice int32

// P2P devices (options for Group Owner (GO) and client).
const (
	P2PDeviceDUT P2PDevice = iota
	P2PDeviceCompanionDUT
	// TODO(b/231261132): add Android phones as GO/Client options.
)

// DutIdx is the type used for DUT Index.
type DutIdx int

// TestFixture sets up the context for a basic WiFi test.
type TestFixture struct {
	duts       []*dutData
	hostUsers  map[string]string
	routers    []*routerData
	routerType support.RouterType
	pcapType   support.RouterType

	pcapTarget string
	pcapHost   *ssh.Conn
	pcap       router.Base

	attenuatorTarget string
	attenuator       *attenuator.Attenuator

	setLogging bool
	logLevel   int
	logTags    []string

	// The following parameters (with prefix p2p*) are used with P2P tests.
	p2pGO              *dut.DUT
	p2pClient          *dut.DUT
	p2pGOIface         string
	p2pGroupSSID       string
	p2pGroupPassphrase string
	p2pClientIface     string

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
}

// connectCompanion dials SSH connection to companion device with the auth key of DUT.
func (tf *TestFixture) connectCompanion(ctx context.Context, hostname string, retryDNSNotFound bool) (*ssh.Conn, error) {
	var sopt ssh.Options
	ssh.ParseTarget(hostname, &sopt)
	// Assumption is, that the key will be shared between DUTs.
	sopt.KeyDir = tf.duts[DefaultDUT].dut.KeyDir()
	sopt.KeyFile = tf.duts[DefaultDUT].dut.KeyFile()

	var conn *ssh.Conn

	if tf.hostUsers != nil {
		if username, ok := tf.hostUsers[hostname]; ok {
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
		Timeout: 5 * time.Minute,
	}); err != nil {
		return nil, err
	}

	return conn, nil
}

// setupNetCertStore sets up tf.netCertStore for EAP-related tests.
func (tf *TestFixture) setupNetCertStore(ctx context.Context, dutIdx DutIdx) error {
	if tf.duts[dutIdx].netCertStore != nil {
		// Nothing to do if it was set up.
		return nil
	}

	runner := hwsec.NewCmdRunner(tf.duts[dutIdx].dut)
	var err error
	tf.duts[dutIdx].netCertStore, err = netcertstore.CreateStore(ctx, runner)
	return err
}

// resetNetCertStore nullifies tf.netCertStore.
func (tf *TestFixture) resetNetCertStore(ctx context.Context, dutIdx DutIdx) error {
	if tf.duts[dutIdx].netCertStore == nil {
		// Nothing to do if it was not set up.
		return nil
	}

	err := tf.duts[dutIdx].netCertStore.Cleanup(ctx)
	tf.duts[dutIdx].netCertStore = nil
	return err
}

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
		duts:      []*dutData{{dut: d}},
		capturers: make(map[*APIface]*pcap.Capturer),
		aps:       make(map[*APIface]struct{}),
		// Set the router's default router type.
		routerType: support.LegacyT,
		// Set the pcap capture device's default router type.
		pcapType: support.LegacyT,
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

	for _, d := range tf.duts {
		var err error
		d.rpc, err = rpc.Dial(daemonCtx, d.dut, rpcHint)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect rpc")
		}
		d.wifiClient = &WifiClient{
			ShillServiceClient: wifi.NewShillServiceClient(d.rpc.Conn),
		}

		// TODO(crbug.com/728769): Make sure if we need to turn off powersave.
		if _, err := d.wifiClient.InitDUT(ctx, &wifi.InitDUTRequest{WithUi: tf.option.withUI}); err != nil {
			return nil, errors.Wrap(err, "failed to InitDUT")
		}

		if tf.setLogging {
			d.originalLogLevel, d.originalLogTags, err = tf.getLoggingConfig(ctx, d.wifiClient)
			if err != nil {
				return nil, err
			}
			if err := setLoggingConfig(ctx, d.wifiClient, tf.logLevel, tf.logTags); err != nil {
				return nil, err
			}
		}
	}

	// Wificell precondition always provides us with router name, but we need
	// to handle case when the fixture is created from outside of the precondition.
	if len(tf.routers) == 0 {
		testing.ContextLog(ctx, "Using default router name")
		name, err := tf.duts[DefaultDUT].dut.CompanionDeviceHostname(dut.CompanionSuffixRouter)
		if err != nil {
			return nil, errors.Wrap(err, "failed to synthesize default router name")
		}
		tf.routers = append(tf.routers, &routerData{target: name})
	}
	for i := range tf.routers {
		rt := tf.routers[i]
		testing.ContextLogf(ctx, "Adding router %s", rt.target)
		routerHost, err := tf.connectCompanion(ctx, rt.target, true /* allow retry */)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to connect to the router %s", rt.target)
		}
		rt.host = routerHost
		routerObj, err := newRouter(ctx, daemonCtx, rt.host,
			strings.ReplaceAll(rt.target, ":", "_"), tf.routerType)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a router object")
		}
		testing.ContextLogf(ctx, "Successfully instantiated %s router controller for router[%d]", routerObj.RouterType().String(), i)
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
		var err error
		testing.ContextLog(ctx, "Using default pcap name")
		tf.pcapTarget, err = tf.duts[DefaultDUT].dut.CompanionDeviceHostname(dut.CompanionSuffixPcap)
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
		var err error
		tf.pcapHost, err = tf.connectCompanion(ctx, tf.pcapTarget, false /* no retry when DNS not found */)
		if err != nil {
			// We want to fallback to use router as pcap iff the default
			// pcap hostname is invalid. Fail here if it's not the case.
			if !useDefaultPcap || !errInvalidHost(err) {
				return nil, errors.Wrap(err, "failed to connect to pcap")
			}
		} else {
			tf.pcap, err = newRouter(ctx, daemonCtx, tf.pcapHost, "pcap", tf.pcapType)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create a router object for pcap")
			}
			testing.ContextLogf(ctx, "Successfully instantiated %s router controller for pcap", tf.pcap.RouterType().String())
			// Validate that the pcap router actually supports pcap
			if _, ok := tf.pcap.(support.Capture); !ok {
				return nil, errors.Errorf("router type %q does not support Capture", tf.pcap.RouterType().String())
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
		var err error
		// openWrtRouter #0 should always be present, thus we use it as a proxy.
		tf.attenuator, err = attenuator.Open(ctx, tf.attenuatorTarget, tf.routers[0].host)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open attenuator")
		}
	}

	// Seed the random as we have some randomization. e.g. default SSID.
	rand.Seed(time.Now().UnixNano())

	// Reinitialize state of routers.
	if err := tf.ReinitRouters(ctx); err != nil {
		return nil, err
	}
	return tf, nil
}

// NumberOfDUTs returns number of DUTs handled by this fixture.
func (tf *TestFixture) NumberOfDUTs() int {
	return len(tf.duts)
}

// DUT returns particular DUT.
func (tf *TestFixture) DUT(dutIdx DutIdx) *dut.DUT {
	return tf.duts[dutIdx].dut
}

// DUTConn returns connection object to particular DUT.
func (tf *TestFixture) DUTConn(dutIdx DutIdx) *ssh.Conn {
	return tf.duts[dutIdx].dut.Conn()
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
		r, ok := rt.object.(support.Logs)
		if !ok {
			return errors.Errorf("router type %q does not support Logs", rt.object.RouterType().String())
		}
		err := r.CollectLogs(ctx)
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
	for i := range tf.duts {
		if err := tf.resetNetCertStore(ctx, DutIdx(i)); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to reset the NetCertStore"))
		}
	}

	if tf.attenuator != nil {
		tf.attenuator.Close()
		tf.attenuator = nil
	}

	// Check if one of routers was used in dual-purpose (router&pcap) mode.
	if tf.pcap != nil {
		for i := range tf.routers {
			rt := tf.routers[i]
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
		router := tf.routers[i]
		if router.object != nil {
			if err := router.object.Close(ctx); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close router %s", router.target))
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
	for _, d := range tf.duts {
		if d.wifiClient != nil {
			if tf.setLogging {
				if err := setLoggingConfig(ctx, d.wifiClient, d.originalLogLevel, d.originalLogTags); err != nil {
					wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
				}
			}
			if _, err := d.wifiClient.TearDown(ctx, &empty.Empty{}); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to tear down test state"))
			}
			d.wifiClient = nil
		}
		if d.rpc != nil {
			// Ignore the error of rpc.Close as aborting rpc daemon will always have error.
			d.rpc.Close(ctx)
			d.rpc = nil
		}
	}

	// Do not close DUT, it'll be closed by the framework.
	return firstErr
}

// Reinit re-initializes the TestFixture by calling both ReinitDUT and
// ReinitRouters. This can be used in precondition or between testcases to
// guarantee a cleaner state.
func (tf *TestFixture) Reinit(ctx context.Context) error {
	ctx, t := timing.Start(ctx, "Reinit")
	defer t.End()
	if err := tf.ReinitDUT(ctx); err != nil {
		return errors.Wrap(err, "failed to reinit DUT")
	}
	if err := tf.ReinitRouters(ctx); err != nil {
		return errors.Wrap(err, "failed to reinit routers")
	}
	return nil
}

// ReinitDUT re-initializes the DUT.
func (tf *TestFixture) ReinitDUT(ctx context.Context) error {
	ctx, t := timing.Start(ctx, "ReinitDUT")
	defer t.End()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := tf.WifiClient().HealthCheck(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to pass wifi client health check")
	}
	if _, err := tf.WifiClient().ReinitTestState(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reinit wifi client test state")
	}
	return nil
}

// ReinitRouters re-initializes the routers.
func (tf *TestFixture) ReinitRouters(ctx context.Context) error {
	ctx, t := timing.Start(ctx, "ReinitRouters")
	defer t.End()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := tf.DeconfigAllAPs(ctx); err != nil {
		return errors.Wrap(err, "failed to deconfig all APs")
	}
	// Reboot all OpenWrt routers and reconnect to them.
	for _, rd := range tf.routers {
		if rd.object.RouterType() != support.OpenWrtT {
			continue
		}
		if err := tf.rebootRouter(ctx, rd); err != nil {
			return err
		}
	}
	return nil
}

func (tf *TestFixture) rebootRouter(ctx context.Context, rd *routerData) error {
	ctx, t := timing.Start(ctx, "rebootRouter_"+rd.object.RouterType().String())
	defer t.End()
	routerName := rd.object.RouterName()
	routerType := rd.object.RouterType()
	routerMsgName := fmt.Sprintf("%s router %q", routerType.String(), routerName)

	// Close and reboot router.
	testing.ContextLogf(ctx, "Preparing %s for reboot", routerMsgName)
	if err := rd.object.Close(ctx); err != nil {
		testing.ContextLogf(ctx, "Failed to close %s before reboot, err: %v", routerMsgName, err)
	}
	testing.ContextLogf(ctx, "Rebooting %s", routerMsgName)
	if err := rd.object.StartReboot(ctx); err != nil {
		return errors.Wrapf(err, "failed to reboot %s", routerMsgName)
	}
	_ = rd.host.Close(ctx)
	rd.host = nil
	rd.object = nil

	// Wait for router reboot to complete and for the router to be ready for use.
	// Currently, there's no reliable way to identify router state is stabilized
	// enough to run tests, so as a short term work around, just Sleep for fixed
	// amount of time which is considered long enough to stabilize the reboot.
	// TODO(b/239583375): Replace this simple wait with a more optimized process.
	testing.ContextLogf(ctx, "Waiting %s before trying to reconnect to %s", routerPostRebootWaitTime, routerMsgName)
	if err := testing.Sleep(ctx, routerPostRebootWaitTime); err != nil {
		return errors.Wrapf(err, "failed to wait for %s after rebooting %s", routerPostRebootWaitTime, routerMsgName)
	}

	// Reconnect to router and create a new router controller.
	testing.ContextLogf(ctx, "Reconnecting to %s", routerMsgName)
	routerHost, err := tf.connectCompanion(ctx, rd.target, true)
	if err != nil {
		return errors.Wrapf(err, "failed to reconnect to %s after reboot", routerMsgName)
	}
	rd.host = routerHost
	testing.ContextLogf(ctx, "Reconnected to %s", routerMsgName)
	routerObject, err := newRouter(ctx, ctx, rd.host, routerName, routerType)
	if err != nil {
		return errors.Wrapf(err, "failed to recreate %s", routerMsgName)
	}
	rd.object = routerObject
	testing.ContextLogf(ctx, "Reconnected to %s with new router controller after reboot", routerMsgName)
	return nil
}

// UniqueAPName returns a unique ID string for each AP as their name, so that related
// logs/pcap can be identified easily.
func (tf *TestFixture) UniqueAPName() string {
	id := strconv.Itoa(tf.apID)
	tf.apID++
	return id
}

// ConfigureAPOnRouterID is an extended version of ConfigureAP, allowing to choose router
// to establish the AP on.
func (tf *TestFixture) ConfigureAPOnRouterID(ctx context.Context, idx int, ops []hostapd.Option, fac security.ConfigFactory, enableDNS, enableHTTP bool) (ret *APIface, retErr error) {
	ctx, st := timing.Start(ctx, "tf.ConfigureAP")
	defer st.End()
	r := tf.routers[idx].object
	name := tf.UniqueAPName()

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
		return nil, errors.Errorf("router index (%d) out of range [0, %d)", idx, len(tf.routers))
	}

	var capturer *pcap.Capturer
	if tf.option.packetCapture {
		freqOps, err := config.PcapFreqOptions()
		if err != nil {
			return nil, err
		}
		p, ok := tf.pcap.(support.Capture)
		if !ok {
			return nil, errors.Errorf("pcap device with router type %q does not have log capture support", tf.pcap.RouterType().String())
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

	ap, err := StartAPIface(ctx, r, name, config, enableDNS, enableHTTP)
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
	return tf.ConfigureAPOnRouterID(ctx, 0, ops, fac, false, false)
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
		if p, ok := tf.pcap.(support.Capture); !ok {
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
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()
	p, ok := tf.pcap.(support.Capture)
	if !ok {
		return errors.Errorf("router type %q does not support Capture", tf.pcap.RouterType().String())
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
func (tf *TestFixture) StartWPAMonitor(ctx context.Context, dutIdx DutIdx) (wpaMonitor *wpacli.WPAMonitor, stop func(), newCtx context.Context, retErr error) {
	wpaMonitor = new(wpacli.WPAMonitor)
	stop, newCtx, err := wpaMonitor.StartWPAMonitor(ctx, tf.duts[dutIdx].dut.Conn(), wpaMonitorStopTimeout)
	if err != nil {
		return nil, nil, ctx, err
	}
	return wpaMonitor, stop, newCtx, nil
}

// Capturer returns the auto-spawned Capturer for the APIface instance.
func (tf *TestFixture) Capturer(ap *APIface) (*pcap.Capturer, bool) {
	capturer, ok := tf.capturers[ap]
	return capturer, ok
}

// ConnectWifi is backwards-compatible version of ConnectWifiFromDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ConnectWifi(ctx context.Context, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.ConnectWifiFromDUT(ctx, DefaultDUT, ssid, options...)
}

// ConnectWifiFromDUT asks the DUT #dutIdx to connect to the specified WiFi.
func (tf *TestFixture) ConnectWifiFromDUT(ctx context.Context, dutIdx DutIdx, ssid string, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	c := &dutcfg.ConnConfig{
		Ssid:    ssid,
		SecConf: &base.Config{},
	}
	for _, op := range options {
		op(c)
	}
	ctx, st := timing.Start(ctx, "tf.ConnectWifiFromDUT")
	defer st.End()

	// Setup the NetCertStore only for EAP-related tests.
	if c.SecConf.NeedsNetCertStore() {
		if err := tf.setupNetCertStore(ctx, dutIdx); err != nil {
			return nil, errors.Wrap(err, "failed to set up the NetCertStore")
		}

		if err := c.SecConf.InstallClientCredentials(ctx, tf.duts[dutIdx].netCertStore); err != nil {
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
		Ssid:          []byte(c.Ssid),
		Hidden:        c.Hidden,
		SecurityClass: c.SecConf.Class(),
		Shillprops:    propsEnc,
	}
	response, err := tf.duts[dutIdx].wifiClient.Connect(ctx, request)
	if err != nil {
		return nil, errors.Wrapf(err, "client failed to connect to WiFi network with SSID %q", c.Ssid)
	}
	return response, nil
}

// ConnectWifiAPFromDUT asks the given DUT to connect to the WiFi provided by the given AP.
func (tf *TestFixture) ConnectWifiAPFromDUT(ctx context.Context, dutIdx DutIdx, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	conf := ap.Config()
	opts := append([]dutcfg.ConnOption{dutcfg.ConnHidden(conf.Hidden), dutcfg.ConnSecurity(conf.SecurityConfig)}, options...)
	return tf.ConnectWifiFromDUT(ctx, dutIdx, conf.SSID, opts...)
}

// ConnectWifiAP is backwards-compatible version of ConnectWifiAPFromDUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ConnectWifiAP(ctx context.Context, ap *APIface, options ...dutcfg.ConnOption) (*wifi.ConnectResponse, error) {
	return tf.ConnectWifiAPFromDUT(ctx, DefaultDUT, ap, options...)
}

func (tf *TestFixture) disconnectWifi(ctx context.Context, dutIdx DutIdx, removeProfile bool) error {
	ctx, st := timing.Start(ctx, "tf.disconnectWifi")
	defer st.End()

	resp, err := tf.duts[dutIdx].wifiClient.SelectedService(ctx, &empty.Empty{})
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
	if _, err := tf.duts[dutIdx].wifiClient.Disconnect(ctx, req); err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}

// DisconnectWifi is backwards-compatible version of DisconnectDUTFromWifi. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) DisconnectWifi(ctx context.Context) error {
	return tf.disconnectWifi(ctx, DefaultDUT, false)
}

// CleanDisconnectWifi is backwards-compatible version of CleanDisconnectDUTFromWifi. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) CleanDisconnectWifi(ctx context.Context) error {
	return tf.disconnectWifi(ctx, DefaultDUT, true)
}

// DisconnectDUTFromWifi asks the given DUT to disconnect from current WiFi service.
func (tf *TestFixture) DisconnectDUTFromWifi(ctx context.Context, dutIdx DutIdx) error {
	return tf.disconnectWifi(ctx, dutIdx, false)
}

// CleanDisconnectDUTFromWifi asks the given DUT to disconnect from current WiFi service and removes the configuration.
func (tf *TestFixture) CleanDisconnectDUTFromWifi(ctx context.Context, dutIdx DutIdx) error {
	return tf.disconnectWifi(ctx, dutIdx, true)
}

// ReserveForDisconnect returns a shorter ctx and cancel function for tf.DisconnectWifi.
func (tf *TestFixture) ReserveForDisconnect(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
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
	iface, err := tf.DUTClientInterface(ctx, dutIdx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	// Bind ping used in all WiFi Tests to WiFiInterface. Otherwise if the
	// WiFi interface is not up yet they will be routed through the Ethernet
	// interface. Also see b/225205611 for details.
	opts = append(opts, ping.BindAddress(true), ping.SourceIface(iface))

	ctx, st := timing.Start(ctx, "tf.PingFromSpecificDUT")
	defer st.End()

	pr := remoteping.NewRemoteRunner(tf.duts[dutIdx].dut.Conn())
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

	pr := remoteping.NewRemoteRunner(tf.routers[idx].host)
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
	ctx, st := timing.Start(ctx, "tf.ArpingFromSpecificDUT")
	defer st.End()

	iface, err := tf.DUTClientInterface(ctx, dutIdx)
	if err != nil {
		return errors.Wrap(err, "failed to get the client WiFi interface")
	}

	runner := remotearping.NewRemoteRunner(tf.duts[dutIdx].dut.Conn())
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
	ctx, st := timing.Start(ctx, "tf.ArpingFromRouterID")
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

// ClientIPv4Addrs is backwards-compatible version of DUTIPv4Addrs. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) ClientIPv4Addrs(ctx context.Context) ([]net.IP, error) {
	return tf.DUTIPv4Addrs(ctx, DefaultDUT)
}

// DUTIPv4Addrs returns the IPv4 addresses for the network interface.
func (tf *TestFixture) DUTIPv4Addrs(ctx context.Context, dutIdx DutIdx) ([]net.IP, error) {
	iface, err := tf.DUTClientInterface(ctx, dutIdx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}

	netIface := &wifi.GetIPv4AddrsRequest{
		InterfaceName: iface,
	}
	addrs, err := tf.DUTWifiClient(dutIdx).GetIPv4Addrs(ctx, netIface)
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
	ctx, st := timing.Start(ctx, "tf.AssertNoDisconnect")
	defer st.End()

	el, err := iw.NewEventLogger(ctx, tf.duts[dutIdx].dut)
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

// RouterByID returns the respective router object in the fixture.
func (tf *TestFixture) RouterByID(idx int) router.Base {
	return tf.routers[idx].object
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
	return tf.pcap
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
	return tf.duts[dutIdx].wifiClient
}

// RPC returns the gRPC connection of the default DUT. Deprecated.
// TODO(b/234845693): remove after stabilizing period.
func (tf *TestFixture) RPC() *rpc.Client {
	return tf.DUTRPC(DefaultDUT)
}

// DUTRPC returns the gRPC connection of the given DUT.
func (tf *TestFixture) DUTRPC(dutIdx DutIdx) *rpc.Client {
	return tf.duts[dutIdx].rpc
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

// DefaultOpenNetworkAPwithDNSHTTP configures the router to provide an 802.11n open wifi and
// enables DNS server and HTTP server on router.
func (tf *TestFixture) DefaultOpenNetworkAPwithDNSHTTP(ctx context.Context) (*APIface, error) {
	return tf.ConfigureAPOnRouterID(ctx, 0, DefaultOpenNetworkAPOptions(), nil, true, true)
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
	wpar := remotewpacli.NewRemoteRunner(tf.duts[dutIdx].dut.Conn())

	err := wpar.ClearBSSIDIgnore(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to clear WPA BSSID_IGNORE")
	}

	return nil
}

// SendChannelSwitchAnnouncement sends a CSA frame and waits for Client_Disconnection, or Channel_Switch event.
func (tf *TestFixture) SendChannelSwitchAnnouncement(ctx context.Context, dutIdx DutIdx, ap *APIface, maxRetry, alternateChannel int) error {
	ctxForCloseFrameSender := ctx
	r, ok := tf.Router().(support.FrameSender)
	if !ok {
		return errors.Errorf("router type %q does not support FrameSender", tf.Router().RouterType().String())
	}
	ctx, cancel := ctxutil.Shorten(ctx, common.RouterCloseFrameSenderDuration)
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

	ew, err := iw.NewEventWatcher(ctx, tf.duts[dutIdx].dut)
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
	iwr := iw.NewRemoteRunner(tf.duts[dutIdx].dut.Conn())
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
func setLoggingConfig(ctx context.Context, wc *WifiClient, level int, tags []string) error {
	testing.ContextLogf(ctx, "Configuring logging level: %d, tags: %v", level, tags)
	_, err := wc.SetLoggingConfig(ctx, &wifi.SetLoggingConfigRequest{DebugLevel: int32(level), DebugTags: tags})
	return err
}

// getLoggingConfig returns the current DUT's logging setting (level and tags).
func (tf *TestFixture) getLoggingConfig(ctx context.Context, wc *WifiClient) (int, []string, error) {
	currentConfig, err := wc.GetLoggingConfig(ctx, &empty.Empty{})
	if err != nil {
		return 0, nil, err
	}
	return int(currentConfig.DebugLevel), currentConfig.DebugTags, err
}

// SetWakeOnWifi sets properties related to wake on WiFi.
// DEPRECATED: Use tf.WifiClient().SetWakeOnWifi instead.
func (tf *TestFixture) SetWakeOnWifi(ctx context.Context, ops ...SetWakeOnWifiOption) (shortenCtx context.Context, cleanupFunc func() error, retErr error) {
	return tf.WifiClient().SetWakeOnWifi(ctx, ops...)
}

// newRouter connects to and initializes the router via SSH then returns the router object.
// This method takes two context: ctx and daemonCtx, the first is the context for the NewRouter
// method and daemonCtx is for the spawned background daemons.
// After getting a Server instance, d, the caller should call r.Close() at the end, and use the
// shortened ctx (provided by d.ReserveForClose()) before r.Close() to reserve time for it to run.
func newRouter(ctx, daemonCtx context.Context, host *ssh.Conn, name string, rtype support.RouterType) (router.Base, error) {
	ctx, st := timing.Start(ctx, "NewRouter")
	defer st.End()

	if rtype == support.UnknownT {
		if resolvedType, err := resolveRouterTypeFromHost(ctx, host); err != nil {
			return nil, errors.Wrap(err, "failed to resolve router type from host")
		} else if resolvedType == support.UnknownT {
			rtype = support.LegacyT
			testing.ContextLogf(ctx, "Unable to resolve specific router type from host, defaulting to %q", rtype.String())
		} else {
			rtype = resolvedType
			testing.ContextLogf(ctx, "Resolved host router type to be %q", rtype.String())
		}
	}

	switch rtype {
	case support.LegacyT:
		return legacy.NewRouter(ctx, daemonCtx, host, name)
	case support.AxT:
		return ax.NewRouter(ctx, daemonCtx, host, name)
	case support.OpenWrtT:
		return openwrt.NewRouter(ctx, daemonCtx, host, name)
	default:
		return nil, errors.Errorf("unexpected routerType, got %v", rtype)
	}
}

func resolveRouterTypeFromHost(ctx context.Context, host *ssh.Conn) (support.RouterType, error) {
	if isLegacy, err := legacy.HostIsLegacyRouter(ctx, host); err != nil {
		return -1, err
	} else if isLegacy {
		return support.LegacyT, nil
	}
	if isOpenWrt, err := openwrt.HostIsOpenWrtRouter(ctx, host); err != nil {
		return -1, err
	} else if isOpenWrt {
		return support.OpenWrtT, nil
	}
	if isAx, err := ax.HostIsAXRouter(ctx, host); err != nil {
		return -1, err
	} else if isAx {
		return support.AxT, nil
	}
	return support.UnknownT, nil
}

// WaitWifiConnected waits until WiFi is connected to the SHILL profile with specific GUID.
func (tf *TestFixture) WaitWifiConnected(ctx context.Context, dutIdx DutIdx, guid string) error {
	testing.ContextLogf(ctx, "Waiting for WiFi to be connected from DUT #%v to profile with GUID: %s", dutIdx, guid)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		req := &wifi.RequestScansRequest{Count: 1}
		if _, err := tf.DUTWifiClient(dutIdx).RequestScans(ctx, req); err != nil {
			errors.Wrap(err, "failed to request scan")
		}

		serInfo, err := tf.DUTWifiClient(dutIdx).QueryService(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the WiFi service information from DUT #%v", dutIdx)
		}

		if guid == serInfo.Guid && serInfo.IsConnected {
			iface, err := tf.DUTClientInterface(ctx, dutIdx)
			if err != nil {
				return errors.Wrapf(err, "failed to get interface from the DUT #%v", dutIdx)
			}

			addrs, err := tf.DUTWifiClient(dutIdx).GetIPv4Addrs(ctx, &wifi.GetIPv4AddrsRequest{InterfaceName: iface})
			if err != nil {
				return errors.Wrapf(err, "failed to get client #%v IPv4 addresses", dutIdx)
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

// P2PDeviceConn returns the P2P device ssh connection.
func (tf *TestFixture) P2PDeviceConn(ctx context.Context, device P2PDevice) (*dut.DUT, error) {
	switch device {
	case P2PDeviceDUT:
		return tf.duts[DefaultDUT].dut, nil
	case P2PDeviceCompanionDUT:
		return tf.duts[PeerDUT].dut, nil
	}
	return nil, errors.Errorf("unexpected P2P device type: %d", device)
}

// P2PConfigureGO configures the DUT as a p2p group owner (GO).
func (tf *TestFixture) P2PConfigureGO(ctx context.Context, device P2PDevice) error {
	// This function removes any existing P2P interfaces before adding the
	// group owner. After that, the function waits for the p2p group owner
	// interface to be available. The GO interface name, network SSID and
	// passpharse are saved.

	var err error
	if tf.p2pGO, err = tf.P2PDeviceConn(ctx, device); err != nil {
		return err
	}

	wpar := remotewpacli.NewRemoteRunner(tf.p2pGO.Conn())
	ipr := remoteip.NewRemoteRunner(tf.p2pGO.Conn())

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	const wpaMonitorStopTimeout = 5 * time.Second
	wpaMonitor := new(wpacli.WPAMonitor)
	stop, ctx, err := wpaMonitor.StartWPAMonitor(timeoutCtx, tf.p2pGO.Conn(), wpaMonitorStopTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to start wpa monitor")
	}
	defer stop()

	// Add a p2p group owner (GO).
	if err := wpar.P2PGroupAdd(ctx); err != nil {
		return err
	}

	const waitForP2PGroupStartedTimeout = 10 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		event, err := wpaMonitor.WaitForEvent(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to wait for P2PGroupStartedEvent"))
		}
		if event == nil { // timeout
			return testing.PollBreak(errors.New("timed out waiting for P2PGroupStartedEvent"))
		}
		if p2pGroupStartedEvent, p2pGroupStartedSuccess := event.(*wpacli.P2PGroupStartedEvent); p2pGroupStartedSuccess {
			tf.p2pGOIface = p2pGroupStartedEvent.IfaceName
			tf.p2pGroupSSID = p2pGroupStartedEvent.SSID
			tf.p2pGroupPassphrase = p2pGroupStartedEvent.Passphrase
			return nil
		}

		return errors.New("no P2PGroupStartedEvent found")
	}, &testing.PollOptions{Timeout: waitForP2PGroupStartedTimeout}); err != nil {
		return err
	}

	if err := ipr.SetLinkUp(ctx, tf.p2pGOIface); err != nil {
		return err
	}
	if err := ipr.AddIP(ctx, tf.p2pGOIface, net.ParseIP(p2pGOIPAddress), 24); err != nil {
		return err
	}
	testing.ContextLog(ctx, "P2P Group owner (GO): Configured")

	return nil
}

// P2PConfigureClient configures the companion DUT as a p2p client.
func (tf *TestFixture) P2PConfigureClient(ctx context.Context, device P2PDevice) error {
	// This function scans for the p2p group owner network using tf.p2pGroupSSID
	// and adds the network in the client device (companion DUT).

	var err error
	if tf.p2pClient, err = tf.P2PDeviceConn(ctx, device); err != nil {
		return err
	}

	wpar := remotewpacli.NewRemoteRunner(tf.p2pClient.Conn())

	if err := wpar.DiscoverNetwork(ctx, tf.duts[PeerDUT].dut.Conn(), tf.p2pGroupSSID); err != nil {
		return err
	}
	if err := wpar.P2PAddGONetwork(ctx, tf.p2pGroupSSID, tf.p2pGroupPassphrase); err != nil {
		return err
	}
	testing.ContextLog(ctx, "P2P Client: Configured")

	return nil
}

// P2PConnect connects the p2p client to the p2p group owner (GO) network and waits for the service to be connected.
func (tf *TestFixture) P2PConnect(ctx context.Context) error {
	wpar := remotewpacli.NewRemoteRunner(tf.p2pClient.Conn())
	ipr := remoteip.NewRemoteRunner(tf.p2pClient.Conn())

	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	const wpaMonitorStopTimeout = 5 * time.Second
	wpaMonitor := new(wpacli.WPAMonitor)
	stop, ctx, err := wpaMonitor.StartWPAMonitor(timeoutCtx, tf.p2pClient.Conn(), wpaMonitorStopTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to start wpa monitor")
	}
	defer stop()

	if err := wpar.P2PGroupAddPersistent(ctx); err != nil {
		return err
	}

	const waitForP2PGroupStartedTimeout = 10 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		event, err := wpaMonitor.WaitForEvent(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to wait for P2PGroupStartedEvent"))
		}
		if event == nil { // timeout
			return testing.PollBreak(errors.New("timed out waiting for P2PGroupStartedEvent"))
		}
		if p2pGroupStartedEvent, p2pGroupStartedSuccess := event.(*wpacli.P2PGroupStartedEvent); p2pGroupStartedSuccess {
			tf.p2pClientIface = p2pGroupStartedEvent.IfaceName
			return nil
		}

		return errors.New("no P2PGroupStartedEvent found")
	}, &testing.PollOptions{Timeout: waitForP2PGroupStartedTimeout}); err != nil {
		return err
	}

	if err := ipr.SetLinkUp(ctx, tf.p2pClientIface); err != nil {
		return err
	}
	if err := ipr.AddIP(ctx, tf.p2pClientIface, net.ParseIP(p2pClientIPAddress), 24); err != nil {
		return err
	}

	testing.ContextLog(ctx, "The p2p client is connected to the p2p group owner (GO) network")

	return nil
}

// P2PAddIPRoute routes the ip addresses for the p2p group owner (GO) and p2p client.
func (tf *TestFixture) P2PAddIPRoute(ctx context.Context) error {
	iprDUT := remoteip.NewRemoteRunner(tf.p2pGO.Conn())
	iprPeer := remoteip.NewRemoteRunner(tf.p2pClient.Conn())

	if err := iprDUT.RouteIP(ctx, tf.p2pGOIface, net.ParseIP(p2pClientIPAddress)); err != nil {
		return err
	}
	if err := iprPeer.RouteIP(ctx, tf.p2pClientIface, net.ParseIP(p2pGOIPAddress)); err != nil {
		return err
	}

	return nil
}

// P2PDeleteIPRoute deletes the ip routing for the p2p group owner (GO) and p2p client.
func (tf *TestFixture) P2PDeleteIPRoute(ctx context.Context) error {
	iprDUT := remoteip.NewRemoteRunner(tf.p2pGO.Conn())
	iprPeer := remoteip.NewRemoteRunner(tf.p2pClient.Conn())

	if err := iprDUT.DeleteIPRoute(ctx, tf.p2pGOIface, net.ParseIP(p2pGOIPAddress)); err != nil {
		return err
	}
	if err := iprPeer.DeleteIPRoute(ctx, tf.p2pClientIface, net.ParseIP(p2pClientIPAddress)); err != nil {
		return err
	}
	if err := iprDUT.DeleteIP(ctx, tf.p2pGOIface, net.ParseIP(p2pGOIPAddress), 24); err != nil {
		return err
	}
	if err := iprPeer.DeleteIP(ctx, tf.p2pClientIface, net.ParseIP(p2pClientIPAddress), 24); err != nil {
		return err
	}
	if err := iprDUT.SetLinkDown(ctx, tf.p2pGOIface); err != nil {
		return err
	}
	if err := iprPeer.SetLinkDown(ctx, tf.p2pClientIface); err != nil {
		return err
	}

	return nil
}

// P2PAssertPingFromGO pings the p2p client from the group owner (GO) device.
func (tf *TestFixture) P2PAssertPingFromGO(ctx context.Context, opts ...ping.Option) error {
	pr := remoteping.NewRemoteRunner(tf.p2pGO.Conn())

	opts = append(opts, ping.BindAddress(true), ping.SourceIface(tf.p2pGOIface))
	testing.ContextLog(ctx, "Ping p2p client from p2p group owner (GO)")
	res, err := pr.Ping(ctx, p2pClientIPAddress, opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	if res.Loss > pingLossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	return nil
}

// P2PAssertPingFromClient pings the p2p group owner (GO) from the p2p client device.
func (tf *TestFixture) P2PAssertPingFromClient(ctx context.Context, opts ...ping.Option) error {
	pr := remoteping.NewRemoteRunner(tf.p2pClient.Conn())

	opts = append(opts, ping.BindAddress(true), ping.SourceIface(tf.p2pClientIface))
	testing.ContextLog(ctx, "Ping p2p group owner (GO) from p2p client")
	res, err := pr.Ping(ctx, p2pGOIPAddress, opts...)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "ping statistics=%+v", res)
	if res.Loss > pingLossThreshold {
		return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	return nil
}

// P2PDeconfigureGO deconfigures the p2p group owner (GO).
func (tf *TestFixture) P2PDeconfigureGO(ctx context.Context) error {
	wpa := remotewpacli.NewRemoteRunner(tf.p2pGO.Conn())

	if err := wpa.RemoveAllNetworks(ctx); err != nil {
		return err
	}
	if err := wpa.P2PFlush(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "P2P Group owner (GO): Deconfigured")

	return nil
}

// P2PDeconfigureClient deconfigures the p2p client.
func (tf *TestFixture) P2PDeconfigureClient(ctx context.Context) error {
	wpa := remotewpacli.NewRemoteRunner(tf.p2pClient.Conn())

	if err := wpa.P2PGroupRemove(ctx, tf.p2pClientIface); err != nil {
		return err
	}
	if err := wpa.RemoveAllNetworks(ctx); err != nil {
		return err
	}
	if err := wpa.P2PFlush(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "P2P Client: Deconfigured")

	return nil
}

// ReserveForDeconfigP2P returns a shorter ctx and cancel function for tf.P2PDeconfigureGO() or tf.P2PDeconfigureClient().
func (tf *TestFixture) ReserveForDeconfigP2P(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}

// ReserveForDeleteIPRoute returns a shorter ctx and cancel function for tf.P2PDeleteIPRoute().
func (tf *TestFixture) ReserveForDeleteIPRoute(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 2*time.Second)
}

// StartTethering configures the specific DUT to provide a tethering session with the options specified.
func (tf *TestFixture) StartTethering(ctx context.Context, dutIdx DutIdx, ops []tethering.Option) (*tethering.Config, *wifi.TetheringResponse, error) {
	ctx, st := timing.Start(ctx, "tf.StartTethering")
	defer st.End()

	c, err := tethering.NewConfig(ops...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create tethering config")
	}

	request := &wifi.TetheringRequest{
		NoUplink:          c.NoUL,
		AutoDisableMinute: c.AutoDisableMin,
		Ssid:              []byte(c.SSID),
		Band:              c.Band.String(),
	}

	if c.SecConf.Class() == shillconst.SecurityClassPSK {
		request.Psk = c.PSK
		if c.SecMode == wpa.ModePureWPA2 {
			request.Security = shillconst.SecurityWPA2
		} else if c.SecMode == wpa.ModePureWPA3 {
			request.Security = shillconst.SecurityWPA3
		} else if c.SecMode == wpa.ModeMixedWPA3 {
			request.Security = shillconst.SecurityWPA2WPA3
		}
	} else if c.SecConf.Class() == shillconst.SecurityNone {
		request.Security = shillconst.SecurityNone
	}

	resp, err := tf.duts[dutIdx].wifiClient.StartTethering(ctx, request)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "client failed to start tethering session with SSID %q", c.SSID)
	}

	return c, resp, nil
}

// StopTethering attempts to stop the tethering session for the specified DUT.
func (tf *TestFixture) StopTethering(ctx context.Context, dutIdx DutIdx) error {
	ctx, st := timing.Start(ctx, "tf.StopTethering")
	defer st.End()

	_, err := tf.duts[dutIdx].wifiClient.StopTethering(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "client failed to stop tethering session")
	}

	return nil
}

// ReserveForStopTethering returns a shorter ctx and cancel function for tf.StopTethering().
func (tf *TestFixture) ReserveForStopTethering(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 10*time.Second)
}
