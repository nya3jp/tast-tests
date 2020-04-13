// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linuxssh

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/sshtest"
	"chromiumos/tast/testutil"
)

// initFileTest creates a temporary directory with a subdirectory containing files.
// The temp dir's and subdir's paths are returned.
func initFileTest(t *testing.T, files map[string]string) (tmpDir, srcDir string) {
	tmpDir = testutil.TempDir(t)
	srcDir = filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if err := testutil.WriteFiles(srcDir, files); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	return tmpDir, srcDir
}

// checkFile returns an error if p's contents differ from exp.
func checkFile(p, exp string) error {
	if b, err := ioutil.ReadFile(p); err != nil {
		return errors.Wrapf(err, "failed to read %v after copy", p)
	} else if string(b) != exp {
		return errors.Errorf("expected content %q after copy, got %v", exp, string(b))
	}
	return nil
}

// checkDir returns an error if dir's contents don't match exp, a map from paths to data.
func checkDir(dir string, exp map[string]string) error {
	if act, err := testutil.ReadFiles(dir); err != nil {
		return err
	} else if !reflect.DeepEqual(exp, act) {
		return errors.Errorf("files not copied correctly: src %v, dst %v", exp, act)
	}
	return nil
}

func TestGetFileRegular(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	files := map[string]string{"file": "foo"}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(srcDir, "file")
	dstFile := filepath.Join(tmpDir, "file.copy")
	if err := GetFile(td.Ctx, td.Hst, srcFile, dstFile); err != nil {
		t.Fatal(err)
	}
	if err := checkFile(dstFile, files["file"]); err != nil {
		t.Error(err)
	}

	// GetFile should overwrite local files.
	if err := GetFile(td.Ctx, td.Hst, srcFile, dstFile); err != nil {
		t.Error(err)
	}
}

func TestGetFileDir(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	files := map[string]string{
		"myfile":     "some data",
		"mydir/file": "this is in a subdirectory",
	}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)

	// Copy the full source directory.
	dstDir := filepath.Join(tmpDir, "dst")
	if err := GetFile(td.Ctx, td.Hst, srcDir, dstDir); err != nil {
		t.Fatal(err)
	}
	if err := checkDir(dstDir, files); err != nil {
		t.Error(err)
	}

	// GetFile should overwrite local dirs.
	if err := GetFile(td.Ctx, td.Hst, srcDir, dstDir); err != nil {
		t.Error(err)
	}
}

func TestGetFileTimeout(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	files := map[string]string{"file": "data"}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(srcDir, "file")
	dstFile := filepath.Join(tmpDir, "file")
	td.ExecTimeout = sshtest.StartTimeout
	if err := GetFile(td.Ctx, td.Hst, srcFile, dstFile); err == nil {
		t.Errorf("GetFile() with expired context didn't return error")
	}
}

func TestPutFiles(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	const (
		weirdSrcName = "weird[a], *.b" // various regexp chars plus comma delimiter
		weirdDstName = "\\1\\a,&"      // sed replacement chars plus comma delimiter
	)
	files := map[string]string{
		"file1":             "file1 new",
		"dir/file2":         "file2 new",
		"dir2/subdir/file3": "file3 new",
		"file4":             "file4 new",
		weirdSrcName:        "file5 new",
		"file6":             "file6 new",
	}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)

	dstDir := filepath.Join(tmpDir, "dst")
	if err := testutil.WriteFiles(dstDir, map[string]string{
		"file1":             "file1 old",
		"dir/file2":         "file2 old",
		"dir2/subdir/file3": "file3 old",
		"file4":             "file4 old",
	}); err != nil {
		t.Fatal(err)
	}

	if n, err := PutFiles(td.Ctx, td.Hst, map[string]string{
		filepath.Join(srcDir, "file1"):             filepath.Join(dstDir, "newfile1"),           // rename to preserve orig file
		filepath.Join(srcDir, "dir/file2"):         filepath.Join(dstDir, "dir/file2"),          // overwrite orig file
		filepath.Join(srcDir, "dir2/subdir/file3"): filepath.Join(dstDir, "dir2/subdir2/file3"), // rename subdir
		filepath.Join(srcDir, weirdSrcName):        filepath.Join(dstDir, "file5"),              // check that regexp chars are escaped
		filepath.Join(srcDir, "file6"):             filepath.Join(dstDir, weirdDstName),         // check that replacement chars are also escaped
	}, PreserveSymlinks); err != nil {
		t.Fatal(err)
	} else if n <= 0 {
		t.Errorf("Copied non-positive %v bytes", n)
	}

	if err := checkDir(dstDir, map[string]string{
		"file1":              "file1 old",
		"newfile1":           "file1 new",
		"dir/file2":          "file2 new",
		"dir2/subdir/file3":  "file3 old",
		"dir2/subdir2/file3": "file3 new",
		"file4":              "file4 old",
		"file5":              "file5 new",
		weirdDstName:         "file6 new",
	}); err != nil {
		t.Error(err)
	}
}

func TestPutFilesUnchanged(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	files := map[string]string{
		"src1":     "1",
		"dir/src2": "2",
	}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)

	dstDir := filepath.Join(tmpDir, "dst")
	if err := testutil.WriteFiles(dstDir, map[string]string{
		"dst1":     "1",
		"dir/dst2": "2",
	}); err != nil {
		t.Fatal(err)
	}

	// No bytes should be sent since the dest dir already contains the renamed source files.
	if n, err := PutFiles(td.Ctx, td.Hst, map[string]string{
		filepath.Join(srcDir, "src1"):     filepath.Join(dstDir, "dst1"),
		filepath.Join(srcDir, "dir/src2"): filepath.Join(dstDir, "dir/dst2"),
	}, PreserveSymlinks); err != nil {
		t.Fatal(err)
	} else if n != 0 {
		t.Errorf("PutFiles() copied %v bytes; want 0", n)
	}
}

func TestPutFilesTimeout(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	files := map[string]string{"file": "data"}
	tmpDir, srcDir := initFileTest(t, files)
	defer os.RemoveAll(tmpDir)
	dstDir := filepath.Join(tmpDir, "dst")
	td.ExecTimeout = sshtest.EndTimeout
	if _, err := PutFiles(td.Ctx, td.Hst, map[string]string{
		filepath.Join(srcDir, "file"): filepath.Join(dstDir, "file"),
	}, PreserveSymlinks); err == nil {
		t.Errorf("PutFiles() with expired context didn't return error")
	}
}

func TestPutFilesSymlinks(t *testing.T) {
	t.Parallel()
	td := sshtest.NewTestDataConn(t)
	defer td.Close()

	tmpDir, srcDir := initFileTest(t, nil)
	defer os.RemoveAll(tmpDir)

	// Create a symlink pointing to an executable file outside of the source dir.
	const (
		data = "executable file"
		link = "symlink"
		mode = 0755
	)
	targetPath := filepath.Join(tmpDir, "exe")
	if err := ioutil.WriteFile(targetPath, []byte(data), mode); err != nil {
		t.Fatalf("Failed to write %v: %v", targetPath, err)
	}
	if err := os.Symlink(targetPath, filepath.Join(srcDir, link)); err != nil {
		t.Fatalf("Failed to create symlink to %v: %v", targetPath, err)
	}

	// PreserveSymlinks should copy symlinks directly.
	dstDir := filepath.Join(tmpDir, "dst_preserve")
	if _, err := PutFiles(td.Ctx, td.Hst, map[string]string{
		filepath.Join(srcDir, link): filepath.Join(dstDir, link),
	}, PreserveSymlinks); err != nil {
		t.Error("PutFiles failed with PreserveSymlinks: ", err)
	} else {
		dstFile := filepath.Join(dstDir, link)
		if fi, err := os.Lstat(dstFile); err != nil {
			t.Errorf("Failed to lstat %v: %v", dstFile, err)
		} else if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%v is not a symlink; got mode %v", dstFile, fi.Mode())
		} else if target, err := os.Readlink(dstFile); err != nil {
			t.Errorf("Failed to read %v: %v", dstFile, err)
		} else if target != targetPath {
			t.Errorf("%v has target %v; want %v", dstFile, target, targetPath)
		}
	}

	// DereferenceSymlinks should copy symlinks' targets while preserving the original mode.
	dstDir = filepath.Join(tmpDir, "dst_deref")
	if _, err := PutFiles(td.Ctx, td.Hst, map[string]string{
		filepath.Join(srcDir, link): filepath.Join(dstDir, link),
	}, DereferenceSymlinks); err != nil {
		t.Error("PutFiles failed with DereferenceSymlinks: ", err)
	} else {
		dstFile := filepath.Join(dstDir, link)
		if fi, err := os.Lstat(dstFile); err != nil {
			t.Errorf("Failed to lstat %v: %v", dstFile, err)
		} else if fi.Mode()&os.ModeType != 0 {
			t.Errorf("%v is not a regular file; got mode %v", dstFile, fi.Mode())
		} else if perm := fi.Mode() & os.ModePerm; perm != mode {
			t.Errorf("%v has perms %#o; want %#o", dstFile, perm, mode)
		}
		if b, err := ioutil.ReadFile(dstFile); err != nil {
			t.Errorf("Failed to read %v: %v", dstFile, err)
		} else if string(b) != data {
			t.Errorf("%v contains %q; want %q", dstFile, b, data)
		}
	}
}
