// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"go.chromium.org/chromiumos/config/go/api/test/tls"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Serializer,
		Desc: "Basic test to check that minidump crashes are serialized",
		Contacts: []string{
			"mutexlox@chromium.org",
			"cros-telemetry@google.com",
		},
		Params: []testing.Param{{
			Name: "",
			Val:  false,
		}, {
			Name: "fetch_core",
			Val:  true,
		}},
		Attr: []string{"group:mainline"},
	})
}

// verifyMetaField verifies that the given meta field is found in the crash's CrashMetadata
func verifyMetaField(ctx context.Context, key, expected string, crash *tls.CrashInfo) bool {
	for _, kv := range crash.Fields {
		if kv.Key == key {
			if expected != kv.Text {
				testing.ContextLogf(ctx, "Unexpected value for %s. Want %s, got %s", key, expected, kv.Text)
				return false
			}
			return true
		}
	}
	testing.ContextLogf(ctx, "%s was unexpectedly not present (want %s)", key, expected)
	return false
}

func Serializer(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes(crash.FilterInIgnoreAllCrashes), crash.WithMockConsent()); err != nil {
		s.Fatal("Setup failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	const (
		basename = "some_program.1.2.3.4"
		// Coredump should be large enough to need to split into multiple messages
		coreBytes  = 3 * 1024 * 1024
		coreChunks = 3
	)
	exp, err := crash.AddFakeMinidumpCrash(ctx, basename)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}
	// Explicitly clean up created files, since the serializer won't.
	defer func() {
		if err := os.Remove(exp.MetadataPath); err != nil && !os.IsNotExist(err) {
			s.Errorf("Failed to remove %s: %s", exp.MetadataPath, err)
		}
		if err := os.Remove(exp.PayloadPath); err != nil && !os.IsNotExist(err) {
			s.Errorf("Failed to remove %s: %s", exp.PayloadPath, err)
		}
	}()

	coreName := filepath.Join(crash.SystemCrashDir, basename+".core")
	addCore := s.Param().(bool)
	if addCore {
		if err := crash.CreateRandomFile(coreName, coreBytes); err != nil {
			s.Fatal("Failed to add a coredump: ", err)
		}
		defer func() {
			if err := os.Remove(coreName); err != nil && !os.IsNotExist(err) {
				s.Errorf("Failed to remove %s: %s", coreName, err)
			}
		}()
	}

	s.Log("Running crash_serializer")
	var args []string
	if addCore {
		args = append(args, "--fetch_coredumps")
	}
	cmd := testexec.CommandContext(ctx, "/usr/local/sbin/crash_serializer", args...)
	out, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to run crash_serializer: ", err)
	}
	// Else, output will be the serialized protos.
	var protos []*tls.FetchCrashesResponse
	for len(out) != 0 {
		size := binary.BigEndian.Uint64(out[0:8])
		out = out[8:]
		next := &tls.FetchCrashesResponse{}
		if err := proto.Unmarshal(out[:size], next); err != nil {
			s.Fatal("Failed to unmarshal proto: ", err)
		}
		out = out[size:]
		protos = append(protos, next)
	}

	expectedLen := 2
	if addCore {
		expectedLen += coreChunks
	}

	if len(protos) != expectedLen {
		s.Fatalf("Unexpected number of protos. Want %d, got %d ", expectedLen, len(protos))
	}

	crashID := protos[0].CrashId

	switch x := protos[0].Data.(type) {
	case *tls.FetchCrashesResponse_Crash:
		if exp.Version != x.Crash.Ver {
			s.Errorf("Unexpected version. Want %s, got %s", exp.Version, x.Crash.Ver)
		}

		if exp.Product != x.Crash.Prod {
			s.Errorf("Unexpected product. Want %s, got %s", exp.Product, x.Crash.Prod)
		}

		if !verifyMetaField(ctx, "board", exp.Board, x.Crash) {
			s.Error("Failed to verify board")
		}

		if !verifyMetaField(ctx, "hwclass", exp.HWClass, x.Crash) {
			s.Error("Failed to verify hwclass")
		}

		if exp.Executable != x.Crash.ExecName {
			s.Errorf("Unexpected exec name. Want %s, got %s", exp.Executable, x.Crash.ExecName)
		}
	default:
		s.Errorf("Unexpected oneof type for protos[0]: %T", x)
	}

	if protos[1].CrashId != crashID {
		s.Errorf("CrashID unexpectedly changed from %d to %d", crashID, protos[1].CrashId)
	}

	switch x := protos[1].Data.(type) {
	case *tls.FetchCrashesResponse_Blob:
		basePayload := filepath.Base(exp.PayloadPath)
		if basePayload != x.Blob.Filename {
			s.Errorf("Unexpected filename. Want %s, got %s", basePayload, x.Blob.Filename)
		}
		contents, err := ioutil.ReadFile(exp.PayloadPath)
		if err != nil {
			s.Fatal("Failed to read payload file: ", err)
		}
		if !bytes.Equal(contents, x.Blob.Blob) {
			s.Error("Unexpected blob. Saving actual and expected")
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), exp.PayloadPath); err != nil {
				s.Error("Failed to save expected payload: ", err)
			}
			if err := ioutil.WriteFile(filepath.Join(s.OutDir(), basePayload+".actual"), x.Blob.Blob, 0664); err != nil {
				s.Error("Failed to save actual payload: ", err)
			}

		}
	default:
		s.Errorf("Unexpected oneof type for protos[1]: %T", x)
	}

	if addCore {
		var core []byte
		for i := 2; i < expectedLen; i++ {
			if protos[i].CrashId != crashID {
				s.Errorf("index %d: CrashID unexpectedly changed from %d to %d", i, crashID, protos[i].CrashId)
			}
			switch x := protos[i].Data.(type) {
			case *tls.FetchCrashesResponse_Core:
				core = append(core, x.Core...)
			default:
				s.Errorf("Unexpected oneof type for protos[%d]: %T", i, x)
			}
		}
		contents, err := ioutil.ReadFile(coreName)
		if err != nil {
			s.Fatal("Failed to read core file: ", err)
		}
		if !bytes.Equal(contents, core) {
			s.Error("Unexpected core contents. Saving actual and expected")
			if err := crash.MoveFilesToOut(ctx, s.OutDir(), coreName); err != nil {
				s.Error("Failed to save expected core: ", err)
			}
			if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filepath.Base(coreName+".actual")), core, 0664); err != nil {
				s.Error("Failed to save actual core: ", err)
			}
		}
	}
}
