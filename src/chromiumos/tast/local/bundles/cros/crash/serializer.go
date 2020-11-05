// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"bytes"
	"context"
	"encoding/binary"
	"io/ioutil"
	"path"

	"github.com/golang/protobuf/proto"
	"go.chromium.org/chromiumos/config/go/api/test/tls"

	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
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
		Attr: []string{"group:mainline", "informational"},
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

	const basename = "some_program.1.2.3"
	exp, err := crash.AddFakeMinidumpCrash(ctx, basename)
	if err != nil {
		s.Fatal("Failed to add a fake minidump crash: ", err)
	}

	s.Log("Running crash_serializer")
	cmd := testexec.CommandContext(ctx, "/usr/local/sbin/crash_serializer")
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

	if len(protos) != 2 {
		s.Fatal("Unexpected number of protos. Want 2, got ", len(protos))
	}

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

	switch x := protos[1].Data.(type) {
	case *tls.FetchCrashesResponse_Blob:
		basePayload := path.Base(exp.PayloadPath)
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
			if err := ioutil.WriteFile(path.Join(s.OutDir(), basePayload+".actual"), x.Blob.Blob, 0755); err != nil {
				s.Error("Failed to save actual payload: ", err)
			}

		}
	default:
		s.Errorf("Unexpected oneof type for protos[1]: %T", x)
	}
}
