// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

// getRepoInfo returns path's location relative to its git repository and the HEAD revision for the repository.
func getRepoInfo(path string) (relPath, rev string, err error) {
	if path, err = filepath.Abs(path); err != nil {
		return "", "", err
	}
	path = filepath.Clean(path)

	// This prints the base path of the repo on the first line and HEAD's revision on the second.
	cmd := exec.Command("git", "rev-parse", "--show-toplevel", "HEAD")
	cmd.Dir = filepath.Dir(path)
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", "", errors.Errorf("%q printed %q; wanted 2 lines", strings.Join(cmd.Args, " "), string(out))
	}
	rev = lines[1]
	if relPath, err = filepath.Rel(lines[0], path); err != nil {
		return "", "", err
	}
	return relPath, rev, nil
}

// writeTemplate executes the template and writes its output to path.
// tmplStr is the template to get executed, and data is the template's data.
func writeTemplate(tmplStr string, data interface{}, path string) error {
	f, err := ioutil.TempFile(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		f.Close()
		os.Remove(f.Name())
	}()

	if err = template.Must(template.New("header").Parse(tmplStr)).Execute(f, data); err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}

	return nil
}
