// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestPPDs,
		Desc: "Verifies the PPD files pass cupstestppd and foomatic-rip",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		// This may take a while on zork boards.
		Timeout:      time.Minute * 5,
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{ppdsAll},
	})
}

// Files to skip that currently fail the test.
var skip = map[string]bool{
	"epson-20200615-EPSON_USB1.1_MFP_Full-Speed.ppd.gz":                                      true,
	"epson-20200615-EPSON_USB2.0_MFP_Hi-Speed.ppd.gz":                                        true,
	"epson-20200615-EPSON_USB2.0_Printer_Hi-speed.ppd.gz":                                    true,
	"foomatic-20200219-Apple-Color_StyleWriter_1500-lpstyl.ppd.gz":                           true,
	"foomatic-20200219-Apple-Color_StyleWriter_2200-lpstyl.ppd.gz":                           true,
	"foomatic-20200219-Apple-Color_StyleWriter_2400-lpstyl.ppd.gz":                           true,
	"foomatic-20200219-Apple-Color_StyleWriter_2500-lpstyl.ppd.gz":                           true,
	"foomatic-20200219-Apple-StyleWriter_1200-lpstyl.ppd.gz":                                 true,
	"foomatic-20200219-Apple-StyleWriter_I-lpstyl.ppd.gz":                                    true,
	"foomatic-20200219-Apple-StyleWriter_II-lpstyl.ppd.gz":                                   true,
	"foomatic-20200219-Brother-HL-1020-hl7x0.ppd.gz":                                         true,
	"foomatic-20200219-Brother-HL-720-hl7x0.ppd.gz":                                          true,
	"foomatic-20200219-Brother-HL-730-hl7x0.ppd.gz":                                          true,
	"foomatic-20200219-Brother-HL-820-hl7x0.ppd.gz":                                          true,
	"foomatic-20200219-Brother-MFC-9050-hl7x0.ppd.gz":                                        true,
	"foomatic-20200219-Compaq-IJ1200-drv_z42.ppd.gz":                                         true,
	"foomatic-20200219-Dell-3010cn-pxldpl.ppd.gz":                                            true,
	"foomatic-20200219-Generic-PCL_3_Printer-pcl3.ppd.gz":                                    true,
	"foomatic-20200219-HP-DesignJet_1050C-Postscript-HP.ppd.gz":                              true,
	"foomatic-20200219-HP-DesignJet_1055CM-Postscript-HP.ppd.gz":                             true,
	"foomatic-20200219-HP-DesignJet_2500CP-Postscript-HP.ppd.gz":                             true,
	"foomatic-20200219-HP-DesignJet_3500CP-Postscript-HP.ppd.gz":                             true,
	"foomatic-20200219-HP-DesignJet_5000PS-Postscript-HP.ppd.gz":                             true,
	"foomatic-20200219-HP-DesignJet_5500ps-Postscript-HP.ppd.gz":                             true,
	"foomatic-20200219-HP-DesignJet_800PS-Postscript-HP.ppd.gz":                              true,
	"foomatic-20200219-HP-DeskJet_1000C-pnm2ppa.ppd.gz":                                      true,
	"foomatic-20200219-HP-DeskJet_712C-pnm2ppa.ppd.gz":                                       true,
	"foomatic-20200219-HP-DeskJet_722C-pnm2ppa.ppd.gz":                                       true,
	"foomatic-20200219-HP-DeskJet_820C-pnm2ppa.ppd.gz":                                       true,
	"foomatic-20200219-Imagistics-im8530-Postscript-Oce.ppd.gz":                              true,
	"foomatic-20200219-KONICA_MINOLTA-bizhub_750-Postscript-KONICA_MINOLTA.ppd.gz":           true,
	"foomatic-20200219-Kyocera-CS-1815-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-CS-C2525E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-CS-C3225E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-CS-C3232E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-CS-C4035E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-FS-1000-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1000plus-Postscript-Kyocera.ppd.gz":                        true,
	"foomatic-20200219-Kyocera-FS-1010-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1018MFP-Postscript-Kyocera.ppd.gz":                         true,
	"foomatic-20200219-Kyocera-FS-1020D-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-1030D-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-1050-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1200-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1700-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1700plus-Postscript-Kyocera.ppd.gz":                        true,
	"foomatic-20200219-Kyocera-FS-1800-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1800plus-Postscript-Kyocera.ppd.gz":                        true,
	"foomatic-20200219-Kyocera-FS-1900-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-1920-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-3750-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-3800-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-3820N-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-3830N-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-3900DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-4000DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-5900C-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-6020-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-6026-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-6700-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-6900-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-7000-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-8000C-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-FS-9000-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-FS-9100DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-9120DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-9500DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-9520DN-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C5015N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C5016N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C5020N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C5025N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C5030N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C8008N-Postscript-Kyocera.ppd.gz":                          true,
	"foomatic-20200219-Kyocera-FS-C8100DN-Postscript-Kyocera.ppd.gz":                         true,
	"foomatic-20200219-Kyocera-FS-C8100DNplus_KPDL-Postscript-Kyocera.ppd.gz":                true,
	"foomatic-20200219-Kyocera-KM-1815-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-2030-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-2530-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-3035-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-3530-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-4030-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-4035-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-5035-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-6030-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-8030-Postscript-Kyocera.ppd.gz":                            true,
	"foomatic-20200219-Kyocera-KM-C2520-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-KM-C2525E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-KM-C3225-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-KM-C3225E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-KM-C3232-Postscript-Kyocera.ppd.gz":                           true,
	"foomatic-20200219-Kyocera-KM-C3232E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Kyocera-KM-C4035E_KPDL-Postscript-Kyocera.ppd.gz":                     true,
	"foomatic-20200219-Lexmark-X125-drv_x125.ppd.gz":                                         true,
	"foomatic-20200219-Lexmark-X73-drv_z42.ppd.gz":                                           true,
	"foomatic-20200219-Lexmark-Z42-drv_z42.ppd.gz":                                           true,
	"foomatic-20200219-Lexmark-Z43-drv_z42.ppd.gz":                                           true,
	"foomatic-20200219-Oki-B4300-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C5300-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C7100-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C7200-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C7300-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C7400-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C7500-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C9200-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C9300-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C9400-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Oki-C9500-Postscript-Oki.ppd.gz":                                      true,
	"foomatic-20200219-Samsung-C268x-Postscript-Samsung.ppd.gz":                              true,
	"foomatic-20200219-Samsung-X3220-Postscript-Samsung.ppd.gz":                              true,
	"foomatic-20200219-Samsung-X401-Postscript-Samsung.ppd.gz":                               true,
	"foomatic-20200219-Samsung-X4300-Postscript-Samsung.ppd.gz":                              true,
	"foomatic-20200219-Samsung-X703-Postscript-Samsung.ppd.gz":                               true,
	"foomatic-20200219-Samsung-X7600-Postscript-Samsung.ppd.gz":                              true,
	"hp-20190918-hplip-3.19.6-hp-Mimas17.ppd.gz":                                             true,
	"hp-20190918-hplip-3.19.6-hp-P15_CISS.ppd.gz":                                            true,
	"hp-20190918-hplip-3.19.6-hp-SPDOfficejetProBsize.ppd.gz":                                true,
	"hplip-20200303-hplip-3.19.12-hp-designjet_Z6_24in-ps.ppd.gz":                            true,
	"hplip-20200303-hplip-3.19.12-hp-designjet_Z6_44in-ps.ppd.gz":                            true,
	"hplip-20200303-hplip-3.19.12-hp-designjet_Z6dr_44in-ps.ppd.gz":                          true,
	"hplip-20201209-hplip-3.20.11-hp-P15_CISS.ppd.gz":                                        true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_Z9_24in-ps.ppd.gz":                            true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_Z9_44in-ps.ppd.gz":                            true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_Z9dr_44in-ps.ppd.gz":                          true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_z2600_postscript-ps.ppd.gz":                   true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_z5400-postscript.ppd.gz":                      true,
	"hplip-20201209-hplip-3.20.11-hp-designjet_z5600_postscript-ps.ppd.gz":                   true,
	"konica_minolta-20200331-konica-minolta-20200331-konica-minolta-bizhub-758-jp-eu.ppd.gz": true,
	"konica_minolta-20200331-konica-minolta-20200331-konica-minolta-bizhub-808-us.ppd.gz":    true,
	"kyocera-20190830-Kyocera_Generic_Monochrome.ppd.gz":                                     true,
	"star-20191209-mcp20.ppd.gz":                                                             true,
	"star-20191209-mcp21.ppd.gz":                                                             true,
	"star-20191209-mcp30.ppd.gz":                                                             true,
	"star-20191209-mcp31.ppd.gz":                                                             true,
	"star-20191209-pop10.ppd.gz":                                                             true,
	"star-20191209-tsp651.ppd.gz":                                                            true,
}

// The PPD archive is generated via "go run ppdTool.go download" in
// src/third_party/autotest/files/client/site_tests/platform_PrinterPpds.
const ppdsAll = "ppds_all.tar.xz"

// fooRip detects if the PPD file uses the foomatic-rip print filter.
var fooRip = regexp.MustCompile(`foomatic-rip"`)

// fooCmd is the actual shell command (garbled) that foomatic filter executes.
// Two PPD files with the same fooCmd will tend to execute the same shell commands
// to generate print data. Note that setting the environment variable
// FOOMATIC_VERIFY_MODE prevents shell commands from being executed by the
// platform2 foomatic shell while still verifying that they are valid.
var fooCmd = regexp.MustCompile(`(?m)^\*FoomaticRIPCommandLine: "[^"]*"`)

type fileError struct {
	file string
	err  error
}

// testPPD creates temp file ppdFile with contents ppdContents and
// tests it for validity. ppdFile is deleted before returning.
func testPPD(ctx context.Context, fooCache *sync.Map, ppdFile string, ppdContents []byte) error {
	const pdf = `%PDF-1.0
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 3 3]>>endobj
xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000052 00000 n
0000000101 00000 n
trailer<</Size 4/Root 1 0 R>>
startxref
147
%EOF`
	if err := ioutil.WriteFile(ppdFile, ppdContents, 0644); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	defer os.Remove(ppdFile)
	// cupstestppd verifies that the PPD file is valid.
	// -W translations ignores translations strings as these are not used by the
	// ChromeOS version of CUPS. Debugd also calls cupstestppd with this flag.
	cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", ppdFile)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "cupstestppd")
	}
	// Check if the PPD file uses the foomatic-rip filter.
	if !fooRip.Match(ppdContents) {
		return nil
	}
	cmds := fooCmd.FindAll(ppdContents, 2)
	if len(cmds) > 1 {
		return errors.New("multiple FoomaticRIPCommandLine matches")
	}
	id := ""
	if len(cmds) == 1 {
		id = string(cmds[0])
	}
	if fooCache != nil {
		val, ok := fooCache.Load(id)
		if ok {
			if val != "" {
				return errors.Errorf("foomatic-rip: same error as %q", val)
			}
			return nil
		}
	}
	cmd = testexec.CommandContext(ctx, "foomatic-rip", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/)
	cmd.Env = []string{"FOOMATIC_VERIFY_MODE=true",
		"PATH=/bin:/usr/bin:/usr/libexec/cups/filter",
		"PPD=" + ppdFile}
	cmd.Stdin = bytes.NewReader([]byte(pdf))
	err := cmd.Run(testexec.DumpLogOnError)
	if fooCache != nil {
		val := ""
		if err != nil {
			val = filepath.Base(ppdFile)
		}
		fooCache.Store(id, val)
	}
	if err != nil {
		return errors.Wrap(err, "foomatic-rip")
	}
	return nil
}

// extractPPD extracts a gzip compressed PPD file and returns its contents.
func extractPPD(ctx context.Context, ppd []byte) ([]byte, error) {
	buf, err := gzip.NewReader(bytes.NewReader(ppd))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create reader")
	}
	ppd, readErr := ioutil.ReadAll(buf)
	if err := buf.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close gzip: ", err)
	}
	if readErr != nil {
		return nil, errors.Wrap(readErr, "failed to read gzip")
	}
	return ppd, nil
}

// testPPDs receives PPD files to test (located in dir) through the
// files channel and returns any errors through the errors channel.
func testPPDs(ctx context.Context, dir string, files <-chan string, errors chan<- []fileError, fooCache *sync.Map) {
	const gzExt = ".gz"
	var errs []fileError
	for file := range files {
		if err := func() error {
			ppd, err := ioutil.ReadFile(filepath.Join(dir, file))
			if err != nil {
				return err
			}
			if filepath.Ext(file) == gzExt {
				ppd, err = extractPPD(ctx, ppd)
				if err != nil {
					return err
				}
				file = file[:len(file)-len(gzExt)]
			}
			return testPPD(ctx, fooCache, filepath.Join(dir, file), ppd)
		}(); err != nil {
			errs = append(errs, fileError{file, err})
		}
	}
	errors <- errs
}

// extractArchive extracts a tar.xz archive via the tar command.
func extractArchive(ctx context.Context, src, dst string) error {
	return testexec.CommandContext(ctx, "tar", "-xC", dst, "-f", src, "--strip-components=1").Run(testexec.DumpLogOnError)
}

func TestPPDs(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(dir)
	// ppds_all.tar.xz takes around 60M when decompressed.
	if err := extractArchive(ctx, s.DataPath(ppdsAll), dir); err != nil {
		s.Fatal("Failed to extract archive: ", err)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Fatal("Failed to read directory: ", err)
	}
	n := len(files)
	// Stride must be a power of 2.
	// It is used to prevent fooCache cache misses by spacing out the
	// lexicographically sorted files so that they are no longer in
	// lexicographical order.
	const stride = 1 << 6
	if n <= stride+1 {
		s.Fatalf("Too few files: %d found", n)
	}
	input := make(chan string, n)
	// Always use at least 2 goroutines in case one of them is blocked.
	numRoutines := runtime.NumCPU() + 1
	testing.ContextLogf(ctx, "Creating %d goroutines", numRoutines)
	errors := make(chan []fileError, numRoutines)
	// fooCache maps fooCmd to an empty string if foomatic-rip succeeds and if it fails,
	// the name of the PPD file on which it failed.
	var fooCache *sync.Map
	// Disabling cacheFoo roughly triples the runtime.
	const cacheFoo = true
	if cacheFoo {
		fooCache = new(sync.Map)
	}
	for i := 0; i < numRoutines; i++ {
		go testPPDs(ctx, dir, input, errors, fooCache)
	}
	// Ensure n is coprime to stride so that each file is selected exactly once.
	if n&1 == 0 {
		n--
		input <- files[n].Name()
	}
	// Space the files out to avoid fooCache cache misses.
	for i := 0; i != n; i += stride {
		if i > n {
			i -= n
		}
		if _, exists := skip[files[i].Name()]; !exists {
			input <- files[i].Name()
		}
	}
	close(input)

	var errs []fileError
	for i := 0; i < numRoutines; i++ {
		errs = append(errs, <-errors...)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].file < errs[j].file })
	for _, fileErr := range errs {
		s.Errorf("%s: %v", fileErr.file, fileErr.err)
	}
}
