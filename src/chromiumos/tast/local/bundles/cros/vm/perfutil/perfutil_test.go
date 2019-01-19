package perfutil

import (
	"context"
	"reflect"
	"testing"

	"chromiumos/tast/local/testexec"
)

func TestHostBinaryRunnerParseLDDOutput(t *testing.T) {
	runner := HostBinaryRunner{
		Binary: "/usr/bin/some_binary",
		RunCmd: func(cmd *testexec.Cmd) ([]byte, error) {
			return nil, nil
		},
	}
	lddOutputString := `        linux-vdso.so.1 (0x00007ffc2996f000)
	libluajit-5.1.so.2 => /usr/local/lib64/libluajit-5.1.so.2 (0x00007f6be5476000)
	libm.so.6 => /lib64/libm.so.6 (0x00007f6be4d43000)
	libpthread.so.0 => /lib64/libpthread.so.0 (0x00007f6be4b25000)
	libc.so.6 => /lib64/libc.so.6 (0x00007f6be476c000)
	libdl.so.2 => /lib64/libdl.so.2 (0x00007f6be4568000)
	/lib64/ld-linux-x86-64.so.2 (0x00007f6be52cc000)`
	expectedDynLibs := map[string]string{
		"libluajit-5.1.so.2": "/usr/local/lib64/libluajit-5.1.so.2",
		"libm.so.6":          "/lib64/libm.so.6",
		"libpthread.so.0":    "/lib64/libpthread.so.0",
		"libc.so.6":          "/lib64/libc.so.6",
		"libdl.so.2":         "/lib64/libdl.so.2",
	}
	expectedDynLinker := "/lib64/ld-linux-x86-64.so.2"
	dynLibs, dynLinker := runner.parseLDDOutput(context.Background(), lddOutputString)
	if !reflect.DeepEqual(dynLibs, expectedDynLibs) {
		t.Errorf("parseLDDOutput() returns dynLibs %v, expect %v", dynLibs, expectedDynLibs)
	}

	if dynLinker != expectedDynLinker {
		t.Errorf("parseLDDOutput() returns dynLinker %q, expect %q", dynLinker, expectedDynLinker)
	}
}
