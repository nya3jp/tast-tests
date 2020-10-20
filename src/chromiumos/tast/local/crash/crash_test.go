// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	gotesting "testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testutil"
)

// crashFile contains information about a crash file used by tests.
// The testutil package uses relative paths while the crash package
// uses absolute paths, so this struct stores both.
type crashFile struct{ rel, abs, data string }

// writeCrashFile writes a file with relative path rel containing data to dir.
func writeCrashFile(t *gotesting.T, dir, rel, data string) crashFile {
	cf := crashFile{rel, filepath.Join(dir, rel), data}
	if err := testutil.WriteFiles(dir, map[string]string{rel: data}); err != nil {
		t.Fatal(err)
	}
	return cf
}

func TestGetCrashes(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	writeCrashFile(t, td, "foo.txt", "") // skipped because non-core/dmp extension
	fooCore := writeCrashFile(t, td, "foo.core", "")
	fooDmp := writeCrashFile(t, td, "foo.dmp", "")
	fooLog := writeCrashFile(t, td, "foo.log", "")
	fooMeta := writeCrashFile(t, td, "foo.meta", "")
	fooInfo := writeCrashFile(t, td, "foo.info", "")
	fooProclog := writeCrashFile(t, td, "foo.proclog", "")
	fooGPU := writeCrashFile(t, td, "foo.i915_error_state.log.xz", "")
	fooCompressedTxt := writeCrashFile(t, td, "foo.txt.gz", "")
	fooBIOSLog := writeCrashFile(t, td, "foo.bios_log", "")
	fooKCrash := writeCrashFile(t, td, "foo.kcrash", "")
	fooCompressedLog := writeCrashFile(t, td, "foo.log.gz", "")
	barDmp := writeCrashFile(t, td, "bar.dmp", "")
	writeCrashFile(t, td, "bar", "")            // skipped because no extenison
	writeCrashFile(t, td, "subdir/baz.dmp", "") // skipped because in subdir
	writeCrashFile(t, td, "foo.info.gz", "")    // skipped because second extension is wrong
	writeCrashFile(t, td, "other.xz", "")

	dirs := []string{filepath.Join(td, "missing"), td} // nonexistent dir should be skipped
	files, err := GetCrashes(dirs...)
	if err != nil {
		t.Fatalf("GetCrashes(%v) failed: %v", dirs, err)
	}
	sort.Strings(files)
	if exp := []string{barDmp.abs, fooBIOSLog.abs, fooCore.abs, fooDmp.abs, fooGPU.abs, fooInfo.abs, fooKCrash.abs, fooLog.abs, fooCompressedLog.abs, fooMeta.abs, fooProclog.abs, fooCompressedTxt.abs}; !reflect.DeepEqual(files, exp) {
		t.Errorf("GetCrashes(%v) = %v; want %v", dirs, files, exp)
	}
}

func TestProcessRunning(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// procName must be <=14 characters long so that gopsutil doesn't look at
	// /proc/$$/cmdline.
	procName := fmt.Sprintf("t_%d", rand.Int31())
	if err := ioutil.WriteFile(filepath.Join(td, procName), []byte("#!/bin/sh\nsleep 10\n"), 0777); err != nil {
		t.Fatal("Failed to write a script: ", err)
	}

	cmd := exec.Command(filepath.Join(td, procName))
	if err := cmd.Start(); err != nil {
		t.Fatal("Failed to start a script: ", err)
	}
	func() {
		defer cmd.Wait()
		defer cmd.Process.Kill()

		if err := testing.Poll(context.Background(), func(ctx context.Context) error {
			running, err := processRunning(procName)
			if err != nil {
				return testing.PollBreak(err)
			}
			if !running {
				return errors.New("processRunning = false")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			t.Fatal("Failed to wait for processRunning to return true: ", err)
		}
	}()

	if err := testing.Poll(context.Background(), func(ctx context.Context) error {
		running, err := processRunning(procName)
		if err != nil {
			return testing.PollBreak(err)
		}
		if running {
			return errors.New("processRunning = true")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		t.Fatal("Failed to wait for processRunning to return false: ", err)
	}
}

func compareWaitForCrashFilesResultHelper(results, expected map[string][]string, td, message string) error {
	// Order doesn't matter for the list of files, so sort before comparison.
	for _, value := range expected {
		for i := range value {
			value[i] = filepath.Join(td, value[i])
		}
		sort.Strings(value)
	}
	for _, value := range results {
		sort.Strings(value)
	}
	if !reflect.DeepEqual(results, expected) {
		return errors.Errorf(message, results, expected)
	}

	return nil
}

func compareWaitForCrashFilesResult(results, expected map[string][]string, td string) error {
	return compareWaitForCrashFilesResultHelper(results, expected, td, "WaitForCrashFiles returned %v, expected %v")
}

func compareRegexesNotFoundPartialMatches(results, expected map[string][]string, td string) error {
	return compareWaitForCrashFilesResultHelper(results, expected, td, "RegexesNotFound.PartialMatches was %v, expected %v")
}

func compareRegexesNotFound(actualErr error, expectedErr *RegexesNotFound, td string) error {
	notFoundErr, ok := actualErr.(RegexesNotFound)
	if expectedErr != nil {
		if !ok {
			return errors.Errorf("WaitForCrashFiles returned error %v, expected RegexesNotFound error %v", actualErr, expectedErr)
		}
		var issues []string

		for i := range expectedErr.Files {
			expectedErr.Files[i] = filepath.Join(td, expectedErr.Files[i])
		}
		// Order doesn't matter for the list of files, so sort before comparison.
		sort.Strings(expectedErr.Files)
		sort.Strings(notFoundErr.Files)
		if !reflect.DeepEqual(notFoundErr.Files, expectedErr.Files) {
			issues = append(issues, fmt.Sprintf("Bad Files list in err, got %v, expected %v", notFoundErr.Files, expectedErr.Files))
		}

		// Order doesn't matter for the missing list, so sort before comparison.
		sort.Strings(expectedErr.Missing)
		sort.Strings(notFoundErr.Missing)
		if !reflect.DeepEqual(notFoundErr.Missing, expectedErr.Missing) {
			issues = append(issues, fmt.Sprintf("Bad Missing list in err, got %v,  expected %v", notFoundErr.Missing, expectedErr.Missing))
		}

		if err := compareRegexesNotFoundPartialMatches(notFoundErr.PartialMatches, expectedErr.PartialMatches, td); err != nil {
			issues = append(issues, err.Error())
		}

		if len(issues) > 0 {
			return errors.Errorf("Returned RegexesNotFound error didn't match expected %v", issues)
		}
	} else if ok {
		return errors.New("WaitForCrashFiles returned RegexesNotFound, but was expecting different error")
	}

	return nil
}

func writeFiles(dir string, files []string, fileContents []byte, overrideFileContents map[string][]byte) error {
	for _, fn := range files {
		contents := fileContents
		if val, ok := overrideFileContents[fn]; ok {
			contents = val
		}
		if err := ioutil.WriteFile(filepath.Join(dir, fn), contents, 0666); err != nil {
			return errors.Wrap(err, "failed to touch file")
		}
	}

	return nil
}

const metaRegex = `.*\.\d{1,8}\.meta`
const dmpRegex = `.*\.\d{1,8}\.dmp`
const logRegex = `.*\.\d{1,8}\.log`
const kcrashRegex = `.*\.\d{1,8}\.kcrash`

var waitForCrashFilesTests = []struct {
	// name is the test name.
	name string
	// files is the list of files to create before calling WaitForCrashFiles.
	files []string
	// laterFiles is a list of files that are created while WaitForCrashFiles is
	// running.
	laterFiles []string
	// fileContents is the contents of the files. (By default, all files have the
	// same contents). Usually contains 'done=1' so that meta files are considered
	// valid.
	fileContents []byte
	// overrideFileContents is a map of file name to contents. If present, it
	// overrides fileContents for the files that appear in the map.
	overrideFileContents map[string][]byte
	// regexes is the regexes parameter to WaitForCrashFiles
	regexes []string
	// optionalRegexes, if not nil, is passed via the OptionalRegexes option.
	optionalRegexes []string
	// expectedResults is the expected non-error return value. The value strings
	// are the base names of the files, which will have the directory prepended
	// during the test.
	expectedResults map[string][]string
	// expectErr is true if we expect error to be non-nil.
	expectErr bool
	// expectErrRegexesNotFound, if not nil, is the RegexesNotFound error we
	// expect to get from WaitForCrashFiles. Only checked if expectErr is true.
	// Note that the various files (in Files and PartialMatches) will have the
	// directory prepended, so only the base names are listed in the test case.
	expectErrRegexesNotFound *RegexesNotFound
	// timeout, if not 0, is the timeout for WaitForCrashFiles
	timeout time.Duration
}{{
	name: "Success",
	// "notreturned.*" will not be returned because it's missing the PID and so
	// doesn't match the pattern we provide below.
	files:        []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		metaRegex: []string{"a.15.meta", "b.7.meta"},
		dmpRegex:  []string{"a.15.dmp", "b.7.dmp"},
		logRegex:  []string{"a.15.log", "b.7.log"},
	},
	expectErr: false,
}, {
	name: "MissOneRegex",
	// No .log files, only .dmp and .meta.
	files:        []string{"a.15.meta", "a.15.dmp", "b.7.meta", "b.7.dmp", "notreturned.kcrash"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectErr:    true,
	expectErrRegexesNotFound: &RegexesNotFound{
		Missing: []string{logRegex},
		Files:   []string{"a.15.meta", "a.15.dmp", "b.7.meta", "b.7.dmp", "notreturned.kcrash"},
		PartialMatches: map[string][]string{
			metaRegex: []string{"a.15.meta", "b.7.meta"},
			dmpRegex:  []string{"a.15.dmp", "b.7.dmp"},
		},
	},
	timeout: time.Second,
}, {
	name:         "MissAllRegexes",
	files:        []string{"notreturned.kcrash"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectErr:    true,
	expectErrRegexesNotFound: &RegexesNotFound{
		Missing:        []string{metaRegex, dmpRegex, logRegex},
		Files:          []string{"notreturned.kcrash"},
		PartialMatches: map[string][]string{},
	},
	timeout: time.Second,
}, {
	name:         "ActuallyWaits",
	files:        []string{"a.15.log", "a.15.dmp", "b.7.log", "b.7.dmp"},
	laterFiles:   []string{"a.15.meta", "b.7.meta"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		metaRegex: []string{"a.15.meta", "b.7.meta"},
		dmpRegex:  []string{"a.15.dmp", "b.7.dmp"},
		logRegex:  []string{"a.15.log", "b.7.log"},
	},
	expectErr: false,
}, {
	name:         "MetasNeedDone",
	files:        []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectErr:    true,
	expectErrRegexesNotFound: &RegexesNotFound{
		Missing: []string{metaRegex},
		Files:   []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
		PartialMatches: map[string][]string{
			logRegex: []string{"a.15.log", "b.7.log"},
			dmpRegex: []string{"a.15.dmp", "b.7.dmp"},
		},
	},
	timeout: time.Second,
}, {
	name:         "NonMetasDontNeedDone",
	files:        []string{"a.15.dmp", "a.15.log", "b.7.dmp", "b.7.log"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex: []string{"a.15.dmp", "b.7.dmp"},
		logRegex: []string{"a.15.log", "b.7.log"},
	},
	expectErr: false,
}, {
	name:                 "OnlyOneMetaNeedsDone",
	files:                []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
	fileContents:         []byte("foo=bar"),
	overrideFileContents: map[string][]byte{"a.15.meta": []byte("done=1")},
	regexes:              []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex:  []string{"a.15.dmp", "b.7.dmp"},
		logRegex:  []string{"a.15.log", "b.7.log"},
		metaRegex: []string{"a.15.meta"}, // Note no b.7.meta
	},
	expectErr: false,
}, {
	name:         "NonCrashFilesAlwaysIgnored",
	files:        []string{"a.dmp", "a.kcrash", "a.notaknownextension"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{".*"},
	expectedResults: map[string][]string{
		".*": []string{"a.dmp", "a.kcrash"},
	},
	expectErr: false,
}, {
	name:         "NonCrashFilesAlwaysIgnored2",
	files:        []string{"a.notaknownextension"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{".*"},
	expectErr:    true,
	expectErrRegexesNotFound: &RegexesNotFound{
		Missing:        []string{".*"},
		Files:          nil,
		PartialMatches: map[string][]string{},
	},
	timeout: time.Second,
}, {
	name:            "OptionalRegexesAreMatched",
	files:           []string{"a.15.meta", "a.15.dmp", "a.15.log", "a.15.kcrash", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents:    []byte("foo=bar\ndone=1"),
	regexes:         []string{metaRegex, dmpRegex},
	optionalRegexes: []string{logRegex, kcrashRegex},
	expectedResults: map[string][]string{
		metaRegex:   []string{"a.15.meta", "b.7.meta"},
		dmpRegex:    []string{"a.15.dmp", "b.7.dmp"},
		logRegex:    []string{"a.15.log", "b.7.log"},
		kcrashRegex: []string{"a.15.kcrash"},
	},
	expectErr: false,
}, {
	name:            "OptionalRegexesDontCauseErrors",
	files:           []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents:    []byte("foo=bar\ndone=1"),
	regexes:         []string{metaRegex, dmpRegex},
	optionalRegexes: []string{logRegex, kcrashRegex},
	// kcrashReges not matched
	expectedResults: map[string][]string{
		metaRegex: []string{"a.15.meta", "b.7.meta"},
		dmpRegex:  []string{"a.15.dmp", "b.7.dmp"},
		logRegex:  []string{"a.15.log", "b.7.log"},
	},
	expectErr: false,
}, {
	name:            "OnlyOptionalRegexesAlwaysSucceeds",
	files:           nil,
	regexes:         nil,
	optionalRegexes: []string{metaRegex, dmpRegex, logRegex, kcrashRegex},
	expectedResults: map[string][]string{},
	expectErr:       false,
}}

func TestWaitForCrashFiles(t *gotesting.T) {
	t.Parallel()
	for _, testCase := range waitForCrashFilesTests {
		testCase := testCase // NOTE: https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		t.Run(testCase.name, func(t *gotesting.T) {
			t.Parallel() // Run subtests in parallel because some have timeouts
			td := testutil.TempDir(t)
			defer os.RemoveAll(td)
			if err := writeFiles(td, testCase.files, testCase.fileContents, testCase.overrideFileContents); err != nil {
				t.Fatal("Failed to create files: ", err)
			}
			if testCase.laterFiles != nil {
				time.AfterFunc(time.Second, func() {
					if err := writeFiles(td, testCase.laterFiles, testCase.fileContents, testCase.overrideFileContents); err != nil {
						t.Fatal("Failed to create late-arriving files: ", err)
					}
				})
			}

			opts := make([]WaitForCrashFilesOpt, 0)
			if testCase.timeout != 0 {
				opts = append(opts, Timeout(testCase.timeout))
			}
			if testCase.optionalRegexes != nil {
				opts = append(opts, OptionalRegexes(testCase.optionalRegexes))
			}
			results, err := WaitForCrashFiles(context.Background(), []string{td}, testCase.regexes, opts...)
			if testCase.expectedResults == nil && results != nil {
				t.Error("WaitForCrashFiles returned ", results, ", expected no results")
			} else {
				if err := compareWaitForCrashFilesResult(results, testCase.expectedResults, td); err != nil {
					t.Error(err)
				}
			}

			if testCase.expectErr {
				if err == nil {
					t.Error("WaitForCrashFiles succeeded, expected error")
				} else if compareErr := compareRegexesNotFound(err, testCase.expectErrRegexesNotFound, td); compareErr != nil {
					t.Error(compareErr)
				}
			} else if err != nil {
				t.Error("WaitForCrashFiles had error ", err, ", expected success")
			}
		})
	}
}

func TestDeleteCoreDumps(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	dir1 := filepath.Join(td, "dir1") // dir1 is missing
	dir2 := filepath.Join(td, "dir2")
	if err := os.Mkdir(dir2, 0777); err != nil {
		t.Fatal("Failed to create dir: ", err)
	}

	initFiles := []string{"a.core", "a.txt", "b.core", "b.dmp", "b.jpg"}
	for _, fn := range initFiles {
		if err := ioutil.WriteFile(filepath.Join(dir2, fn), nil, 0666); err != nil {
			t.Fatal("Failed to touch file: ", err)
		}
	}

	// filesIn returns a list of files under dir.
	filesIn := func(dir string) []string {
		fis, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Fatal("ReadDir failed: ", err)
		}
		var files []string
		for _, fi := range fis {
			files = append(files, fi.Name())
		}
		sort.Strings(files)
		return files
	}

	// First, test the behavior when crash_reporter is running.
	func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reporterRunning := func() (bool, error) {
			// Cancel the context so that deleteCoreDumps returns after observing exactly once that
			// crash_reporter is running.
			cancel()
			return true, nil
		}

		if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err == nil {
			t.Error("deleteCoreDumps succeeded unexpectedly while crash_reporter is running")
		}

		got := filesIn(dir2)
		if diff := cmp.Diff(got, initFiles); diff != "" {
			t.Error("Files mismatch after failed deleteCoreDumps (-got +want):\n", diff)
		}
	}()

	// Second, test the behavior when crash_reporter is not running.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporterRunning := func() (bool, error) {
		// Create a new core file. This should not be deleted.
		for _, fn := range []string{"c.core", "c.dmp"} {
			if err := ioutil.WriteFile(filepath.Join(dir2, fn), nil, 0666); err != nil {
				t.Fatal("Failed to touch file: ", err)
			}
		}
		return false, nil
	}

	if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err != nil {
		t.Error("deleteCoreDumps failed when crash_reporter is not running: ", err)
	}

	// a.core: Not deleted because a.dmp does not exist.
	// b.core: Deleted.
	// c.core: Not deleted because it was created after waiting for crash_reporter.
	got := filesIn(dir2)
	want := []string{"a.core", "a.txt", "b.dmp", "b.jpg", "c.core", "c.dmp"}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Error("Files mismatch after successful deleteCoreDumps (-got +want):\n", diff)
	}
}
