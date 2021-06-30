// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package proxy allows running an http proxy server on the DUT for tests.
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
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
	"chromiumos/tast/testing"
)

const (
	proxyServerBin    = "/usr/local/bin/tinyproxy"
	proxyServerConfig = "/etc/tinyproxy/tinyproxy.conf"
	filterConfig      = `
FilterExtended On
FilterDefaultDeny Yes
Filter "%s"
`
)

// Server represents a HTTP proxy server running on the DUT.
type Server struct {
	HostAndPort string
	pid         int
	lifelineFD  *os.File
	tempDir     string
}

// AuthCredentials can be used to specify an authenticated user with the Basic scheme for the proxy.
type AuthCredentials struct {
	Username string
	Password string
}

// NewServer creates a new Server.
func NewServer() *Server {
	return &Server{pid: -1}
}

// Start starts a proxy server. `port` must be a valid port number where the proxy will listen for connections.
// The proxy address will be assigned by patchpanel. If `auth` is specified, access to the proxy is only granted for
// authenticated users. If proxy configuration and setup are successful, `s.HostAndPort` will be set to the host and
// port of the local proxy in the format <host>:<port>.
// If `allowlist` is not empty, only connection to hosts specified in `allowlist` are allowed. The hostnames will be matched
// using reqular expressions.
func (s *Server) Start(ctx context.Context, port int, auth *AuthCredentials, allowlist []string) (retErr error) {
	// Create a temp dir where configuration and pid files can be saved.
	tempDir, err := ioutil.TempDir("", "tinyproxy-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(tempDir)
		}
	}()
	s.tempDir = tempDir
	pidFile, err := s.createTempFile()
	if err != nil {
		return errors.Wrap(err, "failed to create PID file")
	}
	configFile, err := s.createTinyproxyConfig(ctx, port, auth, pidFile, allowlist)
	if err != nil {
		return errors.Wrap(err, "failed to create the proxy config file")
	}

	if err := testexec.CommandContext(ctx, "/sbin/minijail0", "-e", proxyServerBin, "-c", configFile).Run(); err != nil {
		return errors.Wrap(err, "failed to start sandboxed proxy server")
	}

	if err := s.configureNetwork(ctx, port, pidFile); err != nil {
		return errors.Wrap(err, "failed to setup the network namespace")
	}

	return nil
}

// Stop stops the proxy server instance.
func (s *Server) Stop(ctx context.Context) error {
	// Closing the fd will signal to patchpanel that it needs to tear down the network namespace
	// for the local proxy server.
	s.lifelineFD.Close()
	os.RemoveAll(s.tempDir)
	return testexec.CommandContext(ctx, "killall", proxyServerBin).Run()
}

// createTinyproxyConfig creates the proxy configuration file. Returns the config filename in case of success,
// otherwise returns error.
func (s *Server) createTinyproxyConfig(ctx context.Context, port int, auth *AuthCredentials, pidFileName string, allowlist []string) (string, error) {
	// tinyproxy configuration file
	c := `# User and group for the tinyproxy proxy.
User tinyproxy
Group tinyproxy

# Port where tinyproxy will listen on.
Port %d

# Max seconds of inactivity a connection is allowed to have before it is closed by tinyproxy.
Timeout 10

# The file that gets sent if there is an HTTP error that has occured.
DefaultErrorFile "/usr/share/tinyproxy/default.html"

LogLevel Info

# Write the PID of the main tinyproxy thread to this file so it
# can be used by patchpanel to create a network namespace.
PidFile "%s"

# Max number of threads which will be created.
MaxClients 300

# These settings set the upper and lower limit for the number of spare servers which should be available.
MinSpareServers 5
MaxSpareServers 30

# The number of servers to start initially.
StartServers 10
`
	c = fmt.Sprintf(c, port, pidFileName)

	if auth != nil {
		// Credentials for basic authentication
		c += fmt.Sprintf("BasicAuth %s %s\n", auth.Username, auth.Password)
	}

	if len(allowlist) > 0 {
		hosts := strings.Join(allowlist, "\n")

		filterFile, err := s.createTempFile()
		if err != nil {
			return "", errors.Wrapf(err, "failed to create tinyproxy filter file %s", hosts)
		}

		f, err := os.OpenFile(filterFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return "", errors.Wrapf(err, "failed to open filter file %s for writing", filterFile)
		}
		defer f.Close()

		if _, err := f.WriteString(hosts); err != nil {
			return "", errors.Wrapf(err, "failed to write hostnames to file %s", filterFile)
		}

		c += fmt.Sprintf(filterConfig, filterFile)
	}

	configFile, err := s.createTempFile()
	if err != nil {
		return "", errors.Wrap(err, "failed to create tinyproxy config file")
	}

	f, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.WriteString(c); err != nil {
		return "", err
	}
	return configFile, nil
}

// createTempFile creates a temporary file in proxy server's temporary directory and returns the
// path. The temp files will be removed in case of error or when the proxy server is stopped.
func (s *Server) createTempFile() (string, error) {
	file, err := ioutil.TempFile(s.tempDir, "tinyproxy-pid-")
	if err != nil {
		return "", errors.Wrap(err, "failed create temp file")
	}
	defer file.Close()
	if err := os.Chmod(file.Name(), 0755); err != nil {
		return "", errors.Wrapf(err, "failed to chmod %v", file.Name())
	}
	return file.Name(), nil
}

// configureNetwork calls patchpanel to setup the network namespace for the local proxy.
func (s *Server) configureNetwork(ctx context.Context, port int, pidPath string) error {
	pc, err := patchpanel.New(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create patchpanel client")
	}

	pid, err := s.getPidFromPath(ctx, pidPath)
	if err != nil {
		return errors.Wrap(err, "failed to get proxy process pid")
	}

	fd, resp, err := pc.ConnectNamespace(ctx, int32(pid), "", true)
	if err != nil {
		return err
	}
	s.lifelineFD = fd

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(resp.PeerIpv4Address))
	ip := net.IP(b)
	s.HostAndPort = fmt.Sprintf("%s:%d", ip.String(), port)

	return nil
}

// getPidFromPath reads the pid of the tinyproxy process from `pidPath`. The pid is required by
// patchpanel to setup an isolated network namespace for the proxy server with it's own network
// address.
func (s *Server) getPidFromPath(ctx context.Context, pidPath string) (int, error) {
	var pid int
	// Wait for the tinyproxy service to write the PID at `pidPath`.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dat, err := ioutil.ReadFile(pidPath)
		if err != nil {
			return errors.Wrap(err, "failed to read proxy process pid")
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(dat)))
		if err != nil {
			return errors.Wrap(err, "failed to parse process pid")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return 0, err
	}

	return pid, nil
}
