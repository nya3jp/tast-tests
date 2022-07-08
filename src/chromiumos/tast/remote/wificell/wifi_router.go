// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/errors"
	remotearping "chromiumos/tast/remote/network/arping"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/network/iw"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/router/ax"
	"chromiumos/tast/remote/wificell/router/common/support"
	"chromiumos/tast/remote/wificell/router/legacy"
	"chromiumos/tast/remote/wificell/router/openwrt"
	"chromiumos/tast/remote/wificell/wifiutil"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// WiFiRouter defines interface for wificell's Router device.
type WiFiRouter interface {
	WiFiHost
	// Connect establishes connection to DUT.
	Connect(ctx, daemonCtx context.Context, dut WiFiDUT, hostUsers map[string]string) error
	// CollectLogs downloads related log files.
	CollectLogs(ctx context.Context) error
	// ConfigureAP configures the router to provide a WiFi service with the options specified.
	ConfigureAP(ctx context.Context, ops []hostapd.Option, fac security.ConfigFactory, cap WiFiRouter) (ret *APIface, c *pcap.Capturer, retErr error)
	// RouterType returns the router's type.
	RouterType() support.RouterType
	// SendChannelSwitchAnnouncement sends Channel Switch Announcement.
	SendChannelSwitchAnnouncement(ctx context.Context, dut WiFiDUT, ap *APIface, maxRetry, alternateChannel int) error
	// SupportsCapture checks if underlying object supports Capture interface.
	SupportsCapture() bool
	// DeconfigAP stops the WiFi service on router.
	DeconfigAP(ctx context.Context, ap *APIface) error
	obj() router.Base
}

// APData defines AP-related data
type APData struct {
	ap       *APIface
	pcap     WiFiRouter
	capturer *pcap.Capturer
}

// WiFiRouterImpl implements WiFiRouter interface.
type WiFiRouterImpl struct {
	hostName   string
	conn       *ssh.Conn
	object     router.Base
	routerType support.RouterType
	APs        []APData
}

// ArPingIP sends ARP ping from host to target IP.
func (r *WiFiRouterImpl) ArPingIP(ctx context.Context, targetIP string, opts ...arping.Option) (*arping.Result, error) {
	ctx, st := timing.Start(ctx, "WiFiRouter.PingIP")
	defer st.End()

	pr := remotearping.NewRemoteRunner(r.Conn())
	return pr.Arping(ctx, targetIP, r.APs[0].ap.Interface(), opts...)
}

// Close closes connection to Router.
func (r *WiFiRouterImpl) Close(ctx context.Context) error {
	var firstErr error
	if r.object != nil {
		if err := r.object.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close rotuer %s", r.Name()))
		}
	}
	r.object = nil
	if r.conn != nil {
		if err := r.conn.Close(ctx); err != nil {
			wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrapf(err, "failed to close router %s ssh", r.Name()))
		}
	}
	r.conn = nil
	return firstErr
}

// CollectLogs downloads related log files.
func (r *WiFiRouterImpl) CollectLogs(ctx context.Context) error {
	// Assert router can collect logs
	rt, ok := r.object.(support.Logs)
	if !ok {
		return errors.Errorf("router type %q does not support Logs", r.object.RouterType().String())
	}
	return rt.CollectLogs(ctx)
}

// ConfigureAP configures the router to provide a WiFi service with the options specified.
// Note that after getting an APIface, ap, the caller should defer WiFiRouter.DeconfigAP(ctx, ap) and
// use WiFiRouter.ReserveForClose(ctx, ap) to reserve time for the deferred call.
func (r *WiFiRouterImpl) ConfigureAP(ctx context.Context, ops []hostapd.Option, fac security.ConfigFactory, cap WiFiRouter) (ret *APIface, c *pcap.Capturer, retErr error) {
	ctx, st := timing.Start(ctx, "tf.ConfigureAP")
	defer st.End()
	name := UniqueAPName()

	if fac != nil {
		// Defer the securityConfig generation from test's init() to here because the step may emit error and that's not allowed in test init().
		securityConfig, err := fac.Gen()
		if err != nil {
			return nil, nil, err
		}
		ops = append([]hostapd.Option{hostapd.SecurityConfig(securityConfig)}, ops...)
	}
	config, err := hostapd.NewConfig(ops...)
	if err != nil {
		return nil, nil, err
	}

	var capturer *pcap.Capturer
	if cap != nil {
		freqOps, err := config.PcapFreqOptions()
		if err != nil {
			return nil, nil, err
		}
		if !cap.SupportsCapture() {
			return nil, nil, errors.Errorf("pcap device with router type %q does not have log capture support", "foo") //cap.obj().RouterType().String())
		}
		capturer, err = cap.obj().(support.Capture).StartCapture(ctx, name, config.Channel, freqOps)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to start capturer")
		}
		defer func() {
			if retErr != nil {
				cap.obj().(support.Capture).StopCapture(ctx, capturer)
			}
		}()
	}

	ap, err := StartAPIface(ctx, r, name, config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start APIface")
	}
	r.APs = append(r.APs, APData{ap: ap, pcap: cap, capturer: capturer})

	return ap, capturer, nil
}

// Conn returns SSH connection object to a given router.
func (r *WiFiRouterImpl) Conn() *ssh.Conn {
	return r.conn
}

// Connect establishes connection to DUT.
func (r *WiFiRouterImpl) Connect(ctx, daemonCtx context.Context, dut WiFiDUT, hostUsers map[string]string) error {
	testing.ContextLogf(ctx, "Adding router %s", r.hostName)
	routerHost, err := dut.connectCompanion(ctx, r.hostName, hostUsers, true /* allow retry */)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to the router %s", r.hostName)
	}
	r.conn = routerHost
	routerObj, err := newRouter(ctx, daemonCtx, r.conn,
		strings.ReplaceAll(r.hostName, ":", "_"), support.UnknownT)
	if err != nil {
		return errors.Wrap(err, "failed to create a router object")
	}
	testing.ContextLogf(ctx, "Successfully instantiated %s router controller for router[%s]", routerObj.RouterType().String(), r.hostName)
	r.object = routerObj

	return nil
}

// DeconfigAP stops the WiFi service on router.
func (r *WiFiRouterImpl) DeconfigAP(ctx context.Context, ap *APIface) error {
	ctx, st := timing.Start(ctx, "tf.DeconfigAP")
	defer st.End()

	for i, apData := range r.APs {
		if apData.ap == ap {
			var firstErr error

			if err := ap.Stop(ctx); err != nil {
				wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop APIface"))
			}
			if apData.capturer != nil {
				if err := apData.pcap.obj().(support.Capture).StopCapture(ctx, apData.capturer); err != nil {
					wifiutil.CollectFirstErr(ctx, &firstErr, errors.Wrap(err, "failed to stop capturer"))
				}
			}
			r.APs = append(r.APs[:i], r.APs[i+1:]...)
			return firstErr
		}
	}
	return errors.New("AP not found")
}

// HwAddr returns Hardware address of the wifi-related interface.
func (r *WiFiRouterImpl) HwAddr(ctx context.Context) (net.HardwareAddr, error) {
	if len(r.APs) == 0 {
		return nil, errors.New("no APs configured, cannot get MAC")
	}

	ipRunner := ip.NewRemoteRunner(r.Conn())
	return ipRunner.MAC(ctx, r.APs[0].ap.dhcpd.Interface())
}

// IPv4Addrs returns IPv4 addresses of the wifi-related interface.
func (r *WiFiRouterImpl) IPv4Addrs(ctx context.Context) ([]net.IP, error) {
	if len(r.APs) == 0 {
		return nil, errors.New("no APs configured, cannot get IP")
	}
	ifName := r.APs[0].ap.dhcpd.Interface()

	ipr := ip.NewRemoteRunner(r.Conn())
	ip, _, err := ipr.IPv4(ctx, ifName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get IP of the %s interface", ifName)
	}
	return []net.IP{ip}, nil
}

// Name returns host name.
func (r *WiFiRouterImpl) Name() string {
	return r.hostName //object.RouterName()
}

// PingIP sends ICMP ping from host to target IP.
func (r *WiFiRouterImpl) PingIP(ctx context.Context, targetIP string, opts ...ping.Option) (*ping.Result, error) {
	ctx, st := timing.Start(ctx, "WiFiRouter.PingIP")
	defer st.End()

	// addrs, err := tf.ClientIPv4Addrs(ctx)
	// if err != nil || len(addrs) == 0 {
	// 	return errors.Wrap(err, "failed to get the IP address")
	// }

	pr := remoteping.NewRemoteRunner(r.Conn())
	return pr.Ping(ctx, targetIP, opts...)
}

// RouterType returns the router's type.
func (r *WiFiRouterImpl) RouterType() support.RouterType {
	return r.object.RouterType()
}

// SendChannelSwitchAnnouncement sends Channel Switch Announcement.
func (r *WiFiRouterImpl) SendChannelSwitchAnnouncement(ctx context.Context, dut WiFiDUT, ap *APIface, maxRetry, alternateChannel int) error {
	ctxForCloseFrameSender := ctx
	fs, ok := r.obj().(support.FrameSender)
	if !ok {
		return errors.Errorf("router type %q does not support FrameSender", r.obj().RouterType().String())
	}
	ctx, cancel := fs.ReserveForCloseFrameSender(ctx)
	defer cancel()
	sender, err := fs.NewFrameSender(ctx, ap.Interface())
	if err != nil {
		return errors.Wrap(err, "failed to create frame sender")
	}
	defer func(ctx context.Context) error {
		if err := fs.CloseFrameSender(ctx, sender); err != nil {
			return errors.Wrap(err, "failed to close frame sender")
		}
		return nil
	}(ctxForCloseFrameSender)

	ew, err := iw.NewEventWatcher(ctx, dut.DUT())
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

// SupportsCapture checks if underlying object supports Capture interface.
func (r *WiFiRouterImpl) SupportsCapture() bool {
	_, ok := r.object.(support.Capture)
	return ok
}

func (r *WiFiRouterImpl) obj() router.Base {
	return r.object
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
