// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

// PreparedArchives is a list of data files used in the test.
var PreparedArchives = []string{
	"crosdisks/Duplicate Filenames.zip",
	"crosdisks/Encrypted Full V4.rar",
	"crosdisks/Encrypted Full V5.rar",
	"crosdisks/Encrypted Partial V4.rar",
	"crosdisks/Encrypted Partial V5.rar",
	"crosdisks/Encrypted AES-128.zip",
	"crosdisks/Encrypted AES-192.zip",
	"crosdisks/Encrypted AES-256.zip",
	"crosdisks/Encrypted ZipCrypto.zip",
	"crosdisks/Encrypted Various.zip",
	"crosdisks/Invalid.rar",
	"crosdisks/Invalid.zip",
	"crosdisks/Format V4.rar",
	"crosdisks/Format V5.rar",
	"crosdisks/Multipart Old Style.rar",
	"crosdisks/Multipart Old Style.r00",
	"crosdisks/Multipart New Style 01.rar",
	"crosdisks/Multipart New Style 02.rar",
	"crosdisks/Multipart New Style 03.rar",
	"crosdisks/Nested.rar",
	"crosdisks/Nested.zip",
	"crosdisks/Strict Password.zip",
	"crosdisks/Symlinks.zip",
	"crosdisks/Unicode.zip",
	"crosdisks/MacOS UTF-8 Bug 903664.zip",
	"crosdisks/SJIS Bug 846195.zip",
	"crosdisks/archive.rar",
	"crosdisks/archive.tar",
	"crosdisks/archive.tar.gz",
	"crosdisks/archive.zip",
}

func withMountedArchiveDo(ctx context.Context, cd *crosdisks.CrosDisks, archivePath, password string, f func(ctx context.Context, mountPath string) error) error {
	options := ""
	if password != "" {
		options = "password=" + password
	}
	return withMountDo(ctx, cd, archivePath, filepath.Ext(archivePath), options, f)
}

func verifyArchiveContent(ctx context.Context, cd *crosdisks.CrosDisks, archivePath, password string, expectedContent DirectoryContents) error {
	return withMountedArchiveDo(ctx, cd, archivePath, password, func(ctx context.Context, mountPath string) error {
		return verifyDirectoryContents(ctx, mountPath, expectedContent)
	})
}

func verifyEncryptedArchiveContent(ctx context.Context, cd *crosdisks.CrosDisks, archivePath, password string, expectedContent DirectoryContents) error {
	// Check that it fails without a password.
	if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), "", crosdisks.MountErrorNeedPassword); err != nil {
		return errors.Wrap(err, "verification failed for encrypted archive without password")
	}
	// Check that it fails with a wrong password.
	for _, pw := range []string{"", password + "foo", password[0 : len(password)-1]} {
		if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), "password="+pw, crosdisks.MountErrorNeedPassword); err != nil {
			return errors.Wrap(err, "verification failed for encrypted archive with incorrect password")
		}
	}

	return verifyArchiveContent(ctx, cd, archivePath, password, expectedContent)
}

func testValidArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	// Each archive.* file has a different file format but they all contain a
	// 942 byte "romeo.txt" file that starts with the line "Romeo and Juliet".
	romeoAndJuliet := []byte("Romeo and Juliet")
	for _, f := range []string{
		"archive.rar",
		"archive.tar",
		"archive.tar.gz",
		"archive.zip",
	} {
		if err := withMountedArchiveDo(ctx, cd, filepath.Join(dataDir, f), "", func(ctx context.Context, mountPath string) error {
			data, err := ioutil.ReadFile(filepath.Join(mountPath, "romeo.txt"))
			if err != nil {
				return errors.Wrap(err, `could not read "romeo.txt" within archive`)
			} else if (len(data) != 942) || !bytes.HasPrefix(data, romeoAndJuliet) {
				return errors.New(`unexpected contents for "romeo.txt" within archive`)
			}
			return nil
		}); err != nil {
			s.Errorf("Test failed for %q: %v", f, err)
		}
	}
}

func testInvalidArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	for _, f := range []string{
		"Invalid.rar",
		"Invalid.zip",
		"Not There.rar",
		"Not There.zip",
	} {
		if err := verifyMountStatus(ctx, cd, filepath.Join(dataDir, f), filepath.Ext(f), "", crosdisks.MountErrorMountProgramFailed); err != nil {
			s.Errorf("Unexpected status of mounting invalid archive %q: %v", f, err)
		}
	}
}

func testMultipartArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	data := make([]string, 201)
	for i := 0; i < 200; i++ {
		data[i] = fmt.Sprintf("Line %03d", i+1)
	}
	expectedContent := DirectoryContents{
		"Lines": {Data: []byte(strings.Join(data, "\n"))},
	}
	for _, archive := range []string{
		"Multipart Old Style.rar",
		"Multipart New Style 01.rar",
		"Multipart New Style 02.rar",
		"Multipart New Style 03.rar",
	} {
		if err := verifyArchiveContent(ctx, cd, filepath.Join(dataDir, archive), "", expectedContent); err != nil {
			s.Errorf("Test failed for %q: %v", archive, err)
		}
	}
}

func testNestedArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	for _, archive := range []string{"Nested.rar", "Nested.zip"} {
		expectedMountPath := filepath.Join("/media/archive", archive)
		if err := withMountedArchiveDo(ctx, cd, filepath.Join(dataDir, archive), "", func(ctx context.Context, mountPath string) error {
			if mountPath != expectedMountPath {
				return errors.Errorf("mount path is different from expected one: got %q, want %q", mountPath, expectedMountPath)
			}
			return verifyUnicodeArchives(ctx, cd, mountPath)
		}); err != nil {
			s.Errorf("Test failed for %q: %v", archive, err)
		}
	}
}

func verifyUnicodeArchives(ctx context.Context, cd *crosdisks.CrosDisks, archiveDir string) error {
	// Test RAR V4 with Unicode BMP characters in file and directory names.
	expectedContent := DirectoryContents{
		"File D79F \uD79F.txt": {Data: []byte("Char U+D79F is \uD79F HANGUL SYLLABLE HIC\n")},
		" Space Oddity ":       {Data: []byte("Mind the gap\n")},
		"Level 1/Empty":        {Data: []byte{}},
		"Level 1/Digits":       {Data: []byte("0123456789")},
		"Level 1/Small":        {Data: []byte("Small file\n")},
		"Level 1/Level 2/Big":  {Data: []byte(strings.Repeat("a", 65536))},
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(archiveDir, "Format V4.rar"), "", expectedContent); err != nil {
		return err
	}
	// Test RAR v5 and ZIP with both Unicode BMP and non-BMP characters in file and directory names.
	expectedContent["Dir 1F601 \U0001F601/File 1F602 \U0001F602.txt"] = FileItem{Data: []byte("Char U+1F602 is \U0001F602 FACE WITH TEARS OF JOY\n")}
	expectedContent["File 1F600 \U0001F600.txt"] = FileItem{Data: []byte("Char U+1F600 is \U0001F600 GRINNING FACE\n")}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(archiveDir, "Format V5.rar"), "", expectedContent); err != nil {
		return err
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(archiveDir, "Unicode.zip"), "", expectedContent); err != nil {
		return err
	}
	return nil
}

func testUnicodeFilenamesInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	if err := verifyUnicodeArchives(ctx, cd, dataDir); err != nil {
		s.Error("Test failed: ", err)
	}
}

func testMacOSUTF8InArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"ファイル.dat": {1541735375, []byte("This is a file.\n")},
		"日本語フォルダ/新しいテキストドキュメント.txt": {1541735341, []byte("新しいテキストドキュメントです。\n")},
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(dataDir, "MacOS UTF-8 Bug 903664.zip"), "", expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

// testSJISInArchives tests that filenames encoded in Shift JIS are correctly detected and converted to UTF-8.
// https://crbug.com/846195
// https://crbug.com/834544
func testSJISInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"新しいフォルダ/SJIS_835C_ソ.txt":    {Mtime: 347068800},
		"新しいフォルダ/新しいテキスト ドキュメント.txt": {Mtime: 1002026088},
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), "", expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

func testSymlinksDisabledInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"textfile": {1357584423, []byte("sample text\n")},
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(dataDir, "Symlinks.zip"), "", expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

func testUniformEncryptionInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"Secret.txt": {Data: []byte("This is my little secret\n")},
	}
	for _, archive := range []string{
		"Encrypted Full V4.rar",
		"Encrypted Full V5.rar",
		"Encrypted Partial V4.rar",
		"Encrypted Partial V5.rar",
		"Encrypted AES-128.zip",
		"Encrypted AES-192.zip",
		"Encrypted AES-256.zip",
		"Encrypted ZipCrypto.zip",
	} {
		archivePath := filepath.Join(dataDir, archive)
		if err := verifyEncryptedArchiveContent(ctx, cd, archivePath, "password", expectedContent); err != nil {
			s.Errorf("Test failed for %q: %v", archivePath, err)
		}
	}
}

func testMixedEncryptioninArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	archivePath := filepath.Join(dataDir, "Encrypted Various.zip")
	expectedContent := DirectoryContents{
		"ClearText.txt":           {1598592138, []byte("This is not encrypted.\n")},
		"Encrypted AES-128.txt":   {1598592200, []byte("This is encrypted with AES-128.\n")},
		"Encrypted AES-192.txt":   {1598592206, []byte("This is encrypted with AES-192.\n")},
		"Encrypted AES-256.txt":   {1598592213, []byte("This is encrypted with AES-256.\n")},
		"Encrypted ZipCrypto.txt": {1598592187, []byte("This is encrypted with ZipCrypto.\n")},
	}
	if err := verifyEncryptedArchiveContent(ctx, cd, archivePath, "password", expectedContent); err != nil {
		s.Errorf("Test failed for %q: %v", archivePath, err)
	}
}

// testStrictPasswordInArchives checks that invalid password is not accidentally accepted. https://crbug.com/1127752
func testStrictPasswordInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	archivePath := filepath.Join(dataDir, "Strict Password.zip")
	if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), "password=sample", crosdisks.MountErrorNeedPassword); err != nil {
		s.Errorf("Test failed for %q: %v", archivePath, err)
	}
}

func testDuplicateFilenamesInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	mtime := int64(1600602814)
	expectedContent := DirectoryContents{
		"Simple.txt":               {mtime, []byte("Simple 1\n")},
		"Simple (1).txt":           {mtime, []byte("Simple 2 \n")},
		"Simple (2).txt":           {mtime, []byte("Simple 3  \n")},
		"Suspense...":              {mtime, []byte("Suspense 1\n")},
		"Suspense... (1)":          {mtime, []byte("Suspense 2 \n")},
		"Suspense... (2)":          {mtime, []byte("Suspense 3  \n")},
		"No Dot":                   {mtime, []byte("No Dot 1\n")},
		"No Dot (1)":               {mtime, []byte("No Dot 2 \n")},
		"No Dot (2)":               {mtime, []byte("No Dot 3  \n")},
		".Hidden":                  {mtime, []byte("Hidden 1\n")},
		".Hidden (1)":              {mtime, []byte("Hidden 2 \n")},
		".Hidden (2)":              {mtime, []byte("Hidden 3  \n")},
		"Folder/Simple.txt":        {mtime, []byte("Simple 1\n")},
		"Folder/Simple (1).txt":    {mtime, []byte("Simple 2 \n")},
		"Folder/Simple (2).txt":    {mtime, []byte("Simple 3  \n")},
		"Folder/Suspense...":       {mtime, []byte("Suspense 1\n")},
		"Folder/Suspense... (1)":   {mtime, []byte("Suspense 2 \n")},
		"Folder/Suspense... (2)":   {mtime, []byte("Suspense 3  \n")},
		"Folder/No Dot":            {mtime, []byte("No Dot 1\n")},
		"Folder/No Dot (1)":        {mtime, []byte("No Dot 2 \n")},
		"Folder/No Dot (2)":        {mtime, []byte("No Dot 3  \n")},
		"Folder/.Hidden":           {mtime, []byte("Hidden 1\n")},
		"Folder/.Hidden (1)":       {mtime, []byte("Hidden 2 \n")},
		"Folder/.Hidden (2)":       {mtime, []byte("Hidden 3  \n")},
		"With.Dot/Simple.txt":      {mtime, []byte("Simple 1\n")},
		"With.Dot/Simple (1).txt":  {mtime, []byte("Simple 2 \n")},
		"With.Dot/Simple (2).txt":  {mtime, []byte("Simple 3  \n")},
		"With.Dot/Suspense...":     {mtime, []byte("Suspense 1\n")},
		"With.Dot/Suspense... (1)": {mtime, []byte("Suspense 2 \n")},
		"With.Dot/Suspense... (2)": {mtime, []byte("Suspense 3  \n")},
		"With.Dot/No Dot":          {mtime, []byte("No Dot 1\n")},
		"With.Dot/No Dot (1)":      {mtime, []byte("No Dot 2 \n")},
		"With.Dot/No Dot (2)":      {mtime, []byte("No Dot 3  \n")},
		"With.Dot/.Hidden":         {mtime, []byte("Hidden 1\n")},
		"With.Dot/.Hidden (1)":     {mtime, []byte("Hidden 2 \n")},
		"With.Dot/.Hidden (2)":     {mtime, []byte("Hidden 3  \n")},
	}
	if err := verifyArchiveContent(ctx, cd, filepath.Join(dataDir, "Duplicate Filenames.zip"), "", expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

// copyFile copies a file. Sadly fsutil.CopyFile is unsuitable for copying into FAT filesystem. This is an adaptation of it.
func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	fi, err := sf.Stat()
	if err != nil {
		return err
	} else if !fi.Mode().IsRegular() {
		return errors.Errorf("source not regular file (mode %s)", fi.Mode())
	}

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(df, sf); err != nil {
		df.Close()
		os.Remove(df.Name())
		return err
	}
	if err := df.Close(); err != nil {
		os.Remove(df.Name())
		return err
	}

	// Try to set mode and owner, but ignore failures, as on some filesystems it always fails.
	os.Chmod(df.Name(), fi.Mode())
	if os.Geteuid() == 0 {
		st := fi.Sys().(*syscall.Stat_t)
		os.Chown(df.Name(), int(st.Uid), int(st.Gid))
	}
	return nil
}

// RunArchiveTests executes a set of tests which mount different archives using CrosDisks.
func RunArchiveTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	// Create a FAT filesystem containing all our test archive files.
	err = withLoopbackDeviceDo(ctx, cd, 64*1024*1024, "mkfs.vfat -n ARCHIVES", func(ctx context.Context, ld *crosdisks.LoopbackDevice) (err error) {
		// Mounting it through CrosDisks will put the archives where we expect users to have them, so they are already in a permitted location.
		return withMountDo(ctx, cd, ld.DevicePath(), "", "rw", func(ctx context.Context, mountPath string) error {
			s.Logf("Copying all archives to the loopback device mount %q", mountPath)
			for _, name := range PreparedArchives {
				s.Logf("Copy %q", name)
				if err := copyFile(s.DataPath(name), filepath.Join(mountPath, filepath.Base(name))); err != nil {
					return errors.Wrapf(err, "failed to copy data file %q into %q", name, mountPath)
				}
			}

			s.Run(ctx, "ValidArchives", func(ctx context.Context, state *testing.State) {
				testValidArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "InvalidArchives", func(ctx context.Context, state *testing.State) {
				testInvalidArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "Multipart", func(ctx context.Context, state *testing.State) {
				testMultipartArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "Nested", func(ctx context.Context, state *testing.State) {
				testNestedArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "Symlinks", func(ctx context.Context, state *testing.State) {
				testSymlinksDisabledInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "Unicode", func(ctx context.Context, state *testing.State) {
				testUnicodeFilenamesInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "MacOS-utf8", func(ctx context.Context, state *testing.State) {
				testMacOSUTF8InArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "SJIS", func(ctx context.Context, state *testing.State) {
				testSJISInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "UniformEncryption", func(ctx context.Context, state *testing.State) {
				testUniformEncryptionInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "MixedEncryption", func(ctx context.Context, state *testing.State) {
				testMixedEncryptioninArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "StrictPassword", func(ctx context.Context, state *testing.State) {
				testStrictPasswordInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "DuplicateFilenames", func(ctx context.Context, state *testing.State) {
				testDuplicateFilenamesInArchives(ctx, state, cd, mountPath)
			})
			return nil
		})
	})
	if err != nil {
		s.Fatal("Failed to initialize test suite: ", err)
	}
}
