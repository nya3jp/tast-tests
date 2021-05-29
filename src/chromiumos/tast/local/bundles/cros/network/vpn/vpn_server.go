// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// Constants that used by the L2TP/IPsec server.
const (
	chapUser              = "chapuser"
	chapSecret            = "chapsecret"
	makeIPSecDir          = "mkdir -p /run/ipsec"
	ipsecCommand          = "/usr/sbin/ipsec"
	ipsecLogFile          = "var/log/charon.log"
	ipsecPresharedKey     = "preshared-key"
	pppdPidFile           = "run/ppp0.pid"
	xauthUser             = "xauth_user"
	xauthPassword         = "xauth_password"
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
			"proxyarp\n" +
			"ms-dns 8.8.8.8\n" +
			"ms-dns 8.8.4.4\n",
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

// Constants that used by OpenVPN server
const (
	openvpnCommand           = "/usr/sbin/openvpn"
	openvpnConfigFile        = "etc/openvpn/openvpn.conf"
	openvpnCaCertFile        = "etc/openvpn/ca.crt"
	openvpnServerCertFile    = "etc/openvpn/server.crt"
	openvpnServerKeyFile     = "etc/openvpn/server.key"
	openvpnDiffieHellmanFile = "etc/openvpn/diffie-hellman.pem"
	openvpnExpectedAuthFile  = "etc/openvpn_expected_authentication.txt"
	openvpnAuthScript        = "etc/openvpn_authentication_script.sh"
	openvpnLogFile           = "var/log/openvpn.log"
	openvpnPidFile           = "run/openvpn.pid"
	openvpnStatusFile        = "tmp/openvpn.status"
	openvpnUsername          = "username"
	openvpnPassword          = "password"
	openvpnServerIPAddress   = "10.11.12.1"
)

// dh1024PemKey is the Diffie–Hellman parameter which will be used by OpenVPN
// server in the test. The value is borrowed from Autotest
// (client/common_lib/cros/site_eap_certs.py)
const dh1024PemKey = `-----BEGIN DH PARAMETERS-----
MIGHAoGBAL/YrUzFuA5cPGzhXVqTvDugmPi9CpbWZx2+TCTKxZSjNiVJxcICSnql
uZtkR3sOAiWn384E4ZQTBrYPUguOuFfbMTRooADhezaG9SXtrE9oeVy9avIO7xQK
emZydO0bAsRV+eL0XkjGhSyhKoOvSIXaCbJUn7duEsfkICPRLWCrAgEC
-----END DH PARAMETERS-----
`

var (
	openvpnRootDirectories = []string{"etc/openvpn"}
	openvpnConfigs         = map[string]string{
		openvpnCaCertFile:        certificate.TestCert1().CACred.Cert,
		openvpnServerCertFile:    certificate.TestCert1().ServerCred.Cert,
		openvpnServerKeyFile:     certificate.TestCert1().ServerCred.PrivateKey,
		openvpnDiffieHellmanFile: dh1024PemKey,
		openvpnAuthScript:        "#!/bin/bash\ndiff -q $1 {{.expected_authentication_file}}\n",
		openvpnExpectedAuthFile:  "{{.username}}\n{{.password}}\n",
		openvpnConfigFile: "ca /{{.ca_cert}}\n" +
			"cert /{{.server_cert}}\n" +
			"dev tun\n" +
			"dh /{{.diffie_hellman_params_file}}\n" +
			"keepalive 10 120\n" +
			"local {{.netns_ip}}\n" +
			"log /{{.log_file}}\n" +
			"ifconfig-pool-persist /tmp/ipp.txt\n" +
			"key /{{.server_key}}\n" +
			"persist-key\n" +
			"persist-tun\n" +
			"port 1194\n" +
			"proto udp\n" +
			"server 10.11.12.0 255.255.255.0\n" +
			"status /{{.status_file}}\n" +
			"verb 5\n" +
			"writepid /{{.pid_file}}\n" +
			"tmp-dir /tmp\n" +
			"{{.optional_user_verification}}\n",
	}
)

// Server represents a VPN server that can be used in the test.
type Server struct {
	OverlayIP    string
	underlayIP   string
	netChroot    *chroot.NetworkChroot
	stopCommands [][]string
	pidFiles     []string
	logFile      string
}

// StartL2TPIPsecServer starts a L2TP/IPsec server.
func StartL2TPIPsecServer(ctx context.Context, authType string, ipsecUseXauth, underlayIPIsOverlayIP bool) (*Server, error) {
	chro := chroot.NewNetworkChroot()
	server := &Server{
		netChroot:    chro,
		stopCommands: [][]string{{ipsecCommand, "stop"}},
		pidFiles:     []string{xl2tpdPidFile, pppdPidFile},
		logFile:      ipsecLogFile,
	}

	if _, ok := ipsecTypedConfigs[authType]; !ok {
		return nil, errors.Errorf("L2TP/IPSec type %s is not defined", authType)
	}

	chro.AddRootDirectories(xl2tpdRootDirectories)
	chro.AddConfigTemplates(ipsecCommonConfigs)
	chro.AddConfigTemplates(ipsecTypedConfigs[authType])

	configValues := map[string]interface{}{
		"chap_user":                chapUser,
		"chap_secret":              chapSecret,
		"charon_debug_flags":       "dmn 2, mgr 2, ike 2, net 2",
		"charon_logfile":           ipsecLogFile,
		"preshared_key":            ipsecPresharedKey,
		"xauth_user":               xauthUser,
		"xauth_password":           xauthPassword,
		"xl2tpd_server_ip_address": xl2tpdServerIPAddress,
		"use_underlay_ip":          underlayIPIsOverlayIP,
	}

	if ipsecUseXauth {
		configValues["xauth_stanza"] = "rightauth2=xauth"
	} else {
		configValues["xauth_stanza"] = ""
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
		return nil, errors.Wrap(err, "failed to start L2TP/IPsec server")
	}
	server.underlayIP = underlayIP
	if underlayIPIsOverlayIP {
		server.OverlayIP = underlayIP
	} else {
		server.OverlayIP = xl2tpdServerIPAddress
	}
	// Setup internet connectivity for VPN server.
	if err := chro.Command(ctx, "iptables", "-t", "nat", "-A", "POSTROUTING", "!", "-s", xl2tpdServerIPAddress, "-j", "SNAT", "--to", server.underlayIP, "-w").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to setup internet connectivity")
	}
	return server, nil
}

// StartOpenVPNServer starts an OpenVPN server.
func StartOpenVPNServer(ctx context.Context, useUserPassword bool) (*Server, error) {
	chro := chroot.NewNetworkChroot()
	server := &Server{
		netChroot:    chro,
		stopCommands: [][]string{},
		pidFiles:     []string{openvpnPidFile},
		logFile:      openvpnLogFile,
	}

	chro.AddRootDirectories(openvpnRootDirectories)
	chro.AddConfigTemplates(openvpnConfigs)
	configValues := map[string]interface{}{
		"ca_cert":                      openvpnCaCertFile,
		"diffie_hellman_params_file":   openvpnDiffieHellmanFile,
		"expected_authentication_file": openvpnExpectedAuthFile,
		"optional_user_verification":   "",
		"password":                     openvpnPassword,
		"pid_file":                     openvpnPidFile,
		"server_cert":                  openvpnServerCertFile,
		"server_key":                   openvpnServerKeyFile,
		"status_file":                  openvpnStatusFile,
		"username":                     openvpnUsername,
		"log_file":                     openvpnLogFile,
	}
	if useUserPassword {
		configValues["optional_user_verification"] = fmt.Sprintf("auth-user-pass-verify /%s via-file\nscript-security 2", openvpnAuthScript)
	}

	chro.AddConfigValues(configValues)
	chro.AddStartupCommand("chmod 755 " + openvpnAuthScript)
	chro.AddStartupCommand(fmt.Sprintf("%s --config /%s &", openvpnCommand, openvpnConfigFile))
	chro.NetEnv = []string{
		"OPENSSL_CONF=/etc/ssl/openssl.cnf.compat",
		"OPENSSL_CHROMIUM_SKIP_TRUSTED_PURPOSE_CHECK=1",
	}

	underlayIP, err := chro.Startup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start OpenVPN server")
	}
	server.underlayIP = underlayIP
	server.OverlayIP = openvpnServerIPAddress
	return server, nil
}

// StopServer stop VPN server instance.
func (s *Server) StopServer(ctx context.Context) error {
	chro := s.netChroot
	for _, cmd := range s.stopCommands {
		if err := chro.RunChroot(ctx, cmd); err != nil {
			return errors.Wrapf(err, "failed to execute %v", cmd)
		}
	}

	for _, pidFile := range s.pidFiles {
		if err := chro.KillPidFile(ctx, pidFile, true); err != nil {
			return errors.Wrapf(err, "failed to kill the PID file %v", pidFile)
		}
	}

	return nil
}

func (s *Server) collectLogs(ctx context.Context) error {
	content, err := s.netChroot.GetLogContents(ctx, s.logFile)
	if err != nil {
		return errors.Wrap(err, "failed to get vpn log contents")
	}

	// Write the vpn logs to the file logName.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get OutDir")
	}

	if err := ioutil.WriteFile(filepath.Join(dir, logName),
		[]byte(content), 0644); err != nil {
		return errors.Wrap(err, "failed to write vpnlogs output")
	}

	return nil
}

// Exit does a best effort to stop the server, log the contents, and shut down the chroot.
func (s *Server) Exit(ctx context.Context) error {
	var lastErr error

	// We should stop the server before call GetLogContents, since the charon
	// process may not flush all the contents before exiting.
	if err := s.StopServer(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop vpn server: ", err)
		lastErr = err
	}

	if err := s.collectLogs(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to collect logs: ", err)
		lastErr = err
	}

	if err := s.netChroot.Shutdown(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to shutdown the chroot: ", err)
		lastErr = err
	}

	return lastErr
}
