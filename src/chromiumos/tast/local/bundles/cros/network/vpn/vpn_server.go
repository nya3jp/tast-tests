// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

const logName = "vpnlogs.txt"

// Constants that used by the L2TP/IPsec and IKEv2 server.
const (
	caCertFile            = "etc/swanctl/x509ca/ca.cert"
	charonCommand         = "/usr/libexec/ipsec/charon"
	charonLogFile         = "var/log/charon.log"
	charonPidFile         = "run/ipsec/charon.pid"
	chapUser              = "chapuser"
	chapSecret            = "chapsecret"
	ikeClientIdentity     = "client-id"
	ikeServerIdentity     = "C=US, ST=California, L=Mountain View, CN=chromelab-wifi-testbed-server.mtv.google.com"
	ikev2ClientIP         = "192.168.1.128"
	ikev2ServerIP         = "192.168.1.99"
	ikev2InterfaceID      = "2"
	ipsecPresharedKey     = "preshared-key"
	makeIPsecDir          = "mkdir -p /run/ipsec"
	pppdPidFile           = "run/ppp0.pid"
	swanctlCommand        = "/usr/sbin/swanctl"
	viciSocketFile        = "run/ipsec/charon.vici"
	xauthUser             = "xauth_user"
	xauthPassword         = "xauth_password"
	xl2tpdCommand         = "/usr/sbin/xl2tpd"
	xl2tpdConfigFile      = "etc/xl2tpd/xl2tpd.conf"
	xl2tpdPidFile         = "run/xl2tpd.pid"
	xl2tpdServerIPAddress = "192.168.1.99"
)

var (
	strongSwanDirectories = []string{
		"etc/swanctl",
		"etc/swanctl/x509ca",
		"etc/swanctl/x509",
		"etc/swanctl/private",
	}
	l2tpDirectories = []string{
		"etc/ppp",
		"etc/xl2tpd",
	}

	strongSwanConfigs = map[string]string{
		"etc/strongswan.conf": "charon {\n" +
			"  filelog {\n" +
			"    test_vpn {\n" +
			"      path = {{.charon_logfile}}\n" +
			"      default = 3\n" +
			"      time_format = %b %e %T\n" +
			"      dmn = 2\n" +
			"      mgr = 2\n" +
			"      ike = 2\n" +
			"      net = 2\n" +
			"    }\n" +
			"  }\n" +
			"  install_routes = no\n" +
			"  ignore_routing_tables = 0\n" +
			"  routing_table = 0\n" +
			"}\n",

		"etc/swanctl/swanctl.conf": "connections {\n" +
			"  ikev1-l2tp-test {\n" +
			"    version = 1\n" +
			"    {{if .preshared_key}}" +
			"      local-psk {\n" +
			"        auth = psk\n" +
			"      }\n" +
			"      remote-psk {\n" +
			"        auth = psk\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .xauth_user}}" +
			"      remote-xauth {\n" +
			"        auth = xauth\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .server_cert_id}}" +
			"      local-cert {\n" +
			"        auth = pubkey\n" +
			"        id = {{.server_cert_id}}\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .ca_cert_file}}" +
			"      remote-cert {\n" +
			"        auth = pubkey\n" +
			"        cacerts = /{{.ca_cert_file}}\n" +
			"      }\n" +
			"    {{end}}" +
			"    children {\n" +
			"      ikev1-l2tp {\n" +
			"        local_ts = dynamic[/1701]\n" +
			"        mode = transport\n" +
			"      }\n" +
			"    }\n" +
			"  }\n" +
			"" +
			"  ikev2-test {\n" +
			"    version = 2\n" +
			"    pools = ikev2-vip-pools\n" +
			"    {{if .preshared_key}}" +
			"      local-psk {\n" +
			"        auth = psk\n" +
			"        id={{.server_id}}\n" +
			"      }\n" +
			"      remote-psk {\n" +
			"        auth = psk\n" +
			"        id={{.client_id}}\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .eap_user}}" +
			"      remote-eap {\n" +
			"        auth = eap-mschapv2\n" +
			"        eap_id = %any\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .server_cert_id}}" +
			"      local-cert {\n" +
			"        auth = pubkey\n" +
			"        id = {{.server_cert_id}}\n" +
			"      }\n" +
			"    {{end}}" +
			"    {{if .ca_cert_file}}" +
			"      remote-cert {\n" +
			"        auth = pubkey\n" +
			"        cacerts = /{{.ca_cert_file}}\n" +
			"      }\n" +
			"    {{end}}" +
			"    children {\n" +
			"      ikev2 {\n" +
			"        local_ts = 0.0.0.0/0\n" +
			"        {{if .client_vip}}" +
			"        remote_ts = {{.client_vip}}/32\n" +
			"        {{end}}" +
			"        {{if .if_id}}" +
			"        if_id_in = {{.if_id}}\n" +
			"        if_id_out = {{.if_id}}\n" +
			"        {{end}}" +
			"        mode = tunnel\n" +
			"      }\n" +
			"    }\n" +
			"  }\n" +
			"}\n" +
			"" +
			"secrets {\n" +
			"  {{if .preshared_key}}" +
			"  ike-1 {\n" +
			"    secret = \"{{.preshared_key}}\"\n" +
			"  }\n" +
			"  {{end}}" +
			"  {{if .xauth_user}}" +
			"  xauth-1 {\n" +
			"    id = {{.xauth_user}}\n" +
			"    secret = {{.xauth_password}}\n" +
			"  }\n" +
			"  {{end}}" +
			"  {{if .eap_user}}" +
			"  eap-1 {\n" +
			"    id = \"{{.eap_user}}\"\n" +
			"    secret = \"{{.eap_password}}\"\n" +
			"  }\n" +
			"  {{end}}" +
			"}\n" +
			"pools {\n" +
			"  {{if .client_vip}}" +
			"  ikev2-vip-pools {\n" +
			"    addrs = {{.client_vip}}/32\n" +
			"  }\n" +
			"  {{end}}" +
			"}\n",

		"etc/passwd": "root:x:0:0:root:/root:/bin/bash\n" +
			"vpn:*:20174:20174::/dev/null:/bin/false\n",

		"etc/group": "vpn:x:20174:\n",

		caCertFile:                       certificate.TestCert1().CACred.Cert,
		"etc/swanctl/x509/server.cert":   certificate.TestCert1().ServerCred.Cert,
		"etc/swanctl/private/server.key": certificate.TestCert1().ServerCred.PrivateKey,
	}

	l2tpConfigs = map[string]string{
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
)

// Constants that used by OpenVPN server
const (
	openvpnCommand           = "/usr/sbin/openvpn"
	openvpnConfigFile        = "etc/openvpn/openvpn.conf"
	openvpnCaCertFile        = "etc/openvpn/ca.crt"
	openvpnServerCertFile    = "etc/openvpn/server.crt"
	openvpnServerKeyFile     = "etc/openvpn/server.key"
	openvpnTLSAuthFile       = "etc/openvpn/ta.key"
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

// openvpnTLSAuthKey is the TLS auth key used by OpenVPN. This value is
// generated by `openvpn --genkey --secret ta.key`
const openvpnTLSAuthKey = `-----BEGIN OpenVPN Static key V1-----
f2c67579a2e2b621d935b095d53d71d1
5daf6933da213a80b89e2a29cd949823
a281e06a6ffe8fdad456f6378da0e5a0
5ed7cc168d106925c1b0050c971c6419
dd9c6f0e90064e88ca598987326bcb21
c3eb6f1b0b31b35e0793d4ee10363a4c
f635d584c2008915c45b470a11395037
e8483bba005944f9470fd85ec0b26fe1
0ccd55c6880885855d73dd05af643f36
bbd2b85685bee57aefb7fd569df32598
a7bebf0b0c0ab13a1767eab17184f5f8
d543ee0000ae8b8f5d387e51eaebe5e7
837b9546a008c2c5e53c36d7ed7f1aef
076b9fd4da2383bec4f3d082b8c6f77a
15972ca985c52209e4651105fd76d81e
515ed3702ac44d975e0cce5418960ed2
-----END OpenVPN Static key V1-----`

var (
	openvpnRootDirectories = []string{"etc/openvpn"}
	openvpnConfigs         = map[string]string{
		openvpnCaCertFile:        certificate.TestCert1().CACred.Cert,
		openvpnServerCertFile:    certificate.TestCert1().ServerCred.Cert,
		openvpnServerKeyFile:     certificate.TestCert1().ServerCred.PrivateKey,
		openvpnDiffieHellmanFile: dh1024PemKey,
		openvpnTLSAuthFile:       openvpnTLSAuthKey,
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
			"{{if .tls_auth_file}} tls-auth /{{.tls_auth_file}}\n {{end}}" +
			"verb 5\n" +
			"writepid /{{.pid_file}}\n" +
			"tmp-dir /tmp\n" +
			"{{.optional_user_verification}}\n",
	}
)

// Constants that used by WireGuard server. Keys are generated randomly using
// wireguard-tools, only for test usages.
const (
	wgClientPrivateKey        = "8Ez9VkVl2JL+OhrLZvV2FXsRJTqtBpykhErNef5dzns="
	wgClientPublicKey         = "dN8f5XplOXpNDP1m9b1V3/AVuOogbw+HckGisfEAphA="
	wgClientOverlayIP         = "10.12.14.2"
	wgClientOverlayIPPrefix   = "32"
	wgClientOverlayIPv6       = "fc00:caab:0:1:2::"
	wgClientOverlayIPv6Prefix = "128"
	wgServerPrivateKey        = "kKhUZZYELpnWFXZmHKvze5kMJ4UfViHo0aacwx9VSXo="
	wgServerPublicKey         = "VL4pfwqKV4pWX1xJRmvceOZLTftNKi2PrFoBbJWNKXw="
	wgServerOverlayIP         = "10.12.14.1"
	wgServerOverlayIPv6       = "fc00:caab:0:1:1::"
	wgServerAllowedIPs        = "10.12.0.0/16,fc00:caab:0:1:1::/128"
	wgServerListenPort        = "12345"
	wgSecondServerPrivateKey  = "MKLi0UPHP09PwZDH0EPVd2mMTeGi98NDR8dfkzPuQHs="
	wgSecondServerPublicKey   = "wJXMGS2jhLPy4x75yev7oh92OwjHFcSWio4U/pWLYzg="
	wgSecondServerOverlayIP   = "10.14.16.1"
	wgSecondServerAllowedIPs  = "10.14.0.0/16"
	wgSecondServerListenPort  = "54321"
	wgPresharedKey            = "LqgZ5/qyT8J8nr25n9IEcUi+vOBkd3sphGn1ClhkHw0="
	wgConfigFile              = "tmp/wg.conf"
)

var (
	wgConfigs = map[string]string{
		wgConfigFile: "[Interface]\n" +
			"PrivateKey = {{.server_private_key}}\n" +
			"ListenPort = {{.server_listen_port}}\n" +
			"\n" +
			"[Peer]\n" +
			"PublicKey = {{.client_public_key}}\n" +
			"AllowedIPs = {{.client_ip}}/{{.client_ip_prefix}}\n" +
			"{{if .preshared_key}}PresharedKey = {{.preshared_key}}{{end}}",
	}
	wgClientOverlayIPv4List        = [...]string{"10.12.14.2"}
	wgClientOverlayIPv6List        = [...]string{"fc00:caab:0:1:2::"}
	wgClientOverlayIPv4AndIPv6List = [...]string{"10.12.14.2", "fc00:caab:0:1:2::"}
)

// Server represents a VPN server that can be used in the test.
type Server struct {
	OverlayIP    string
	UnderlayIP   string
	serverRunner *serverRunner
	stopCommands [][]string
	pidFiles     []string
	logFiles     []string
}

// StartL2TPIPsecServer starts a L2TP/IPsec server.
func StartL2TPIPsecServer(ctx context.Context, env *env.Env, authType string, ipsecUseXauth, underlayIPIsOverlayIP bool) (*Server, error) {
	runner := newServerRunner(env)
	server := &Server{
		serverRunner: runner,
		stopCommands: [][]string{},
		pidFiles:     []string{charonPidFile, xl2tpdPidFile, pppdPidFile},
		logFiles:     []string{charonLogFile},
	}

	runner.AddRootDirectories(strongSwanDirectories)
	runner.AddRootDirectories(l2tpDirectories)
	runner.AddConfigTemplates(strongSwanConfigs)
	runner.AddConfigTemplates(l2tpConfigs)

	configValues := map[string]interface{}{
		"chap_user":                chapUser,
		"chap_secret":              chapSecret,
		"charon_logfile":           charonLogFile,
		"xl2tpd_server_ip_address": xl2tpdServerIPAddress,
		"use_underlay_ip":          underlayIPIsOverlayIP,
	}

	switch authType {
	case AuthTypePSK:
		configValues["preshared_key"] = ipsecPresharedKey
	case AuthTypeCert:
		configValues["server_cert_id"] = ikeServerIdentity
		configValues["ca_cert_file"] = caCertFile
	default:
		return nil, errors.Errorf("L2TP/IPsec type %s is not defined", authType)
	}

	if ipsecUseXauth {
		configValues["xauth_user"] = xauthUser
		configValues["xauth_password"] = xauthPassword
	}

	// For running strongSwan VPN with flag --with-piddir=/run/ipsec. We
	// want to use /run/ipsec for strongSwan runtime data dir instead of
	// /run, and the cmdline flag applies to both client and server
	runner.AddStartupCommand(makeIPsecDir)

	runner.AddConfigValues(configValues)
	runner.AddStartupCommand(fmt.Sprintf("%s &", charonCommand))

	xl2tpdCmdStr := fmt.Sprintf("%s -c /%s -C /tmp/l2tpd.control", xl2tpdCommand, xl2tpdConfigFile)
	runner.AddStartupCommand(xl2tpdCmdStr)

	underlayIP, err := runner.Startup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start L2TP/IPsec server")
	}

	// After starting charon, execute `swanctl --load-all` to load the
	// connection config. The execution may fail until the charon process is
	// ready, so we use a Poll here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return runner.RunChroot(ctx, []string{swanctlCommand, "--load-all"})
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to load swanctl config")
	}

	server.UnderlayIP = underlayIP
	if underlayIPIsOverlayIP {
		server.OverlayIP = underlayIP
	} else {
		server.OverlayIP = xl2tpdServerIPAddress
	}
	return server, nil
}

// StartIKEv2Server starts an IKEv2 server.
func StartIKEv2Server(ctx context.Context, env *env.Env, authType string) (*Server, error) {
	runner := newServerRunner(env)
	server := &Server{
		serverRunner: runner,
		stopCommands: [][]string{{"/bin/ip", "link", "del", "xfrm1"}},
		pidFiles:     []string{charonPidFile},
		logFiles:     []string{charonLogFile},
	}

	runner.AddRootDirectories(strongSwanDirectories)
	runner.AddConfigTemplates(strongSwanConfigs)

	configValues := map[string]interface{}{
		"chap_user":      chapUser,
		"chap_secret":    chapSecret,
		"charon_logfile": charonLogFile,
		"client_vip":     ikev2ClientIP,
		"if_id":          ikev2InterfaceID,
	}

	switch authType {
	case AuthTypePSK:
		configValues["client_id"] = ikeClientIdentity
		configValues["preshared_key"] = ipsecPresharedKey
		configValues["server_id"] = ikeServerIdentity
	case AuthTypeCert:
		configValues["ca_cert_file"] = caCertFile
		configValues["server_cert_id"] = ikeServerIdentity
	case AuthTypeEAP:
		configValues["eap_user"] = xauthUser
		configValues["eap_password"] = xauthPassword
		configValues["server_cert_id"] = ikeServerIdentity
	default:
		return nil, errors.Errorf("IKEv2 type %s is not defined", authType)
	}

	runner.AddConfigValues(configValues)

	// For running strongSwan VPN with flag --with-piddir=/run/ipsec. We
	// want to use /run/ipsec for strongSwan runtime data dir instead of
	// /run, and the cmdline flag applies to both client and server
	runner.AddStartupCommand(makeIPsecDir)
	runner.AddStartupCommand(fmt.Sprintf("%s &", charonCommand))
	runner.AddStartupCommand("ip link add xfrm1 type xfrm dev lo if_id " + ikev2InterfaceID)
	runner.AddStartupCommand("ip addr add dev xfrm1 " + ikev2ServerIP + "/24")
	runner.AddStartupCommand("ip link set dev xfrm1 up")

	underlayIP, err := runner.Startup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start IKEv2 VPN server")
	}

	// After starting charon, execute `swanctl --load-all` to load the
	// connection config. The execution may fail until the charon process is
	// ready, so we use a Poll here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return runner.RunChroot(ctx, []string{swanctlCommand, "--load-all"})
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to load swanctl config")
	}

	server.UnderlayIP = underlayIP
	server.OverlayIP = ikev2ServerIP

	return server, nil
}

// StartOpenVPNServer starts an OpenVPN server.
func StartOpenVPNServer(ctx context.Context, env *env.Env, useUserPassword, useTLSAuth bool) (*Server, error) {
	runner := newServerRunner(env)
	server := &Server{
		serverRunner: runner,
		stopCommands: [][]string{},
		pidFiles:     []string{openvpnPidFile},
		logFiles:     []string{openvpnLogFile},
	}

	runner.AddRootDirectories(openvpnRootDirectories)
	runner.AddConfigTemplates(openvpnConfigs)
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
	if useTLSAuth {
		configValues["tls_auth_file"] = openvpnTLSAuthFile
	}

	runner.AddConfigValues(configValues)
	runner.AddStartupCommand("chmod 755 " + openvpnAuthScript)
	runner.AddStartupCommand(fmt.Sprintf("%s --config /%s &", openvpnCommand, openvpnConfigFile))
	runner.NetEnv = []string{
		"OPENSSL_CONF=/etc/ssl/openssl.cnf.compat",
		"OPENSSL_CHROMIUM_SKIP_TRUSTED_PURPOSE_CHECK=1",
	}

	underlayIP, err := runner.Startup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start OpenVPN server")
	}
	server.UnderlayIP = underlayIP
	server.OverlayIP = openvpnServerIPAddress
	return server, nil
}

// StartWireGuardServer starts a WireGuard server.
func StartWireGuardServer(ctx context.Context, env *env.Env, clientPublicKey string, usePSK, useIPv6, isSecondServer bool) (*Server, error) {
	runner := newServerRunner(env)
	server := &Server{
		serverRunner: runner,
		stopCommands: [][]string{{"/bin/ip", "link", "del", "wg1"}},
		pidFiles:     []string{},
		logFiles:     []string{}, // No log for WireGuard server.
	}

	configValues := map[string]interface{}{
		"client_public_key": clientPublicKey,
		"client_ip":         wgClientOverlayIP,
		"client_ip_prefix":  wgClientOverlayIPPrefix,
	}
	if usePSK {
		configValues["preshared_key"] = wgPresharedKey
	}
	if useIPv6 {
		server.OverlayIP = wgServerOverlayIPv6
		configValues["client_ip"] = wgClientOverlayIPv6
		configValues["client_ip_prefix"] = wgClientOverlayIPv6Prefix

	} else {
		server.OverlayIP = wgServerOverlayIP
	}
	if isSecondServer {
		configValues["server_private_key"] = wgSecondServerPrivateKey
		configValues["server_listen_port"] = wgSecondServerListenPort
		server.OverlayIP = wgSecondServerOverlayIP
	}
	if !isSecondServer {

		configValues["server_private_key"] = wgServerPrivateKey
		configValues["server_listen_port"] = wgServerListenPort

	}

	runner.AddConfigTemplates(wgConfigs)
	runner.AddConfigValues(configValues)
	runner.AddStartupCommand("ip link add wg1 type wireguard")
	runner.AddStartupCommand("wg setconf wg1 /" + wgConfigFile)
	runner.AddStartupCommand("ip addr add dev wg1 " + server.OverlayIP)
	runner.AddStartupCommand("ip link set dev wg1 up")
	if isSecondServer {
		runner.AddStartupCommand("ip route add " + wgClientOverlayIP + " dev wg1")
	} else {
		runner.AddStartupCommand("ip route add " + wgClientOverlayIP + " dev wg1")
	}
	if useIPv6 {
		runner.AddStartupCommand("ip route add " + wgClientOverlayIPv6 + " dev wg1")
	}

	var err error
	if server.UnderlayIP, err = runner.Startup(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start WireGuard server")
	}
	return server, nil
}

// StopServer stop VPN server instance.
func (s *Server) StopServer(ctx context.Context) error {
	runner := s.serverRunner
	for _, cmd := range s.stopCommands {
		if err := runner.RunChroot(ctx, cmd); err != nil {
			return errors.Wrapf(err, "failed to execute %v", cmd)
		}
	}

	for _, pidFile := range s.pidFiles {
		if err := runner.KillPidFile(ctx, pidFile, true); err != nil {
			return errors.Wrapf(err, "failed to kill the PID file %v", pidFile)
		}
	}

	return nil
}

func (s *Server) collectLogs(ctx context.Context) error {
	var getLogErr error
	content, err := s.serverRunner.GetLogContents(ctx, s.logFiles)
	if err != nil {
		getLogErr = errors.Wrap(err, "failed to get vpn log contents")
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

	return getLogErr
}

// Exit does a best effort to stop the server, log the contents, and shut down the serverRunner.
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

	if err := s.serverRunner.Shutdown(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to shutdown the serverRunner: ", err)
		lastErr = err
	}

	return lastErr
}

// SetupInternetAccess setup internet connectivity for VPN server.
func (s *Server) SetupInternetAccess(ctx context.Context) error {
	if err := s.serverRunner.RunChroot(ctx, []string{"/sbin/iptables", "-t", "nat", "-A", "POSTROUTING", "!", "-s", s.OverlayIP, "-j", "SNAT", "--to", s.UnderlayIP, "-w"}); err != nil {
		return errors.Wrap(err, "failed to setup internet connectivity")
	}
	return nil
}
