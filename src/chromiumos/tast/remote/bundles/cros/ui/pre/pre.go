// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/cuj"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const tempDirCreateTimeout = 1 * time.Minute

// localStore is the struct returned by the preconditions.
var localStore = &localStoreImpl{name: "local_store", timeout: tempDirCreateTimeout, cdstmp: make(map[string]*dutTmpPath)}

// LocalStore returns a precondition of local storage for test case.
// Creates temporary directory on target DUTs, and remote cases may want to save
// logs, screenshots or binaries after manipulating via gRPC call. When case done,
// the Close method would pull those directory from each DUT.
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
	p.clients = append(p.clients, cl)

	cr := cuj.NewLocalStoreServiceClient(cl.Conn)
	res, err := cr.Create(ctx, &empty.Empty{})
	if err != nil {
		s.Log("Failed to create tempdir: ", err)
		return nil
	}

	p.dtmp = &dutTmpPath{client: cl, path: res.Path}

	var g errgroup.Group
	var mux sync.Mutex
	for _, duts := range s.CompDUTs() {
		for i := range duts {
			d := duts[i]
			g.Go(func() error {
				cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
				if err != nil {
					return errors.Wrapf(err, "failed to connect to gRPC service on the DUT: %s", d.HostName())
				}

				cr := cuj.NewLocalStoreServiceClient(cl.Conn)
				res, err := cr.Create(ctx, &empty.Empty{})
				if err != nil {
					return errors.Wrapf(err, "failed to create tempdir on the DUT: %s", d.HostName())
				}

				mux.Lock()
				defer mux.Unlock()
				p.clients = append(p.clients, cl)
				p.cdstmp[d.HostName()] = &dutTmpPath{client: cl, path: res.Path}
				m[d.HostName()] = res.Path

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		s.Fatal("Failed to create tempdir on DUTs: ", err)
	}

	return &LocalStoreData{DUTTempDir: p.dtmp.path, CompDUTsTempDir: m}
}

// Close collects files from specified temporary directory on each DUT
func (p *localStoreImpl) Close(ctx context.Context, s *testing.PreState) {
	const localStorePrefix = "local-store-"
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	d := s.DUT()

	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Log("Failed to get name of output directory")
		return
	}

	dst := filepath.Join(dir, localStorePrefix+"main")
	if err := linuxssh.GetFile(ctx, d.Conn(), p.dtmp.path, dst); err != nil {
		s.Fatalf("Failed to download %v from DUT to %v at local host: %v", p.dtmp.path, dst, err)
	}

	var g errgroup.Group
	for _, duts := range s.CompDUTs() {
		for i := range duts {
			d := duts[i]
			g.Go(func() error {
				host := d.HostName()
				if _, ok := p.cdstmp[host]; !ok {
					return errors.Wrapf(nil, "failed to get local store tempdir from DUT: %s", host)
				}
				dst := filepath.Join(dir, localStorePrefix+host)
				if err := linuxssh.GetFile(ctx, d.Conn(), p.cdstmp[host].path, dst); err != nil {
					return errors.Wrapf(err, "failed to download %v from DUT to %v at local host", p.cdstmp[host].path, dst)
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		s.Fatal("Failed to collect log from DUTs: ", err)
	}

	for _, cl := range p.clients {
		cr := cuj.NewLocalStoreServiceClient(cl.Conn)
		if _, err := cr.Remove(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to create tempdir: ", err)
		}
	}
}
