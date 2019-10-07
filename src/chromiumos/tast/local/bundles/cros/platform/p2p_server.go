// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/mdns"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/p2p"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     P2PServer,
		Desc:     "Tests that Chromium OS can serve files to local network peers with p2p-server",
		Contacts: []string{"nya@chromium.org"},
	})
}

// queryP2PServices queries P2P services available on the virtual network.
func queryP2PServices(ctx context.Context, timeout time.Duration) ([]*mdns.ServiceEntry, error) {
	if dl, ok := ctx.Deadline(); ok {
		ctxTimeout := dl.Sub(time.Now())
		if ctxTimeout < timeout {
			timeout = ctxTimeout
		}
	}

	ch := make(chan *mdns.ServiceEntry)
	var err error
	go func() {
		defer close(ch)
		params := &mdns.QueryParam{
			Timeout: timeout,
			Domain:  "local",
			Service: p2p.ServiceType,
			Entries: ch,
		}
		err = mdns.Query(params)
	}()

	var srvs []*mdns.ServiceEntry
	for srv := range ch {
		if srv.Addr.String() == p2p.IsolatedNSIP {
			srvs = append(srvs, srv)
		}
	}
	return srvs, err
}

// waitP2PService waits for a P2P service on the virtual network to be ready.
// It is an error if there are more than a single P2P service.
func waitP2PService(ctx context.Context) (*mdns.ServiceEntry, error) {
	var srvs []*mdns.ServiceEntry

	const maxWait = 5 * time.Second
	wait := 500 * time.Millisecond
	for len(srvs) == 0 {
		var err error
		srvs, err = queryP2PServices(ctx, wait)
		if err != nil {
			return nil, err
		}
		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}

	if len(srvs) > 1 {
		return nil, errors.New("multiple services found")
	}
	return srvs[0], nil
}

func generateRandomBytes(size int) ([]byte, error) {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make([]byte, size)
	if _, err := io.ReadFull(f, out); err != nil {
		return nil, err
	}
	return out, nil
}

func P2PServer(fullCtx context.Context, s *testing.State) {
	// Shorten the timeout to allow some time for cleanup.
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if err := p2p.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up: ", err)
	}
	defer func() {
		if err := p2p.CleanUp(fullCtx); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	if err := upstart.EnsureJobRunning(ctx, "p2p"); err != nil {
		s.Fatal("Failed to start p2p: ", err)
	}

	// Find the P2P service and check that the announced information is correct.
	s.Log("Discovering P2P service")
	srv, err := waitP2PService(ctx)
	if err != nil {
		s.Fatal("Failed to find P2P service: ", err)
	}

	s.Logf("P2P service found at %s:%d; %s", srv.Addr.String(), srv.Port, srv.Info)

	if srv.Port != p2p.ServicePort {
		s.Errorf("Service port is %d; want %d", srv.Port, p2p.ServicePort)
	}

	// Share a file and check that it is advertised.
	s.Log("Testing a new file is advertised")

	const (
		testFileBase = "somefile"
		testFileName = "somefile.p2p"
		testFileSize = 123456
		advertisedID = "id_somefile=123456"
	)
	rand, err := generateRandomBytes(testFileSize)
	if err != nil {
		s.Fatal("Failed to generate a random file: ", err)
	}
	if err := ioutil.WriteFile(filepath.Join(p2p.SharedDir, testFileName), rand, 0666); err != nil {
		s.Fatalf("Failed to save %s: %v", testFileName, err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		srv, err = waitP2PService(ctx)
		if err != nil {
			return err
		}
		for _, txt := range srv.InfoFields {
			if txt == advertisedID {
				return nil
			}
		}
		return errors.Errorf("file not advertised; info=%s", srv.Info)
	}, nil); err != nil {
		s.Fatal("Failed to wait for advertisement: ", err)
	}

	s.Logf("P2P service at %s:%d; %s", srv.Addr.String(), srv.Port, srv.Info)

	// Attempt to download the file, but we can't download it locally (crbug.com/309708).
	s.Log("Testing that local download is blocked")
	for _, host := range []string{p2p.DefaultNSIP, "127.0.0.1"} {
		url := fmt.Sprintf("http://%s:%d/%s", host, srv.Port, testFileBase)
		// We use curl here instead of net/http to align with the later test.
		cmd := testexec.CommandContext(ctx, "curl", url)
		err := cmd.Run()
		// curl's exit code 7: Failed to connect to host.
		if st, ok := testexec.GetWaitStatus(err); !ok {
			s.Errorf("curl %s failed: %v", url, err)
		} else if st.ExitStatus() != 7 {
			s.Errorf("curl %s exited with status %d; want 7 (failed to connect to host)", url, st.ExitStatus())
		}
	}

	// Download succeeds from remote.
	s.Log("Testing that remote download succeeds")
	url := fmt.Sprintf("http://%s:%d/%s", p2p.DefaultNSIP, srv.Port, testFileBase)
	cmd := testexec.CommandContext(ctx, "ip", "netns", "exec", p2p.NSName, "curl", url)
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("curl failed: ", err)
	} else if !bytes.Equal(out, rand) {
		s.Error("Served file is corrupted")
	}
}
