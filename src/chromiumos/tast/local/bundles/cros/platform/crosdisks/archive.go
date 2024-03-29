// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

// PreparedArchives is a list of data files used in the test.
var PreparedArchives = []string{
	"Duplicate Filenames.zip",
	"Encrypted Full V4.rar",
	"Encrypted Full V5.rar",
	"Encrypted Partial V4.rar",
	"Encrypted Partial V5.rar",
	"Encrypted AES-128.zip",
	"Encrypted AES-192.zip",
	"Encrypted AES-256.zip",
	"Encrypted ZipCrypto.zip",
	"Encrypted Various.zip",
	"Invalid.rar",
	"Invalid.zip",
	"Format V4.rar",
	"Format V5.rar",
	"LZMA.zip",
	"Multipart Old Style.rar",
	"Multipart Old Style.r00",
	"Multipart New Style 01.rar",
	"Multipart New Style 02.rar",
	"Multipart New Style 03.rar",
	"Nested.rar",
	"Nested.zip",
	"Nested.tar.gz",
	"Smile 😀.txt.bz2",
	"Smile 😀.txt.gz",
	"Smile 😀.txt.lz",
	"Smile 😀.txt.lzma",
	"Smile 😀.txt.xz",
	"Smile 😀.txt.Z",
	"Smile 😀.txt.zst",
	"Strict Password.zip",
	"Symlinks.zip",
	"Unicode.7z",
	"Unicode.crx",
	"Unicode.iso",
	"Unicode.tZ",
	"Unicode.taZ",
	"Unicode.tar",
	"Unicode.tar.Z",
	"Unicode.tar.bz",
	"Unicode.tar.bz2",
	"Unicode.tar.gz",
	"Unicode.tar.lz",
	"Unicode.tar.lzma",
	"Unicode.tar.xz",
	"Unicode.tar.zst",
	"Unicode.tb2",
	"Unicode.tbz",
	"Unicode.tbz2",
	"Unicode.tgz",
	"Unicode.tlz",
	"Unicode.tlzma",
	"Unicode.txz",
	"Unicode.tz2",
	"Unicode.tzst",
	"Unicode.zip",
	"MacOS UTF-8 Bug 903664.zip",
	"SJIS Bug 846195.zip",
	"b1238564.gz",
}

func WithMountedArchiveDo(ctx context.Context, cd *crosdisks.CrosDisks, archivePath string, options []string, f func(ctx context.Context, mountPath string) error) error {
	return WithMountDo(ctx, cd, archivePath, filepath.Ext(archivePath), options, func(ctx context.Context, mountPath string, readOnly bool) error {
		if !readOnly {
			return errors.Errorf("unexpected read-only flag for %q: got %v; want true", mountPath, readOnly)
		}

		return f(ctx, mountPath)
	})
}

func VerifyArchiveContent(ctx context.Context, cd *crosdisks.CrosDisks, archivePath string, options []string, expectedContent DirectoryContents) error {
	return WithMountedArchiveDo(ctx, cd, archivePath, options, func(ctx context.Context, mountPath string) error {
		return verifyDirectoryContents(ctx, mountPath, expectedContent)
	})
}

func verifyEncryptedArchiveContent(ctx context.Context, cd *crosdisks.CrosDisks, archivePath, password string, expectedContent DirectoryContents) error {
	// Check that it fails without a password.
	if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), nil, crosdisks.MountErrorNeedPassword); err != nil {
		return errors.Wrap(err, "verification failed for encrypted archive without password")
	}

	// Check that it fails with a wrong password.
	for _, pw := range []string{"", password + "foo", password[0 : len(password)-1]} {
		if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), []string{"password=" + pw}, crosdisks.MountErrorNeedPassword); err != nil {
			return errors.Wrap(err, "verification failed for encrypted archive with incorrect password")
		}
	}

	// Check that it works with the right password.
	return VerifyArchiveContent(ctx, cd, archivePath, []string{"password=" + password}, expectedContent)
}

func testInvalidArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	for _, f := range []string{
		"Invalid.rar",
		"Invalid.zip",
	} {
		if err := verifyMountStatus(ctx, cd, filepath.Join(dataDir, f), filepath.Ext(f), nil, crosdisks.MountErrorMountProgramFailed); err != nil {
			s.Errorf("Unexpected status of mounting invalid archive %q: %v", f, err)
		}
	}

	for _, f := range []string{
		"Not There.rar",
		"Not There.zip",
	} {
		if err := verifyMountStatus(ctx, cd, filepath.Join(dataDir, f), filepath.Ext(f), nil, crosdisks.MountErrorInvalidPath); err != nil {
			s.Errorf("Unexpected status of mounting absent archive %q: %v", f, err)
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
		if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, archive), nil, expectedContent); err != nil {
			s.Errorf("Test failed for %q: %v", archive, err)
		}
	}
}

func testNestedArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	for _, archive := range []string{"Nested.rar", "Nested.zip", "Nested.tar.gz"} {
		expectedMountPath := filepath.Join("/media/archive", archive)
		if err := WithMountedArchiveDo(ctx, cd, filepath.Join(dataDir, archive), nil, func(ctx context.Context, mountPath string) error {
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
	want := DirectoryContents{
		"File D79F \uD79F.txt": {Data: []byte("Char U+D79F is \uD79F HANGUL SYLLABLE HIC\n")},
		" Space Oddity ":       {Data: []byte("Mind the gap\n")},
		"Level 1/Empty":        {Data: []byte{}},
		"Level 1/Digits":       {Data: []byte("0123456789")},
		"Level 1/Small":        {Data: []byte("Small file\n")},
		"Level 1/Level 2/Big":  {Data: []byte(strings.Repeat("a", 65536))},
	}

	// RAR v4 does not support full Unicode filenames.
	{
		archive := "Format V4.rar"
		archivePath := filepath.Join(archiveDir, archive)
		if err := VerifyArchiveContent(ctx, cd, archivePath, nil, want); err != nil {
			return errors.Wrapf(err, "test failed for %q", archive)
		}
	}

	// Test RAR v5 and other archive formats with both Unicode BMP and non-BMP
	// characters in file and directory names.
	want["Dir 1F601 😁/File 1F602 😂.txt"] = FileItem{Data: []byte("Char U+1F602 is 😂 FACE WITH TEARS OF JOY\n")}
	want["File 1F600 😀.txt"] = FileItem{Data: []byte("Char U+1F600 is 😀 GRINNING FACE\n")}

	for _, archive := range []string{
		"Format V5.rar",
		"Unicode.7z",
		"Unicode.crx",
		"Unicode.iso",
		"Unicode.tZ",
		"Unicode.taZ",
		"Unicode.tar",
		"Unicode.tar.Z",
		"Unicode.tar.bz",
		"Unicode.tar.bz2",
		"Unicode.tar.gz",
		"Unicode.tar.lz",
		"Unicode.tar.lzma",
		"Unicode.tar.xz",
		"Unicode.tar.zst",
		"Unicode.tb2",
		"Unicode.tbz",
		"Unicode.tbz2",
		"Unicode.tgz",
		"Unicode.tlz",
		"Unicode.tlzma",
		"Unicode.txz",
		"Unicode.tz2",
		"Unicode.tzst",
		"Unicode.zip",
	} {
		archivePath := filepath.Join(archiveDir, archive)
		if err := VerifyArchiveContent(ctx, cd, archivePath, nil, want); err != nil {
			return errors.Wrapf(err, "test failed for %q", archive)
		}
	}

	// Test single-file archives.
	want = DirectoryContents{
		"Smile 😀.txt": {Data: []byte("Don't forget to smile 😀!\n")},
	}
	for _, archive := range []string{
		"Smile 😀.txt.bz2",
		"Smile 😀.txt.gz",
		"Smile 😀.txt.lz",
		"Smile 😀.txt.lzma",
		"Smile 😀.txt.xz",
		"Smile 😀.txt.zst",
		"Smile 😀.txt.Z",
	} {
		archivePath := filepath.Join(archiveDir, archive)
		if err := VerifyArchiveContent(ctx, cd, archivePath, nil, want); err != nil {
			return errors.Wrapf(err, "test failed for %q", archive)
		}
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
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "MacOS UTF-8 Bug 903664.zip"), nil, expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

// testSJISInArchives tests that filenames encoded in Shift JIS are correctly
// detected and converted to UTF-8.
// https://crbug.com/846195
// https://crbug.com/834544
// https://crbug.com/1287893
func testSJISInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"新しいフォルダ/SJIS_835C_ソ.txt":    {Mtime: 347068800},
		"新しいフォルダ/新しいテキスト ドキュメント.txt": {Mtime: 1002026088},
	}
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), nil, expectedContent); err != nil {
		s.Error("Test failed without encoding: ", err)
	}

	// Check that passed encoding is taken in account.
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), []string{"encoding=Shift_JIS"}, expectedContent); err != nil {
		s.Error("Test failed with encoding=Shift_JIS: ", err)
	}

	// Check that if the passed encoding is wrong, we get garbled filenames.
	// Convert from Code Page 866.
	expectedContent = DirectoryContents{
		"РVВ╡ВвГtГHГЛГ_/SJIS_835C_Г\\.txt":               {Mtime: 347068800},
		"РVВ╡ВвГtГHГЛГ_/РVВ╡ВвГeГLГXГg ГhГLГЕГБГУГg.txt": {Mtime: 1002026088},
	}
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), []string{"encoding=cp866"}, expectedContent); err != nil {
		s.Error("Test failed with encoding=cp866: ", err)
	}
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), []string{"encoding=IBM866"}, expectedContent); err != nil {
		s.Error("Test failed with encoding=IBM866: ", err)
	}

	// Using the special "libzip" encoding instructs mount-zip to use libzip's
	// encoding detection and conversion. In this case, it considers that the
	// filenames are in Code Page 437.
	expectedContent = DirectoryContents{
		"ÉVé╡éóâtâHâïâ_/SJIS_835C_â\\.txt":               {Mtime: 347068800},
		"ÉVé╡éóâtâHâïâ_/ÉVé╡éóâeâLâXâg âhâLâàâüâôâg.txt": {Mtime: 1002026088},
	}
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "SJIS Bug 846195.zip"), []string{"encoding=libzip"}, expectedContent); err != nil {
		s.Error("Test failed with encoding=libzip: ", err)
	}
}

func testSymlinksDisabledInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	expectedContent := DirectoryContents{
		"textfile": {1357584423, []byte("sample text\n")},
	}
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "Symlinks.zip"), nil, expectedContent); err != nil {
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

// testStrictPasswordInArchives checks that invalid password is not accidentally
// accepted (https://crbug.com/1127752).
func testStrictPasswordInArchives(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	archivePath := filepath.Join(dataDir, "Strict Password.zip")
	if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), []string{"password=sample"}, crosdisks.MountErrorNeedPassword); err != nil {
		s.Errorf("Test failed for %q: %v", archivePath, err)
	}
}

// testUnsupportedCompressionMethod checks that a ZIP containing a file with an
// unsupported compression method is not accepted (https://crbug.com/1360291).
func testUnsupportedCompressionMethod(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	archivePath := filepath.Join(dataDir, "LZMA.zip")
	if err := verifyMountStatus(ctx, cd, archivePath, filepath.Ext(archivePath), nil, crosdisks.MountErrorMountProgramFailed); err != nil {
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
	if err := VerifyArchiveContent(ctx, cd, filepath.Join(dataDir, "Duplicate Filenames.zip"), nil, expectedContent); err != nil {
		s.Error("Test failed: ", err)
	}
}

func testCancellation(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	// Set MountCompleted event watcher.
	watcher, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Fatal("Cannot watch mount completed events: ", err)
	}

	defer watcher.Close(ctx)

	archivePath := filepath.Join(dataDir, "b1238564.gz")

	// Use a short timeout of 2 seconds while mounting the archive.
	ctxForMounting, close := context.WithTimeout(ctx, time.Second*2)
	defer close()
	if err = cd.Mount(ctxForMounting, archivePath, ".gz", []string{}); err != nil {
		s.Fatalf("Cannot mount %q: %v", archivePath, err)
	}

	// This is a slow mounter, and we shouldn't get the mount completed signal
	// within the 2-second timeout. Instead we should get a deadline exceeded
	// error.
	if _, err = watcher.Wait(ctxForMounting); !errors.Is(err, context.DeadlineExceeded) {
		s.Errorf("Unexpected error: got %v want %v", err, context.DeadlineExceeded)
	}

	// Use a short timeout of 2 seconds while unmounting.
	ctxForUnmounting, close2 := context.WithTimeout(ctx, time.Second*2)
	defer close2()

	// Unmounting by passing the original archive path should cancel the mount
	// operation in progress.
	if err := cd.Unmount(ctxForUnmounting, archivePath, []string{}); err != nil {
		s.Fatalf("Cannot unmount %q: %v", archivePath, err)
	}

	// Wait for MountCompleted event.
	event, err := watcher.Wait(ctxForUnmounting)
	if err != nil {
		s.Fatal("Cannot wait for MountCompleted event: ", err)
	}

	// The MountCompleted event should indicate a cancellation.
	if event.Status != crosdisks.MountErrorCancelled {
		s.Errorf(
			"Unexpected mount status for %q: got %v, want %v",
			archivePath, event.Status, crosdisks.MountErrorCancelled)
	}
}

// CopyFile copies a file. Sadly fsutil.CopyFile is unsuitable for copying into
// FAT filesystem. This is an adaptation of it.
func CopyFile(src, dst string) error {
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

	// Try to set mode and owner, but ignore failures, as on some filesystems it
	// always fails.
	os.Chmod(df.Name(), fi.Mode())
	if os.Geteuid() == 0 {
		st := fi.Sys().(*syscall.Stat_t)
		os.Chown(df.Name(), int(st.Uid), int(st.Gid))
	}
	return nil
}

// RunArchiveTests executes a set of tests which mount different archives using
// CrosDisks.
func RunArchiveTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Cannot connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	// Create a FAT filesystem containing all our test archive files.
	err = WithLoopbackDeviceDo(ctx, cd, 512*1024*1024, "mkfs.vfat -n ARCHIVES", func(ctx context.Context, ld *crosdisks.LoopbackDevice) (err error) {
		// Mounting it through CrosDisks will put the archives where we expect users
		// to have them, so they are already in a permitted location.
		return WithMountDo(ctx, cd, ld.DevicePath(), "", []string{"rw"}, func(ctx context.Context, mountPath string, readOnly bool) error {
			if readOnly {
				return errors.Errorf("unexpected read-only flag for %q: got %v; want false", mountPath, readOnly)
			}

			s.Logf("Copying archives to loopback device mounted at %q", mountPath)
			for _, name := range PreparedArchives {
				s.Logf("Copying %q to %q", name, mountPath)
				if err := CopyFile(s.DataPath(name), filepath.Join(mountPath, filepath.Base(name))); err != nil {
					return errors.Wrapf(err, "cannot copy file %q into %q", name, mountPath)
				}
			}

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
			s.Run(ctx, "UnsupportedCompressionMethod", func(ctx context.Context, state *testing.State) {
				testUnsupportedCompressionMethod(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "DuplicateFilenames", func(ctx context.Context, state *testing.State) {
				testDuplicateFilenamesInArchives(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "CancelMounting", func(ctx context.Context, state *testing.State) {
				testCancellation(ctx, state, cd, mountPath)
			})
			return nil
		})
	})

	if err != nil {
		s.Fatal("Error while running tests: ", err)
	}
}
