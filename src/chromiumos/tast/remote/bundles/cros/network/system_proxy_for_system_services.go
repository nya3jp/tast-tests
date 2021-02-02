// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemProxyForSystemServices,
		Desc: "Test that tlsdated can successfully connect to the remote host through the system-proxy daemon",
		Contacts: []string{
			"acostinas@google.com",
			"hugobenichi@chromium.org",
			"omorsi@chromium.org",
			"pmarko@chromium.org",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.network.ProxyService"},
		Timeout:      12 * time.Minute,
	})
}

func SystemProxyForSystemServices(ctx context.Context, s *testing.State) {
	// Runs tlsdate once, in the foreground. Returns an error if tlsdate didn't use system-proxy to connect to the remote host
	// or if the certificate verification failed.
	runTLSDate := func() error {
		// tlsdated is a CrOS daemon that runs the tlsdate binary periodically in the background and does proxy resolution through Chrome.
		// Prepend `timeout` to the command that runs tlsdated to send SIGTERM to the daemon after the first invocation (otherwise tlsdated
		// will run in the foreground until the connection times out).
		// The `-m <n>` option means tlsdate should run at most once every n seconds in steady state
		// The `-p` option means dry run.
		// The result will be written in a temp file. Errors are ignored because `timeout` will generate an error even if tlsdated terminates
		// gracefully.
		s.DUT().Conn().Command("sh", "-c", "timeout 20 /usr/bin/tlsdated -p -m 60 -- /usr/bin/tlsdate -v -C /usr/share/chromeos-ca-certificates -l >/tmp/result.txt 2>&1").Run(ctx)

		out, err := s.DUT().Conn().Command("cat", "/tmp/result.txt").Output(ctx)
		if err != nil {
			return errors.Wrap(err, "error reading the result file")
		}
		var result = string(out)
		// system-proxy has an address in the 100.115.92.0/24 subnet (assigned by patchpanel) and listens on port 3128.
		proxyMsg := regexp.MustCompile("V: using proxy http://100.115.92.[0-9]+:3128")
		const successMsg = "V: certificate verification passed"

		if !proxyMsg.Match([]byte(result)) {
			return errors.Errorf("tlsdated is not using the system-proxy daemon: %s", result)
		}

		if !strings.Contains(result, successMsg) {
			return errors.New("certificate verification failed: " + result)
		}

		return nil
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	// Start an HTTP proxy instance on the DUT which requires username and password authentication.
	proxyClient := network.NewProxyServiceClient(cl.Conn)
	response, err := proxyClient.StartServer(ctx,
		&network.StartServerRequest{
			Port:            3129,
			AuthCredentials: &network.AuthCredentials{Username: "andreea", Password: "test"},
		})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	pb := fakedms.NewPolicyBlob()
	// Configure the proxy on the DUT via policy to point to the local proxy instance started via the `ProxyService`.
	pb.AddPolicy(&policy.ProxySettings{
		Val: &policy.ProxySettingsValue{
			ProxyMode:       "fixed_servers",
			ProxyServer:     fmt.Sprintf("http://%s", response.HostAndPort),
			ProxyBypassList: "",
		}})
	// Start system-proxy and configure it with the credentials of the local proxy instance.
	pb.AddPolicy(&policy.SystemProxySettings{
		Val: &policy.SystemProxySettingsValue{
			SystemProxyEnabled:           true,
			SystemServicesUsername:       "andreea",
			SystemServicesPassword:       "test",
			PolicyCredentialsAuthSchemes: []string{},
		}})

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if err = runTLSDate(); err != nil {
		s.Fatal("Failed to run tlsdate with system-proxy: ", err)
	}
}
