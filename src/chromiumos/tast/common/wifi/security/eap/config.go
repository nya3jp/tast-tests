// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package eap is an abstract class for EAP security classes which need certificate/key installation.
package eap

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
)

const (
	systemSlotID = 0
	systemPin    = "111111"
	dutWorkDir   = "/tmp/wifi-tast-test"
)

// lastTPMID stores a session unique TPM identifier.
var lastTPMID = uint64(0)

// reserveTPMID returns a session unique hexadecimal string to be TPM identifier.
// The TPM identifier should be a hexadecimal string able to be converted to byte array,
// that is, it should be a string with even length containing only [a-fA-F0-9].
// Every call to NewTPMStore create a blank keystore so all ids are acceptable.
func reserveTPMID() string {
	b := make([]byte, binary.Size(lastTPMID))
	binary.BigEndian.PutUint64(b, lastTPMID)
	lastTPMID++
	return hex.EncodeToString(b)
}

// Config implements security.Config interface for EAP protected network.
type Config struct {
	// fileSuffix is the file name suffix of the cert/key files which are installed onto the router.
	fileSuffix string
	// identity is the client identity used by shill for setting up service of type "802.1x".
	identity string
	// tpmID is the PKCS#11 identifier of the client cert/key used by shill for setting up services of type "802_1x".
	tpmID string

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
}

// Class returns the security class of EAP network.
func (c *Config) Class() string {
	return shill.Security8021x
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
	ret := map[string]interface{}{
		shill.ServicePropertyEAPIdentity: c.identity,
		shill.ServicePropertyEAPPin:      systemPin,
	}

	if c.clientCACert != "" {
		// Technically, we could accept a list of certificates here, but we have no such tests.
		ret[shill.ServicePropertyEAPCACertPEM] = []string{c.clientCACert}
	}
	if c.clientCred.Cert != "" {
		ret[shill.ServicePropertyEAPCertID] = fmt.Sprintf("%d:%s", systemSlotID, c.tpmID)
	}
	if c.clientCred.PrivateKey != "" {
		ret[shill.ServicePropertyEAPKeyID] = fmt.Sprintf("%d:%s", systemSlotID, c.tpmID)
	}

	return ret, nil
}

func putFiles(ctx context.Context, s *ssh.Conn, filesAndContents map[string]string) error {
	pathMap := make(map[string]string)

	for path, content := range filesAndContents {
		tmpfile, err := ioutil.TempFile("", "upload_tmp_")
		if err != nil {
			return errors.Wrap(err, "unable to create temp file")
		}
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		tmpfile.Close()
		if err != nil {
			return errors.Wrap(err, "unable to write to temp file")
		}

		pathMap[tmpfile.Name()] = path
	}

	if _, err := linuxssh.PutFiles(ctx, s, pathMap, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "unable to upload file to host")
	}

	return nil
}

// InstallRouterCredentials installs the necessary credentials onto router.
func (c *Config) InstallRouterCredentials(ctx context.Context, host *ssh.Conn, workDir string) error {

	c.ServerCACertFile = filepath.Join(workDir, "hostapd_ca_cert_file."+c.fileSuffix)
	c.ServerCertFile = filepath.Join(workDir, "hostapd_cert_file."+c.fileSuffix)
	c.ServerKeyFile = filepath.Join(workDir, "hostapd_key_file."+c.fileSuffix)
	c.ServerEAPUsersFile = filepath.Join(workDir, "hostapd_eap_user_cert_file."+c.fileSuffix)

	return putFiles(ctx, host, map[string]string{
		c.ServerCACertFile:   c.serverCACert,
		c.ServerCertFile:     c.serverCred.Cert,
		c.ServerKeyFile:      c.serverCred.PrivateKey,
		c.ServerEAPUsersFile: c.serverEAPUsers,
	})
}

// InstallClientCredentials installs credentials on the DUT.
func (c *Config) InstallClientCredentials(ctx context.Context, chaps *pkcs11.Chaps, d *dut.DUT) (retErr error) {
	if !c.hasClientCred() {
		return nil
	}

	// Create a working directory on DUT. May be moved to and controled by TestFixture.
	if err := d.Command("mkdir", dutWorkDir).Run(ctx); err != nil {
		return errors.Wrapf(err, "failed to create working directory %s", dutWorkDir)
	}
	defer func() {
		if err := d.Command("rm", "-rf", dutWorkDir).Run(ctx); err != nil && retErr == nil {
			retErr = errors.Wrapf(err, "failed to remove working directory %s", dutWorkDir)
		}
	}()

	keyPath := filepath.Join(dutWorkDir, "private_key")
	certPath := filepath.Join(dutWorkDir, "certificate")

	if err := putFiles(ctx, d.Conn(), map[string]string{
		keyPath:  c.clientCred.PrivateKey,
		certPath: c.clientCred.Cert,
	}); err != nil {
		return err
	}

	_, err := chaps.ImportPEMPrivateKeyAndCertBySlot(ctx, dutWorkDir, keyPath, certPath, c.tpmID, systemSlotID, false)
	if err != nil {
		return errors.Wrap(err, "failed to import")
	}
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
	if _, err := hex.DecodeString(c.tpmID); err != nil {
		return errors.Wrap(err, "invalid tpmID")
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
	if conf.tpmID == "" {
		conf.tpmID = reserveTPMID()
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
