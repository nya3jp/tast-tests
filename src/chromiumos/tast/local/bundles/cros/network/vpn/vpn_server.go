// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vpn context enclosing the use of a VPN server instance..
package vpn

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/chroot"
	"chromiumos/tast/testing"
)

// Constants that used by the l2tpipsec_psk server.
const (
	ChapUser              = "chapuser"
	ChapSecret            = "chapsecret"
	makeIPSecDir          = "mkdir -p /run/ipsec"
	ipsecCommand          = "/usr/sbin/ipsec"
	ipsecLogFile          = "var/log/charon.log"
	IPsecPresharedKey     = "preshared-key"
	pppdPidFile           = "run/ppp0.pid"
	XauthUser             = "xauth_user"
	XauthPassword         = "xauth_password"
	xl2tpdCommand         = "/usr/sbin/xl2tpd"
	xl2tpdConfigFile      = "etc/xl2tpd/xl2tpd.conf"
	xl2tpdPidFile         = "run/xl2tpd.pid"
	xl2tpdServerIPAddress = "192.168.1.99"
	logName               = "vpnlogs.txt"
)

var (
	xl2tpdRootDirectories = []string{"etc/ipsec.d", "etc/ipsec.d/cacerts",
		"etc/ipsec.d/certs", "etc/ipsec.d/crls",
		"etc/ipsec.d/private", "etc/ppp", "etc/xl2tpd"}

	ipsecCommonConfigs = map[string]string{
		"etc/strongswan.conf": "charon {\n" +
			"  filelog {\n" +
			"    test_vpn {\n" +
			"      path = {{.charon_logfile}}\n" +
			"      default = 3\n" +
			"      time_format = %b %e %T\n" +
			"    }\n" +
			"  }\n" +
			"  install_routes = no\n" +
			"  ignore_routing_tables = 0\n" +
			"  routing_table = 0\n" +
			"}\n",

		"etc/passwd": "root:x:0:0:root:/root:/bin/bash\n" +
			"ipsec:*:212:212::/dev/null:/bin/false\n",

		"etc/group": "ipsec:x:212:\n",

		xl2tpdConfigFile: "[global]\n" +
			"\n" +
			"[lns default]\n" +
			"  ip range = 192.168.1.128-192.168.1.254\n" +
			"  local ip = {{if .use_underlay_ip}}{{.netns_ip}}{{else}}{{.xl2tpd_server_ip_address}}{{end}}\n" +
			"  require chap = yes\n" +
			"  refuse pap = yes\n" +
			"  require authentication = yes\n" +
			"  name = LinuxVPNserver\n" +
			"  ppp debug = yes\n" +
			"  pppoptfile = /etc/ppp/options.xl2tpd\n" +
			"  length bit = yes\n",

		"etc/xl2tpd/l2tp-secrets": "*      them    l2tp-secret",

		"etc/ppp/chap-secrets": "{{.chap_user}}        *       {{.chap_secret}}      *",

		"etc/ppp/options.xl2tpd": "ipcp-accept-local\n" +
			"ipcp-accept-remote\n" +
			"noccp\n" +
			"auth\n" +
			"crtscts\n" +
			"idle 1800\n" +
			"mtu 1410\n" +
			"mru 1410\n" +
			"nodefaultroute\n" +
			"debug\n" +
			"lock\n" +
			"proxyarp\n",
	}
	ipsecTypedConfigs = map[string]map[string]string{
		"psk": {
			"etc/ipsec.conf": "config setup\n" +
				"  charondebug=\"{{.charon_debug_flags}}\"\n" +
				"conn L2TP\n" +
				"  keyexchange=ikev1\n" +
				"  ike=aes128-sha1-modp2048!\n" +
				"  esp=3des-sha1!\n" +
				"  type=transport\n" +
				"  authby=psk\n" +
				"  {{.xauth_stanza}}\n" +
				"  rekey=no\n" +
				"  left={{.netns_ip}}\n" +
				"  leftprotoport=17/1701\n" +
				"  right=%any\n" +
				"  rightprotoport=17/%any\n" +
				"  auto=add\n",
			"etc/ipsec.secrets": "{{.netns_ip}} %any : PSK \"{{.preshared_key}}\"\n" +
				"{{.xauth_user}} : XAUTH \"{{.xauth_password}}\"\n"},
		"cert": {
			"etc/ipsec.conf": "config setup\n" +
				"  charondebug=\"{{.charon_debug_flags}}\"\n" +
				"conn L2TP\n" +
				"  keyexchange=ikev1\n" +
				"  ike=aes128-sha1-modp2048!\n" +
				"  esp=3des-sha1!\n" +
				"  type=transport\n" +
				"  left={{.netns_ip}}\n" +
				"  leftcert=server.cert\n" +
				"  leftid=\"C=US, ST=California, L=Mountain View, " +
				"CN=chromelab-wifi-testbed-server.mtv.google.com\"\n" +
				"  leftprotoport=17/1701\n" +
				"  right=%any\n" +
				"  rightca=\"C=US, ST=California, L=Mountain View, " +
				"CN=chromelab-wifi-testbed-root.mtv.google.com\"\n" +
				"  rightprotoport=17/%any\n" +
				"  auto=add\n",
			"etc/ipsec.secrets":              ": RSA server.key \"\"\n",
			"etc/ipsec.d/cacerts/ca.cert":    certificate.TestCert1().CACred.Cert,
			"etc/ipsec.d/certs/server.cert":  certificate.TestCert1().ServerCred.Cert,
			"etc/ipsec.d/private/server.key": certificate.TestCert1().ServerCred.PrivateKey},
	}
)

// L2tpipSecVpnServer represents a L2TP/IPsec VPN server.
type L2tpipSecVpnServer struct {
	authenticationType    string
	underlayIPIsOverlayIP bool
	netChroot             *chroot.NetworkChroot
	UnderlayIP            string
	OverlayIP             string
}

// NewL2tpipSecVpnServer creates a new L2tpipSecVpnServer.
func NewL2tpipSecVpnServer(ctx context.Context, authType string, underlayIPIsOverlayIP bool) *L2tpipSecVpnServer {
	networkChroot := chroot.NewNetworkChroot()
	return &L2tpipSecVpnServer{authenticationType: authType, underlayIPIsOverlayIP: underlayIPIsOverlayIP, netChroot: networkChroot}
}

// StartServer starts a VPN server.
func (s *L2tpipSecVpnServer) StartServer(ctx context.Context) error {
	if _, ok := ipsecTypedConfigs[s.authenticationType]; !ok {
		return errors.Errorf("L2TP/IPSec type %s is not define", s.authenticationType)
	}

	chro := s.netChroot
	chro.AddRootDirectories(xl2tpdRootDirectories)
	chro.AddConfigTemplates(ipsecCommonConfigs)
	chro.AddConfigTemplates(ipsecTypedConfigs[s.authenticationType])

	configValues := map[string]interface{}{
		"chap_user":                ChapUser,
		"chap_secret":              ChapSecret,
		"charon_debug_flags":       "dmn 2, mgr 2, ike 2, net 2",
		"charon_logfile":           ipsecLogFile,
		"preshared_key":            IPsecPresharedKey,
		"xauth_user":               XauthUser,
		"xauth_password":           XauthPassword,
		"xauth_stanza":             "",
		"xl2tpd_server_ip_address": xl2tpdServerIPAddress,
		"use_underlay_ip":          s.underlayIPIsOverlayIP,
	}

	// For running strongSwan VPN with flag --with-piddir=/run/ipsec. We
	// want to use /run/ipsec for strongSwan runtime data dir instead of
	// /run, and the cmdline flag applies to both client and server
	chro.AddStartupCommand(makeIPSecDir)

	chro.AddConfigValues(configValues)
	chro.AddStartupCommand(fmt.Sprintf("%s start", ipsecCommand))

	xl2tpdCmdStr := fmt.Sprintf("%s -c /%s -C /tmp/l2tpd.control", xl2tpdCommand, xl2tpdConfigFile)
	chro.AddStartupCommand(xl2tpdCmdStr)

	underlayIP, err := chro.Startup(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start L2TP/IPsec server")
	}
	s.UnderlayIP = underlayIP
	if s.underlayIPIsOverlayIP {
		s.OverlayIP = s.UnderlayIP
	} else {
		s.OverlayIP = xl2tpdServerIPAddress
	}
	return nil
}

// GetLogContents return all logs related to the chroot.
func (s *L2tpipSecVpnServer) GetLogContents(ctx context.Context) (string, error) {
	content, err := s.netChroot.GetLogContents(ctx)
	if err != nil {
		return "", err
	}

	return content, nil
}

// StopServer stop VPN server instance.
func (s *L2tpipSecVpnServer) StopServer(ctx context.Context) error {
	chro := s.netChroot
	if err := chro.RunChroot(ctx, []string{ipsecCommand, "stop"}); err != nil {
		return errors.Wrap(err, "failed to stop ipsec")
	}

	if err := chro.KillPidFile(ctx, xl2tpdPidFile, true); err != nil {
		return errors.Wrapf(err, "failed to kill the PID file %v", xl2tpdPidFile)
	}

	if err := chro.KillPidFile(ctx, pppdPidFile, true); err != nil {
		return errors.Wrapf(err, "failed to kill the PID file %v", pppdPidFile)
	}

	return nil
}

// Exit stops the server, logs the contents, and shuts down the chroot.
func (s *L2tpipSecVpnServer) Exit(ctx context.Context) error {
	// We should stop the server before call GetLogContents, since the charon
	// process may not flush all the contents before exiting.
	if err := s.StopServer(ctx); err != nil {
		return err
	}

	content, err := s.GetLogContents(ctx)
	if err != nil {
		return err
	}

	// Write the vpn logs to the file logName.
	if dir, ok := testing.ContextOutDir(ctx); ok {
		if err := ioutil.WriteFile(filepath.Join(dir, logName),
			[]byte(content), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to write vpnlogs output: ", err)
		}
	} else {
		testing.ContextLog(ctx, "Failed to open OutDir")
	}

	if err := s.netChroot.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown the chroot")
	}

	return nil
}
