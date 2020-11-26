// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/cuj"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const tempDirCreateTimeout = 1 * time.Minute

var localStore = &localStoreImpl{name: "local_store", timeout: tempDirCreateTimeout, cdstmp: make(map[string]*dutTmpPath)}

// LocalStore returns a precondition of local storage for test case.
func LocalStore() testing.Precondition {
	return localStore
}

// LocalStoreData is the struct returned by the preconditions.
type LocalStoreData struct {
	// DUTTempDir returns temporary directory path from from target DUT.
	DUTTempDir string
	// CompDUTsTempDir return temporary directory path from companion DUTs by hostnmae.
	CompDUTsTempDir map[string]string
}

type dutTmpPath struct {
	client *rpc.Client
	path   string
}

type localStoreImpl struct {
	name    string
	timeout time.Duration
	clients []*rpc.Client
	// dtmp saves created temporary directory path on target DUT.
	dtmp *dutTmpPath
	// cdstmp saves created temporary directory path on companion DUTs by hostname.
	cdstmp map[string]*dutTmpPath
}

// String identifies this Precondition.
func (p *localStoreImpl) String() string {
	return p.name
}

// Timeout is the max time needed to prepare this Precondition.
func (p *localStoreImpl) Timeout() time.Duration {
	return p.timeout
}

// Prepare creates a temporary directionary on each connected DUT (target, companion),
// when the case running, it could get the path and save files in there.
func (p *localStoreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	m := make(map[string]string)
	d := s.DUT()

	// Connect remote DUT, call local store gRPC service to create a temporary directory
	// to store logs, screenshots or any binary data.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Logf("Failed to connect to the RPC service on the DUT: %s: %v", d.HostName(), err)
		return nil
	}

	cr := cuj.NewLocalStoreServiceClient(cl.Conn)
	res, err := cr.Create(ctx, &empty.Empty{})
	if err != nil {
		s.Log("Failed to create tempdir: ", err)
		return nil
	}

	p.dtmp = &dutTmpPath{client: cl, path: res.Path}
	mduts := s.CompDUTs()

	var dwg, dswg sync.WaitGroup
	var mux sync.Mutex
	for _, duts := range mduts {
		dswg.Add(1)
		go func(ds []*dut.DUT) {
			defer dswg.Done()
			dwg.Add(1)
			for _, d := range duts {
				go func(d *dut.DUT) {
					defer dwg.Done()
					cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
					if err != nil {
						s.Logf("Failed to connect to the RPC service on the DUT: %s: %v", d.HostName(), err)
						return
					}

					cr := cuj.NewLocalStoreServiceClient(cl.Conn)
					res, err := cr.Create(ctx, &empty.Empty{})
					if err != nil {
						s.Log("Failed to create tempdir: ", err)
					}

					mux.Lock()
					defer mux.Unlock()
					p.cdstmp[d.HostName()] = &dutTmpPath{client: cl, path: res.Path}
					m[d.HostName()] = res.Path
				}(d)
			}
		}(duts)
	}
	dswg.Wait()
	dwg.Wait()

	return &LocalStoreData{DUTTempDir: p.dtmp.path, CompDUTsTempDir: m}
}

// Close collects files from specified temporary directory on each DUT
func (p *localStoreImpl) Close(ctx context.Context, s *testing.PreState) {
	const localStorePrefix = "local-store-"
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	d := s.DUT()
	mduts := s.CompDUTs()

	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Log("Failed to get name of output directory")
		return
	}

	dst := filepath.Join(dir, localStorePrefix+"main")
	if err := linuxssh.GetFile(ctx, d.Conn(), p.dtmp.path, dst); err != nil {
		s.Logf("Failed to download %v from DUT to %v at local host: %v", p.dtmp.path, dst, err)
	}

	var dwg, dswg sync.WaitGroup
	for _, duts := range mduts {
		dswg.Add(1)
		go func(duts []*dut.DUT) {
			defer dswg.Done()
			for _, d := range duts {
				dwg.Add(1)
				go func(d *dut.DUT) {
					defer dwg.Done()
					host := d.HostName()
					if _, ok := p.cdstmp[host]; !ok {
						s.Log("Failed to get local storage tmpdir from DUT: ", host)
						return
					}
					dst := filepath.Join(dir, localStorePrefix+host)
					if err := linuxssh.GetFile(ctx, d.Conn(), p.cdstmp[host].path, dst); err != nil {
						s.Logf("Failed to download %v from DUT to %v at local host: %v", p.cdstmp[host].path, dst, err)
					}
				}(d)
			}
		}(duts)
	}
	dswg.Wait()
	dwg.Wait()
}
