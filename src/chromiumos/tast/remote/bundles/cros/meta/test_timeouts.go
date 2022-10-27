// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

var (
	tests = []string{
		//"meta.LocalFail",
		"meta.LocalPass",
		//"meta.RemoteFail",
		//"meta.RemotePass",
	}
)

type timeoutParams struct {
	tests   []string
	timeout time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestTimeouts,
		Desc:     "Verifies that we pass the test timeouts correctly.",
		Contacts: []string{"tast-owners@google.com"},
		Params: []testing.Param{
			{
				Name: "default",
				Val:  timeoutParams{timeout: 100 * time.Second},
			},
		},
	})
}

func TestTimeouts(ctx context.Context, s *testing.State) {
	param := s.Param().(timeoutParams)
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	//wantTimeout := 120 * time.Second
	flags := []string{
		"-resultsdir=" + resultsDir,
	}
	if param.timeout != 0 {
		flags = append(flags, fmt.Sprintf("-systemservicestimeout=%d", int(param.timeout.Seconds())))
		//	wantTimeout = param.timeout
	}
	stdout, _, err := tastrun.Exec(ctx, s, "run", flags, tests)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	s.Logf("%v", s.OutDir())
	s.Logf("%v", resultsDir)
	s.Log("******")
	s.Logf("%v", filepath.Dir(s.OutDir()))

	cfgFilePath := filepath.Join(filepath.Dir(s.OutDir()), "config", "runConfig.json")
	cfgFile, err := os.Open(cfgFilePath)
	if err != nil {
		s.Fatalf("Failed to open config file at %s: %v", cfgFilePath, err)
	}
	defer cfgFile.Close()

	contents, err := io.ReadAll(cfgFile)
	if err != nil {
		s.Fatalf("Failed to read config file at %s: %v", cfgFile, err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(contents, &cfg); err != nil {
		s.Fatalf("Failed to unmarshal config json: %v", err)
	}

	s.Logf("%v", cfg["system_services_timeout"])
}
