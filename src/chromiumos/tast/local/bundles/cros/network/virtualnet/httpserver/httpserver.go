// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package httpserver provides the utils to run an httpserver inside a
// virtualnet.Env.
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/testing"
)

// Paths in chroot.
const (
	logPath = "/tmp/httpServer.log"
)

// Paths in root.
const (
	tmpCrtPath    = "/tmp/test_certs"
	serverKeyPath = tmpCrtPath + "/server_key.key"
	serverCrtPath = tmpCrtPath + "/server_cert.crt"
	caCrtPath     = tmpCrtPath + "/ca_cert.pem"
	sslCrtPath    = "/etc/ssl/certs"
)

type httpserver struct {
	// port is the port that the HTTP server will listen and serve on.
	port string
	// serveTLS is true if the server is using HTTPS. If false, the server is using HTTP.
	serveTLS bool
	handle   func(rw http.ResponseWriter, req *http.Request)
	server   *http.Server
	env      *env.Env
}

// Handler creates the object to handle the response for the HTTP server.
type Handler struct {
	handle func(rw http.ResponseWriter, req *http.Request)
}

// ServeHTTP will have the HTTP server respond to requests with |handle|.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.handle(rw, req)
}

// New creates a new httpserver object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object. The
// httpserver will only respond with |handle|. |port| will be the port the
// HTTP server listens and serves on. |serveTLS| is set to true if the server
// serves HTTPS. If false, the server will serve HTTP.
func New(port string, handle func(rw http.ResponseWriter, req *http.Request), serveTLS bool) *httpserver {
	return &httpserver{port: port, handle: handle, serveTLS: serveTLS}
}

// Start starts the HTTP server in a separate process. The HTTP server listens on
// any IPv4 and IPv6 address within the namespace. If |serveTLS| is true, the server
// will server HTTPS.
func (h *httpserver) Start(ctx context.Context, env *env.Env) (retErr error) {
	h.env = env
	handler := &Handler{handle: h.handle}
	h.server = &http.Server{Addr: fmt.Sprintf(":%v", h.port), Handler: handler}

	errChannel := make(chan error)
	go func() {
		cleanup, err := h.env.EnterNetNS(ctx)
		if err != nil {
			errChannel <- errors.Wrapf(err, "failed to enter the associated netns %s", h.env.NetNSName)
			return
		}
		defer cleanup()
		ln, err := net.Listen("tcp", h.server.Addr)
		if err != nil {
			errChannel <- err
			return
		}
		if h.serveTLS {
			if err := installCerts(ctx, certificate.TestCert3()); err != nil {
				uninstallCerts(ctx)
				errChannel <- err
				return
			}
			defer uninstallCerts(ctx)
			errChannel <- nil
			if err := h.server.ServeTLS(ln, serverCrtPath, serverKeyPath); err != http.ErrServerClosed {
				testing.ContextLogf(ctx, "ServeTLS failed to start with err: %q", err)
			}
		} else {
			errChannel <- nil
			h.server.Serve(ln)
		}
	}()

	return <-errChannel
}

// Stop terminates the process running the HTTP server.
func (h *httpserver) Stop(ctx context.Context) error {
	h.server.Shutdown(ctx)
	return nil
}

// WriteLogs writes logs into |f|.
func (h *httpserver) WriteLogs(ctx context.Context, f *os.File) error {
	return h.env.ReadAndWriteLogIfExists(h.env.ChrootPath(logPath), f)
}

// installCerts installs and writes certs for HTTPS server. In order to use symlinks
// in tmp, tmp must be remounted to allow symlinks. uninstallCerts() will restore
// tmp directory to previous state
func installCerts(ctx context.Context, certs certificate.CertStore) error {
	if err := testexec.CommandContext(ctx, "mount", "-F", "tmpfs", "-o", "remount,noexec,nosuid,nodev", "/tmp").Run(); err != nil {
		return errors.Wrap(err, "failed change mount options on tmp to allow symlinks")
	}

	testing.ContextLog(ctx, "Installing CA certs to tmp directory for TLS validation")
	if err := installSSLCerts(ctx, certs.CACred.Cert); err != nil {
		return errors.Wrap(err, "failed to set up temp CA certs")
	}

	testing.ContextLog(ctx, "Writing server certs to tmp directory")
	if err := writeServerCerts(ctx, certs.ServerCred); err != nil {
		return errors.Wrap(err, "failed to write temp server certs")
	}
	return nil
}

// writeServerCerts writes the server certificate and the server private key to a
// tmp directory for use in ServeTLS().
func writeServerCerts(ctx context.Context, serverCreds certificate.Credential) error {
	if err := os.WriteFile(serverCrtPath, []byte(serverCreds.Cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write server cert")
	}
	if err := os.WriteFile(serverKeyPath, []byte(serverCreds.PrivateKey), 0644); err != nil {
		return errors.Wrap(err, "failed to write server private key")
	}
	return nil
}

// installSSLCerts writes the ca |cert| to a tmp directory, rehashes the directory,
// and then mounts the tmp directory to '/etc/ssl/certs'. After this is called, the
// default certs in '/etc/ssl/certs' are hidden. uninstallCerts() will remove the mount
// and make the default certs visible again.
func installSSLCerts(ctx context.Context, cert string) error {
	if err := testexec.CommandContext(ctx, "mkdir", tmpCrtPath).Run(); err != nil {
		return errors.Wrap(err, "failed to make tmp directory")
	}

	if err := os.WriteFile(caCrtPath, []byte(cert), 0644); err != nil {
		return errors.Wrap(err, "failed to write ca cert")
	}

	if err := testexec.CommandContext(ctx, "c_rehash", tmpCrtPath).Run(); err != nil {
		return errors.Wrap(err, "failed to rehash tmp cert directory")
	}

	if err := testexec.CommandContext(ctx, "mount", "-o", "bind", tmpCrtPath, sslCrtPath).Run(); err != nil {
		return errors.Wrap(err, "failed to bind mount")
	}
	return nil
}

// uninstallCerts unmounts '/etc/ssl/certs' and deletes the tmp directory that contained
// the CA cert, server cert, and server private key.
func uninstallCerts(ctx context.Context) {
	if err := testexec.CommandContext(ctx, "umount", sslCrtPath).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to unmount bind: ", err)
	}
	if err := testexec.CommandContext(ctx, "rm", "-rf", tmpCrtPath).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to delete tmp directory: ", err)
	}
	if err := testexec.CommandContext(ctx, "mount", "-F", "tmpfs", "-o", "remount,noexec,nosuid,nodev,nosymfollow", "/tmp").Run(); err != nil {
		testing.ContextLog(ctx, "Failed change mount options on tmp back to default by not allowing symlinks: ", err)
	}
}
