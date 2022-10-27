// Copyright 2022 The ChromiumOS Authors
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

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

var (
	tests = []string{
		"meta.LocalPass",
		"meta.RemotePass",
	}
)

type timeoutParams struct {
	tests      []string
	timeout    int
	saveConfig bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     TestTimeouts,
		Desc:     "Verifies that we pass the system service timeout correctly",
		Contacts: []string{"tast-owners@google.com"},
		Params: []testing.Param{
			{
				Name: "defaultsave",
				Val:  timeoutParams{saveConfig: true},
			},
			{
				Name: "defaultnosave",
				Val:  timeoutParams{saveConfig: false},
			},
			{
				Name: "customsave",
				Val:  timeoutParams{timeout: 100, saveConfig: true},
			},
			{
				Name: "customnosave",
				Val:  timeoutParams{timeout: 100, saveConfig: false},
			},
		},
	})
}

func TestTimeouts(ctx context.Context, s *testing.State) {
	param := s.Param().(timeoutParams)
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	wantTimeout := 120
	flags := []string{
		"-resultsdir=" + resultsDir,
	}
	if param.saveConfig {
		flags = append(flags, fmt.Sprintf("-saveruntimeconfig=true"))
	}

	if param.timeout != 0 {
		flags = append(flags, fmt.Sprintf("-systemservicestimeout=%d", param.timeout))
		wantTimeout = param.timeout
	}

	stdout, _, err := tastrun.Exec(ctx, s, "run", flags, tests)
	if err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Fatalf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	var gotFile bool
	err = filepath.Walk(resultsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// We are only looking for the config file.
		if info.Name() != "runConfig.json" {
			return nil
		}

		gotFile = true
		cfgFile, err := os.Open(path)
		if err != nil {
			s.Fatalf("Failed to open config file at %s: %v", path, err)
		}
		defer cfgFile.Close()

		contents, err := io.ReadAll(cfgFile)
		if err != nil {
			s.Fatalf("Failed to read config file at %s: %v", cfgFile, err)
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(contents, &cfg); err != nil {
			s.Fatal("Failed to unmarshal config json: ", err)
		}
		// The RunConfig struct is internal and not accessible to tests.
		// Treating this as a regular json instead.
		gotTimeout := int(cfg["system_services_timeout"].(map[string]interface{})["seconds"].(float64))
		if wantTimeout != gotTimeout {
			s.Fatalf("SystemServiceTimeout not as expected. got %d want %d", gotTimeout, wantTimeout)
		}

		return nil
	})

	if err != nil {
		s.Fatal("Error looking for config: ", err)
	}

	if param.saveConfig && !gotFile {
		s.Fatal("Did not fing config file under ", resultsDir)
	}
}
