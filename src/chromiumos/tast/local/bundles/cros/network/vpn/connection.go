// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// Config contains the parameters (for both client and server) to configure a
// VPN connection.
type Config struct {
	Type     string
	AuthType string

	// Parameters for an L2TP/IPsec VPN connection.
	IPsecUseXauth         bool
	IPsecXauthMissingUser bool
	IPsecXauthWrongUser   bool
	UnderlayIPIsOverlayIP bool

	// Parameters for an OpenVPN connection.
	OpenVPNUseUserPassword        bool
	OpenVPNCertVerify             bool
	OpenVPNCertVerifyWrongHash    bool
	OpenVPNCertVeirfyWrongSubject bool
	OpenVPNCertVerifyWrongCN      bool
	OpenVPNCertVerifyCNOnly       bool

	// Parameters for a WireGuard connection.
	// WGTwoPeers indicates whether the connection will use one peer or two
	// peers. If true, two peers will be created in two separate network
	// namespace, and the service will use a split routing (for the subnet
	// ranges, see createWireGuardProperties()); if false, the default route
	// ("0.0.0.0/0") to this unique peer will be used.
	WGTwoPeers bool
	// WGAutoGenKey indicates whether letting shill generate the private key for
	// the client side.
	WGAutoGenKey bool
}

// VPN types.
const (
	TypeIKEv2     = "IKEv2"
	TypeL2TPIPsec = "L2TP/IPsec"
	TypeOpenVPN   = "OpenVPN"
	TypeWireGuard = "WireGuard"

	// b/204261554: Temporary VPN types for the two drivers of L2TP/IPsec. Can
	// be removed after the swanctl migration is done.
	TypeL2TPIPsecStroke  = "L2TP/IPsec-stroke"
	TypeL2TPIPsecSwanctl = "L2TP/IPsec-swanctl"
)

// Authentication types.
const (
	AuthTypeCert = "cert"
	AuthTypeEAP  = "eap"
	AuthTypePSK  = "psk"
)

// Connection represents a VPN connection can be used in the test.
type Connection struct {
	Server       *Server
	SecondServer *Server

	config    Config
	manager   *shill.Manager
	certStore *netcertstore.Store
	service   *shill.Service

	// The following TPM-related fields will be set and used when config.AuthType
	// is "cert".
	certID   string
	certSlot string
	certPin  string
}

// NewConnection creates a new connection object. Notes:
// - It is the responsibility of the caller to call Cleanup() when the VPN
//   connection is no longer needed.
// - During connection, we need to modify the profile of shill to configure the
//   VPN client. So the "resetShill" fixture is suggested to make sure that we
//   have a clean shill setup before and after the test.
// Example: the following code can be used to set up a basic L2TP/IPsec VPN
// connection:
//     vpn.NewConnection(ctx, vpn.Config{
//         Type: vpn.TypeL2TPIPsec, AuthType: vpn.AuthTypePSK,
//     })
// Also see vpn_connect.go for a typical usage for this struct.
func NewConnection(ctx context.Context, config Config) (*Connection, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manager proxy")
	}

	return &Connection{
		config:  config,
		manager: manager,
	}, nil
}

// SetUp starts the VPN server and configures the VPN service (client) in shill.
// Callers still need to call Connect() on the connection before it's ready for use.
func (c *Connection) SetUp(ctx context.Context) error {
	// Makes sure that the physical Ethernet service is online before we start,
	// since the physical service change event may affect the VPN connection. We
	// use 60 seconds here for DHCP negotiation since some DUTs will end up
	// retrying DHCP discover/request, and this can often take 15-30 seconds
	// depending on the number of retries.
	props := map[string]interface{}{
		shillconst.ServicePropertyType:  shillconst.TypeEthernet,
		shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
	}
	if _, err := c.manager.WaitForServiceProperties(ctx, props, 60*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for Ethernet online")
	}

	if err := c.prepareCertStore(ctx); err != nil {
		return err
	}

	if err := c.startServer(ctx); err != nil {
		return err
	}

	if err := c.configureService(ctx); err != nil {
		return err
	}
	return nil
}

// Connect lets shill connect to the VPN server. Returns whether the connection is
// established successfully.
func (c *Connection) Connect(ctx context.Context) (bool, error) {
	if connected, err := c.connectService(ctx); err != nil || !connected {
		return false, err
	}

	// Currently, the connected state of a VPN service doesn't mean the VPN
	// service is ready for tunneling traffic: patchpanel needs to setup some
	// iptables rules (for routing and connection pinning) before that. If ping is
	// started before iptables rules are ready, the traffic generated by ping will
	// be "pinned" to the previous default interface. So adds a small timeout here
	// to mitigate this racing case.
	testing.Sleep(ctx, 500*time.Millisecond)
	testing.ContextLogf(ctx, "VPN connected, underlay_ip is %s, overlay_ip is %s", c.Server.UnderlayIP, c.Server.OverlayIP)
	return true, nil
}

// Disconnect will disconnect the shill service. This does not clean up the VPN server
// and callers should still call Cleanup().
func (c *Connection) Disconnect(ctx context.Context) error {
	testing.ContextLog(ctx, "Disconnecting service: ", c.service)
	return c.service.Disconnect(ctx)
}

// Cleanup removes the service from shill, and releases other resources used for
// the connection. Callers don't necessarily need to call Disconnect() before this.
func (c *Connection) Cleanup(ctx context.Context) error {
	var lastErr error

	// Removes service from the profile.
	if c.service != nil {
		if err := c.service.Remove(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to remove service from profile: ", err)
			lastErr = err
		}
	}

	// Wait for charon to stop if service was strongswan-based.
	switch c.config.Type {
	case TypeIKEv2, TypeL2TPIPsec, TypeL2TPIPsecStroke, TypeL2TPIPsecSwanctl:
		if err := waitForCharonStop(ctx); err != nil {
			lastErr = err
		}
	}

	// Shuts down server.
	if c.Server != nil {
		if err := c.Server.Exit(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop VPN server: ", err)
			lastErr = err
		}
	}

	// Cleans up netcertstore.
	if c.certStore != nil {
		if err := c.certStore.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up cert store: ", err)
			lastErr = err
		}
	}

	return lastErr
}

func (c *Connection) prepareCertStore(ctx context.Context) error {
	if c.config.AuthType != AuthTypeCert {
		return nil
	}

	var err error

	runner := hwsec.NewCmdRunner()
	if c.certStore, err = netcertstore.CreateStore(ctx, runner); err != nil {
		return errors.Wrap(err, "failed to create cert store")
	}

	c.certSlot = fmt.Sprintf("%d", c.certStore.Slot())
	c.certPin = c.certStore.Pin()
	clientCred := certificate.TestCert1().ClientCred
	c.certID, err = c.certStore.InstallCertKeyPair(ctx, clientCred.PrivateKey, clientCred.Cert)
	if err != nil {
		return errors.Wrap(err, "failed to insert cert key pair into cert store")
	}

	return nil
}

func (c *Connection) startServer(ctx context.Context) error {
	var err error
	switch c.config.Type {
	case TypeIKEv2:
		c.Server, err = StartIKEv2Server(ctx, c.config.AuthType)
	case TypeL2TPIPsec, TypeL2TPIPsecStroke, TypeL2TPIPsecSwanctl:
		c.Server, err = StartL2TPIPsecServer(ctx, c.config.AuthType, c.config.IPsecUseXauth, c.config.UnderlayIPIsOverlayIP)
	case TypeOpenVPN:
		c.Server, err = StartOpenVPNServer(ctx, c.config.OpenVPNUseUserPassword)
	case TypeWireGuard:
		clientKey := wgClientPublicKey
		if c.config.WGAutoGenKey {
			if clientKey, err = c.generateWireGuardKey(ctx); err != nil {
				return errors.Wrap(err, "failed to get public key")
			}
		}
		c.Server, err = StartWireGuardServer(ctx, clientKey, c.config.AuthType == AuthTypePSK, false /*isSecondServer*/)
		if err == nil && c.config.WGTwoPeers {
			// Always sets preshared key for the second peer.
			c.SecondServer, err = StartWireGuardServer(ctx, clientKey, true /*usePSK*/, true /*isSecondServer*/)
		}
	default:
		return errors.Errorf("unexpected VPN type %s", c.config.Type)
	}
	return err
}

func (c *Connection) configureService(ctx context.Context) error {
	properties, err := c.createProperties()
	if err != nil {
		return errors.Wrap(err, "unable to create service properties")
	}

	servicePath, err := c.manager.ConfigureService(ctx, properties)
	if err != nil {
		return errors.Wrapf(err, "unable to configure the service for the VPN properties %v", properties)
	}

	if c.service, err = shill.NewService(ctx, servicePath); err != nil {
		return errors.Wrap(err, "failed creating shill service proxy")
	}

	return nil
}

// generateWireGuardKey calls configureService() to create an "empty" WireGuard
// service in shill, and then reads and returns the generated public key from
// the service properties. The service created in the profile in this step will
// be overwritten with the full properties after the server is created.
func (c *Connection) generateWireGuardKey(ctx context.Context) (string, error) {
	if err := c.configureService(ctx); err != nil {
		return "", err
	}
	properties, err := c.service.GetProperties(ctx)
	if err != nil {
		return "", err
	}
	provider, err := properties.Get(shillconst.ServicePropertyProvider)
	if err != nil {
		return "", err
	}
	providerMap, ok := provider.(map[string]interface{})
	if !ok {
		return "", errors.New("failed to read Provider property as map")
	}
	publicKey, ok := providerMap["WireGuard.PublicKey"].(string)
	if !ok {
		return "", errors.New("failed to read WireGuard.PublicKey property as string")
	}
	if len(publicKey) != len(wgClientPublicKey) {
		return "", errors.Errorf("generated key is not valid: %s", publicKey)
	}
	return publicKey, nil
}

func (c *Connection) createProperties() (map[string]interface{}, error) {
	switch c.config.Type {
	case TypeIKEv2:
		return c.createIKEv2Properties()
	case TypeL2TPIPsec, TypeL2TPIPsecStroke, TypeL2TPIPsecSwanctl:
		return c.createL2TPIPsecProperties()
	case TypeOpenVPN:
		return c.createOpenVPNProperties()
	case TypeWireGuard:
		return c.createWireGuardProperties(), nil
	default:
		return nil, errors.Errorf("unexpected server type: got %s", c.config.Type)
	}
}

func (c *Connection) createL2TPIPsecProperties() (map[string]interface{}, error) {
	var serverAddress string
	if c.config.UnderlayIPIsOverlayIP {
		serverAddress = c.Server.OverlayIP
	} else {
		serverAddress = c.Server.UnderlayIP
	}

	properties := map[string]interface{}{
		"Provider.Host":      serverAddress,
		"Type":               "vpn",
		"L2TPIPsec.User":     chapUser,
		"L2TPIPsec.Password": chapSecret,
	}

	switch c.config.Type {
	case TypeL2TPIPsec:
		properties["Provider.Type"] = "l2tpipsec"
	case TypeL2TPIPsecStroke:
		properties["Provider.Type"] = "l2tpipsec-stroke"
	case TypeL2TPIPsecSwanctl:
		properties["Provider.Type"] = "l2tpipsec-swanctl"
	default:
		return nil, errors.Errorf("unexpected type: got %s", c.config.Type)
	}

	if c.config.AuthType == AuthTypePSK {
		properties["Name"] = "test-vpn-l2tp-psk"
		properties["L2TPIPsec.PSK"] = ipsecPresharedKey
	} else if c.config.AuthType == AuthTypeCert {
		properties["Name"] = "test-vpn-l2tp-cert"
		properties["L2TPIPsec.CACertPEM"] = []string{certificate.TestCert1().CACred.Cert}
		properties["L2TPIPsec.ClientCertID"] = c.certID
		properties["L2TPIPsec.ClientCertSlot"] = c.certSlot
		properties["L2TPIPsec.PIN"] = c.certPin
	} else {
		return nil, errors.Errorf("unexpected auth type %s for L2TP/IPsec", c.config.AuthType)
	}

	if c.config.IPsecUseXauth && !c.config.IPsecXauthMissingUser {
		if c.config.IPsecXauthWrongUser {
			properties["L2TPIPsec.XauthUser"] = "wrong-user"
			properties["L2TPIPsec.XauthPassword"] = "wrong-password"
		} else {
			properties["L2TPIPsec.XauthUser"] = xauthUser
			properties["L2TPIPsec.XauthPassword"] = xauthPassword
		}
	}

	return properties, nil
}

func (c *Connection) createIKEv2Properties() (map[string]interface{}, error) {
	properties := map[string]interface{}{
		"Name":          "test-ikev2-vpn",
		"Provider.Host": c.Server.UnderlayIP,
		"Provider.Type": "ikev2",
		"Type":          "vpn",
	}

	switch c.config.AuthType {
	case AuthTypePSK:
		properties["IKEv2.AuthenticationType"] = "PSK"
		properties["IKEv2.LocalIdentity"] = ikeClientIdentity
		properties["IKEv2.RemoteIdentity"] = ikeServerIdentity
		properties["IKEv2.PSK"] = ipsecPresharedKey
	case AuthTypeCert:
		properties["IKEv2.AuthenticationType"] = "Cert"
		properties["IKEv2.CACertPEM"] = []string{certificate.TestCert1().CACred.Cert}
		properties["IKEv2.ClientCertID"] = c.certID
		properties["IKEv2.ClientCertSlot"] = c.certSlot
		properties["IKEv2.RemoteIdentity"] = ikeServerIdentity
	case AuthTypeEAP:
		properties["IKEv2.AuthenticationType"] = "EAP"
		properties["IKEv2.CACertPEM"] = []string{certificate.TestCert1().CACred.Cert}
		properties["EAP.EAP"] = "MSCHAPV2"
		properties["EAP.Identity"] = xauthUser
		properties["EAP.Password"] = xauthPassword
	default:
		return nil, errors.Errorf("unexpected auth type %s for IKEv2", c.config.AuthType)
	}

	return properties, nil
}

func (c *Connection) createOpenVPNProperties() (map[string]interface{}, error) {
	properties := map[string]interface{}{
		"Name":                    "test-vpn-openvpn",
		"Provider.Host":           c.Server.UnderlayIP,
		"Provider.Type":           "openvpn",
		"Type":                    "vpn",
		"OpenVPN.CACertPEM":       []string{certificate.TestCert1().CACred.Cert},
		"OpenVPN.Pkcs11.ID":       c.certID,
		"OpenVPN.Pkcs11.PIN":      c.certPin,
		"OpenVPN.RemoteCertEKU":   "TLS Web Server Authentication",
		"OpenVPN.TLSAuthContents": openvpnTLSAuthKey,
		"OpenVPN.Verb":            "5",
	}

	if c.config.OpenVPNUseUserPassword {
		properties["OpenVPN.User"] = openvpnUsername
		properties["OpenVPN.Password"] = openvpnPassword
	}

	if c.config.OpenVPNCertVerify {
		if c.config.OpenVPNCertVerifyWrongHash {
			properties["OpenVPN.VerifyHash"] = "00" + strings.Repeat(":00", 19)
		} else {
			certBlock, _ := pem.Decode([]byte(certificate.TestCert1().CACred.Cert))
			caCert, err := x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse CA cert")
			}
			// Translates the form of SHA-1 hash from []byte to "xx:xx:...:xx".
			properties["OpenVPN.VerifyHash"] = strings.ReplaceAll(fmt.Sprintf("% 02x", sha1.Sum(caCert.Raw)), " ", ":")
		}

		if c.config.OpenVPNCertVeirfyWrongSubject {
			properties["OpenVPN.VerifyX509Name"] = "bogus subject name"
		} else if c.config.OpenVPNCertVerifyWrongCN {
			properties["OpenVPN.VerifyX509Name"] = "bogus cn"
			properties["OpenVPN.VerifyX509Type"] = "name"
		} else if c.config.OpenVPNCertVerifyCNOnly {
			// This can be parsed from certificate.TestCert1().ServerCred.Cert .
			properties["OpenVPN.VerifyX509Name"] = "chromelab-wifi-testbed-server.mtv.google.com"
			properties["OpenVPN.VerifyX509Type"] = "name"
		} else {
			// This can be parsed from certificate.TestCert1().ServerCred.Cert, but
			// the output format of String() function of the parsed result does not
			// match the format that OpenVPN expects.
			properties["OpenVPN.VerifyX509Name"] = "C=US, ST=California, L=Mountain View, CN=chromelab-wifi-testbed-server.mtv.google.com"
		}
	}

	return properties, nil
}

func (c *Connection) createWireGuardProperties() map[string]interface{} {
	var peers []map[string]string
	if c.Server != nil {
		peer := map[string]string{
			"PublicKey":  wgServerPublicKey,
			"Endpoint":   c.Server.UnderlayIP + ":" + wgServerListenPort,
			"AllowedIPs": "0.0.0.0/0",
		}
		if c.config.AuthType == AuthTypePSK {
			peer["PresharedKey"] = wgPresharedKey
		}
		if c.config.WGTwoPeers {
			// Do not set "default route" if we have two peers.
			peer["AllowedIPs"] = wgServerAllowedIPs
		}
		peers = append(peers, peer)
	}

	if c.SecondServer != nil {
		peers = append(peers, map[string]string{
			"PublicKey":    wgSecondServerPublicKey,
			"Endpoint":     c.SecondServer.UnderlayIP + ":" + wgSecondServerListenPort,
			"AllowedIPs":   wgSecondServerAllowedIPs,
			"PresharedKey": wgPresharedKey,
		})
	}

	staticIPConfig := map[string]interface{}{
		"Address": wgClientOverlayIP,
	}
	properties := map[string]interface{}{
		"Name":            "test-vpn-wg",
		"Provider.Host":   "wireguard",
		"Provider.Type":   "wireguard",
		"Type":            "vpn",
		"WireGuard.Peers": peers,
		"StaticIPConfig":  staticIPConfig,
		"SaveCredentials": true, // Not required, just to avoid a WARNING log in shill
	}
	if !c.config.WGAutoGenKey {
		properties["WireGuard.PrivateKey"] = wgClientPrivateKey
	}
	return properties
}

func (c *Connection) connectService(ctx context.Context) (bool, error) {
	// Waits for service to be connected.
	testing.ContextLog(ctx, "Connecting to service: ", c.service)

	// Spawns watcher before connect.
	pw, err := c.service.CreateWatcher(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	if err = c.service.Connect(ctx); err != nil {
		return false, errors.Wrapf(err, "failed to connect the service %v", c.service)
	}

	// Waits until connection established or failed. Unfortunately, some of the
	// failures for L2TP/IPsec VPN are detected based on timeout, which is 30
	// seconds at maximum for all the current test cases (that value is used in
	// vpn_manager::IpsecManager).
	// TODO(b/188489413): Use different timeout values for success and failure
	// cases.
	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	state, err := pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, append(shillconst.ServiceConnectedStates, shillconst.ServiceStateFailure))
	if err != nil {
		return false, err
	}

	return state != shillconst.ServiceStateFailure, nil
}

// ExpectPingSuccess pings 'addr' and expects the ping to succeed. Returns an error otherwise.
func ExpectPingSuccess(ctx context.Context, pr *ping.Runner, addr string) error {
	testing.ContextLog(ctx, "Start to ping ", addr)
	res, err := pr.Ping(ctx, addr, ping.Count(3), ping.User("chronos"))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received")
	}
	return nil
}
