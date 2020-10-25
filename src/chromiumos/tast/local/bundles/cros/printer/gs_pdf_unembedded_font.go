// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/kylelemons/godebug/diff"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GsPDFUnembeddedFont,
		Desc:         "Tests that ghostscript handles unembedded PDF fonts",
		Contacts:     []string{"batrapranav@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cups"},
		Data:         []string{fontFile, fontGoldenFile},
	})
}

const (
	// Adapted from https://jira.atlassian.com/browse/CONFSERVER-58258
	fontFile       = "font-test.pdf"
	fontGoldenFile = "font-golden.pdf"
)

func cleanPDFContents(s string) string {
	r := regexp.MustCompile("(?m)^.*(/ID|Date|DocumentID).*$")
	return r.ReplaceAllLiteralString(s, "")
}

func GsPDFUnembeddedFont(ctx context.Context, s *testing.State) {
	outFilePath := s.OutDir() + "/" + fontGoldenFile
	cmd := testexec.CommandContext(ctx, "gs", "-q", "-sDEVICE=pdfwrite", "-o", outFilePath, s.DataPath(fontFile))
	if out, err := cmd.CombinedOutput(); len(out) > 0 || err != nil {
		cmd.DumpLog(ctx)
		s.Log(string(out))
		s.Fatal()
	}
	got, err := ioutil.ReadFile(outFilePath)
	if err != nil {
		s.Fatal("Failed to read output file: ", err)
	}
	expect, err := ioutil.ReadFile(s.DataPath(fontGoldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}
	if diff := diff.Diff(cleanPDFContents(string(expect)), cleanPDFContents(string(got))); diff != "" {
		const diffFile = "diff"
		if err := ioutil.WriteFile(s.OutDir()+"/"+diffFile, []byte(diff), 0644); err != nil {
			s.Error("Failed to dump diff: ", err)
		}
		s.Fatalf("Gs output differs from expected: diff saved to %q (-want +got), output to %q", diffFile, fontGoldenFile)
	}
	if err := os.Remove(outFilePath); err != nil {
		s.Fatal("Failed to delete output file: ", err)
	}
}
