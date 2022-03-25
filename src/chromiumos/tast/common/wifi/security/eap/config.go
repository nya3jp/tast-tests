// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package eap is an abstract class for EAP security classes which need certificate/key installation.
package eap

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wificell/router"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Config implements security.Config interface for EAP protected network.
type Config struct {
	// fileSuffix is the file name suffix of the cert/key files which are installed onto the router.
	fileSuffix string
	// identity is the client identity used by shill for setting up service of type "802.1x".
	identity string

	serverCACert   string
	serverCred     certificate.Credential
	serverEAPUsers string

	clientCACert string
	clientCred   certificate.Credential

	// Fields that would be set in InstallRouterCredentials().
	ServerCACertFile   string
	ServerCertFile     string
	ServerKeyFile      string
	ServerEAPUsersFile string

	// Fields that would be set in InstallClientCredentials().
	ClientSlotID int
	Pin          string
	NetCertID    string
}

// Class returns the security class of EAP network.
func (c *Config) Class() string {
	return shillconst.SecurityClass8021x
}

// HostapdConfig returns hostapd config of EAP network.
func (c *Config) HostapdConfig() (map[string]string, error) {
	if c.ServerCACertFile == "" || c.ServerCertFile == "" || c.ServerKeyFile == "" || c.ServerEAPUsersFile == "" {
		return nil, errors.New("router credentials are not installed")
	}
	return map[string]string{
		"ieee8021x":     "1",
		"eap_server":    "1", // Do EAP inside hostapd to avoid RADIUS.
		"ca_cert":       c.ServerCACertFile,
		"server_cert":   c.ServerCertFile,
		"private_key":   c.ServerKeyFile,
		"eap_user_file": c.ServerEAPUsersFile,
	}, nil
}

// ShillServiceProperties returns shill properties of EAP network.
func (c *Config) ShillServiceProperties() (map[string]interface{}, error) {
	// For c.ClientSlotID, 0 is a system slot but not a user slot,
	// which means that InstallClientCredentials has not been called.
	if c.NeedsNetCertStore() && (c.Pin == "" || c.ClientSlotID == 0) {
		return nil, errors.New("client credentials are not installed")
	}

	ret := map[string]interface{}{shillconst.ServicePropertyEAPIdentity: c.identity}

	if c.Pin != "" {
		ret[shillconst.ServicePropertyEAPPin] = c.Pin
	}
	if c.clientCACert != "" {
		// Technically, we could accept a list of certificates here, but we have no such tests.
		ret[shillconst.ServicePropertyEAPCACertPEM] = []string{c.clientCACert}
	}
	if c.clientCred.Cert != "" {
		ret[shillconst.ServicePropertyEAPCertID] = fmt.Sprintf("%d:%s", c.ClientSlotID, c.NetCertID)
	}
	if c.clientCred.PrivateKey != "" {
		ret[shillconst.ServicePropertyEAPKeyID] = fmt.Sprintf("%d:%s", c.ClientSlotID, c.NetCertID)
	}

	return ret, nil
}

// InstallRouterCredentials installs the necessary credentials onto router.
func (c *Config) InstallRouterCredentials(ctx context.Context, host *ssh.Conn, workDir string) error {
	ctx, st := timing.Start(ctx, "eap.InstallRouterCredentials")
	defer st.End()

	pathMap := make(map[string]string)

	c.ServerCACertFile = filepath.Join(workDir, "hostapd_ca_cert_file."+c.fileSuffix)
	c.ServerCertFile = filepath.Join(workDir, "hostapd_cert_file."+c.fileSuffix)
	c.ServerKeyFile = filepath.Join(workDir, "hostapd_key_file."+c.fileSuffix)
	c.ServerEAPUsersFile = filepath.Join(workDir, "hostapd_eap_user_cert_file."+c.fileSuffix)

	for _, f := range []struct {
		content string
		path    string
	}{
		{c.serverCACert, c.ServerCACertFile},
		{c.serverCred.Cert, c.ServerCertFile},
		{c.serverCred.PrivateKey, c.ServerKeyFile},
		{c.serverEAPUsers, c.ServerEAPUsersFile},
	} {
		tmpfile, err := ioutil.TempFile("", "upload_tmp_")
		if err != nil {
			return errors.Wrap(err, "unable to create temp file")
		}
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(f.content))
		tmpfile.Close()
		if err != nil {
			return errors.Wrap(err, "unable to write to temp file")
		}

		pathMap[tmpfile.Name()] = f.path
	}

	if _, err := router.PutFiles(ctx, host, pathMap); err != nil {
		return errors.Wrap(err, "unable to upload file to host")
	}

	// Log settings
	credLog, err := json.Marshal(map[string]string{
		c.ServerCACertFile:   c.serverCACert,
		c.ServerCertFile:     c.serverCred.Cert,
		c.ServerKeyFile:      c.serverCred.PrivateKey,
		c.ServerEAPUsersFile: c.serverEAPUsers,
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshall credLog json summary")
	}
	testing.ContextLogf(ctx, "Installed router credentials: %s", string(credLog))
	return nil
}

// NeedsNetCertStore tells that netcert store is necessary for this test.
func (c *Config) NeedsNetCertStore() bool {
	return c.hasClientCred()
}

// InstallClientCredentials installs credentials on the DUT.
func (c *Config) InstallClientCredentials(ctx context.Context, store *netcertstore.Store) error {
	ctx, st := timing.Start(ctx, "eap.InstallClientCredentials")
	defer st.End()

	if !c.hasClientCred() {
		return nil
	}

	c.Pin = store.Pin()
	c.ClientSlotID = store.Slot()
	netCertID, err := store.InstallCertKeyPair(ctx, c.clientCred.PrivateKey, c.clientCred.Cert)
	if err != nil {
		return err
	}
	c.NetCertID = netCertID

	return nil
}

func (c *Config) hasClientCred() bool {
	return c.clientCred.Cert != "" && c.clientCred.PrivateKey != ""
}

// validate validates the Config.
func (c *Config) validate() error {
	if c.identity == "" {
		return errors.New("no EAP identity is set")
	}
	if c.serverCACert == "" {
		return errors.New("no CA certificate is set on server")
	}
	if c.serverCred.Cert == "" {
		return errors.New("no certificate is set on server")
	}
	if c.serverCred.PrivateKey == "" {
		return errors.New("no private key is set on server")
	}
	if c.serverEAPUsers == "" {
		return errors.New("no EAP users is set on server")
	}
	if (c.clientCred.Cert != "") != (c.clientCred.PrivateKey != "") {
		return errors.New("client Cret and PrivateKey should be either both set or both not set")
	}
	return nil
}

// ConfigFactory holds some Option and provides Gen method to build a new Config.
type ConfigFactory struct {
	serverCACert string
	serverCred   certificate.Credential
	ops          []Option
}

// NewConfigFactory builds a ConfigFactory with the given Option.
func NewConfigFactory(serverCACert string, serverCred certificate.Credential, ops ...Option) *ConfigFactory {
	return &ConfigFactory{
		serverCACert: serverCACert,
		serverCred:   serverCred,
		ops:          ops,
	}
}

// Gen builds a Config with the given Option stored in ConfigFactory.
func (f *ConfigFactory) Gen() (security.Config, error) {
	// Default config.
	conf := &Config{
		identity:       "chromeos",
		serverCACert:   f.serverCACert,
		serverCred:     f.serverCred,
		serverEAPUsers: "* TLS",
	}

	for _, op := range f.ops {
		op(conf)
	}

	if conf.fileSuffix == "" {
		conf.fileSuffix = randomSuffix()
	}

	if err := conf.validate(); err != nil {
		return nil, err
	}

	return conf, nil
}

// randomSuffix returns a random suffix of length 10.
func randomSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 10)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)
