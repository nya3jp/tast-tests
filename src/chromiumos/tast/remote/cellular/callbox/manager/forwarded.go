// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	labProxyHostname                    = "chromeos1-proxy"
	labProxyUser                        = "access"
	labProxyCallboxManagerRemoteAddress = "localhost:5000"
)

type forwardedCallboxManager struct {
	host               *ssh.Conn
	localPortForwarder *ssh.Forwarder
}

type forwardedCallboxManagerConfig struct {
	sshKeyDir                   string
	sshKeyFile                  string
	sshUser                     string
	sshHostname                 string
	remoteCallboxManagerAddress string
}

func newForwardedCallboxManager(ctx context.Context, config *forwardedCallboxManagerConfig) (*forwardedCallboxManager, error) {
	fcm := &forwardedCallboxManager{}
	// Connect to host
	sshOptions := &ssh.Options{
		KeyFile:        config.sshKeyFile,
		KeyDir:         config.sshKeyDir,
		ConnectTimeout: 10 * time.Second,
		User:           config.sshUser,
		Hostname:       fmt.Sprintf("%s:%d", config.sshHostname, 22),
	}
	conn, err := ssh.New(ctx, sshOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", sshOptions.Hostname)
	}
	fcm.host = conn

	// Forward local port to remote CallboxManager
	onFwdError := func(err error) {
		testing.ContextLog(ctx, "Ssh forwarding error: ", err)
	}
	fcm.localPortForwarder, err = conn.ForwardLocalToRemote("tcp", "localhost", config.remoteCallboxManagerAddress, onFwdError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to forward local port to remote callbox manager")
	}
	return fcm, nil
}

func newForwardToLabCallboxManager(ctx context.Context, sshKeyDir, sshKeyFile string) (*forwardedCallboxManager, error) {
	return newForwardedCallboxManager(ctx, &forwardedCallboxManagerConfig{
		sshKeyDir:                   sshKeyDir,
		sshKeyFile:                  sshKeyFile,
		sshUser:                     labProxyUser,
		sshHostname:                 labProxyHostname,
		remoteCallboxManagerAddress: labProxyCallboxManagerRemoteAddress,
	})
}

// LocalAddress returns the local address that is forwarded to the remote
// address. This is what should be used to interact with the forwarded
// Callbox Manager.
func (fcm *forwardedCallboxManager) LocalAddress() string {
	return fcm.localPortForwarder.ListenAddr().String()
}

// Close closes the local port forward and the connection to the host.
func (fcm *forwardedCallboxManager) Close(ctx context.Context) error {
	var firstError error
	if err := fcm.Close(ctx); err != nil {
		firstError = err
	}
	if err := fcm.host.Close(ctx); err != nil && firstError != nil {
		firstError = err
	}
	return firstError
}
