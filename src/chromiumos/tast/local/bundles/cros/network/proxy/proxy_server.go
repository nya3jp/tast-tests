// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package proxy allows running a proxy server on the DUT for tests.
package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	patchpanel "chromiumos/tast/local/bundles/cros/network/patchpanel_client"
	"chromiumos/tast/local/testexec"
)

const (
	proxyServerBin       = "/usr/bin/tinyproxy"
	proxyServerConfig    = "/etc/tinyproxy/tinyproxy.conf"
	proxyPidDirectory    = ""
	proxyConfigDirectory = ""
	proxyPort            = 3128
)

type proxyServer struct {
	HostAndPort string
	pid         int
	lifelineDf  *os.File
}

// AuthCredentials can be used to specify an authenticated user with the Basic scheme for the proxy.
type AuthCredentials struct {
	Username string
	Password string
}

// NewProxyServer creates a new proxyServer.
func NewProxyServer() *proxyServer {
	return &proxyServer{pid: -1}
}

// StartServer starts a proxy server. If |AuthCredentials| is specified, access to the proxy is only
// granted for authenticated users. If proxy configuration and setup is successful, |HostAndPort| will
// be set to the host and port of the local proxy in the format <host>:<port>.
func (s *proxyServer) StartServer(ctx context.Context, auth *AuthCredentials) error {
	pidPath, err := createTinyproxyConfig(auth)
	if err != nil {
		return errors.Wrap(err, "failed to create the proxy config file")
	}

	if err := testexec.CommandContext(ctx, "/sbin/minijail0", "-i", "-e", proxyServerBin).Run(); err != nil {
		return errors.Wrap(err, "failed to start sandboxed proxy server")
	}

	if err := s.configureNetwork(ctx, pidPath); err != nil {
		return errors.Wrap(err, "failed to setup the network namespace")
	}

	return nil
}

// StopServer stops the proxy server instance.
func (s *proxyServer) StopServer(ctx context.Context) error {
	// Closing the fd will signal to patchpanel that it needs to tear down the network namespace
	// for the local proxy server.
	s.lifelineDf.Close()
	if err := testexec.CommandContext(ctx, "killall", proxyServerBin).Run(); err != nil {
		return errors.Wrap(err, "error stoping proxy server")
	}
	return nil
}

// createTinyproxyConfig creates the proxy configuration file and an additional file where tinyproxy
// will write the pid of the tinyproxy process. The pid will be used when invoking patchpanel to setup
// the network namespace. Returns the filename containing the process pid in case of success, otherwise
// returns error.
func createTinyproxyConfig(auth *AuthCredentials) (string, error) {
	// tinyproxy configuration file
	c := `# User and group for the tinyproxy proxy.
	User tinyproxy
	Group tinyproxy

	# Port where tinyproxy will listen on.
	Port %d

	# Max seconds of inactivity a connection is allowed to have before it is closed by tinyproxy.
	Timeout 600

	# The file that gets sent if there is an HTTP error that has occured.
	DefaultErrorFile "/usr/share/tinyproxy/default.html"

	LogLevel Info

	# Write the PID of the main tinyproxy thread to this file so it
	# can be used by patchpanel to create a network namespace.
	PidFile "%s"

	# Max number of threads which will be created.
	MaxClients 100

	# These settings set the upper and lower limit for the number of spare servers which should be available.
	MinSpareServers 5
	MaxSpareServers 10

	# StartServers: The number of servers to start initially.
	StartServers 5

	# The number of connections a thread will handle before it is killed.
	MaxRequestsPerChild 0
	`
	// create the file where the pid of the proxy server will be saved
	pidFile, err := createTempFile()
	if err != nil {
		return "", err
	}

	c = fmt.Sprintf(c, proxyPort, pidFile)

	if auth != nil {
		// Credentials for basic authentication
		c += fmt.Sprintf("BasicAuth %s %s \n", auth.Username, auth.Password)
	}

	f, err := os.OpenFile(proxyServerConfig, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.WriteString(c); err != nil {
		return "", err
	}
	return pidFile, nil
}

// createTempFile creates a teporary file where tinyproxy will write it's pid.
func createTempFile() (string, error) {
	td, err := ioutil.TempDir("", "tinyproxy-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	file, err := ioutil.TempFile(td, "tinyproxy-pid-")
	if err != nil {
		return "", errors.Wrapf(err, "failed to chmod %v", file)
	}
	if err := os.Chmod(file.Name(), 0755); err != nil {
		os.RemoveAll(td)
		return "", errors.Wrapf(err, "failed to chmod %v", td)
	}
	return file.Name(), nil
}

// configureNetwork calls patchpanel to setup the network namespace for the local proxy.
func (s *proxyServer) configureNetwork(ctx context.Context, pidPath string) error {
	pc, err := patchpanel.New(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create patchpanel client")
	}

	dat, err := ioutil.ReadFile(pidPath)
	if err != nil {
		return errors.Wrap(err, "failed to read proxy process pid")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(dat)))
	if err != nil {
		return errors.Wrap(err, "failed to get proxy process pid")
	}

	fd, resp, err := pc.ConnectNamespace(ctx, int32(pid), "", true)
	if err != nil {
		return err
	}
	s.lifelineDf = fd

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(resp.HostIpv4Address))
	ip := net.IP(b)
	s.HostAndPort = fmt.Sprintf("%s:%d", ip.String(), proxyPort)

	return nil
}
