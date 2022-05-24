package arc

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

func verifyDiagnose(t *testing.T, logcat, exp string) {
	t.Helper()

	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	tf := filepath.Join(td, "logcat.txt")
	if err := ioutil.WriteFile(tf, []byte(logcat), 0644); err != nil {
		t.Fatal(err)
	}

	msg := diagnose(tf, errors.New("failed")).Error()
	if msg != exp {
		t.Fatalf("diagnose returned %q; want %q", msg, exp)
	}
}

func TestDiagnoseBuildInCrash(t *testing.T) {
	const logcat = `
--------- beginning of crash
11-14 17:04:58.241    68   128 F libc    : Fatal signal 11 (SIGSEGV), code 1, fault addr 0x8 in tid 128 (Binder:68_1)
11-14 17:04:58.279   496   638 E ActivityThread: Failed to find provider info for com.google.android.apps.work.clouddpc.arc.provider
11-14 17:04:58.311   718   718 F DEBUG   : *** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
11-14 17:04:58.311   718   718 F DEBUG   : Build fingerprint: 'google/bob/bob_cheets:7.1.1/R72-11259.0.0/5126437:user/release-keys'
11-14 17:04:58.311   718   718 F DEBUG   : Revision: '0'
11-14 17:04:58.311   718   718 F DEBUG   : ABI: 'arm'
11-14 17:04:58.311   718   718 F DEBUG   : pid: 68, tid: 128, name: Binder:68_1  >>> /system/bin/mediaserver <<<
11-14 17:04:58.440   718   718 F DEBUG   : signal 11 (SIGSEGV), code 1 (SEGV_MAPERR), fault addr 0x8
11-14 17:04:58.440   718   718 F DEBUG   :     r0 ead11000  r1 42ee9ee7  r2 00000000  r3 ec517348
11-14 17:04:58.440   718   718 F DEBUG   :     r4 00000000  r5 00000000  r6 eb0dd5cc  r7 534c676c
11-14 17:04:58.440   718   718 F DEBUG   :     r8 eaed56fc  r9 00000000  sl eaed57ec  fp 00410001
11-14 17:04:58.440   718   718 F DEBUG   :     ip ed5f2860  sp eaed56b0  lr ed5bdfa3  pc ec5bb9aa  cpsr 600f0030
`
	const exp = "Android failed to boot (mediaserver crashed): failed"

	verifyDiagnose(t, logcat, exp)
}

func TestDiagnoseSystemServerCrash(t *testing.T) {
	const logcat = `
--------- beginning of crash
04-18 07:31:54.851   129   129 E AndroidRuntime: *** FATAL EXCEPTION IN SYSTEM PROCESS: main
04-18 07:31:54.851   129   129 E AndroidRuntime: java.lang.Error: No workspace configuration on boot!
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.am.ActivityManagerInjectorArc.onBootPhase(ActivityManagerInjectorArc.java:603)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.am.ActivityManagerService$Lifecycle.onBootPhase(ActivityManagerService.java:2944)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.SystemServiceManager.startBootPhase(SystemServiceManager.java:155)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.SystemServer.startBootstrapServices(SystemServer.java:611)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.SystemServer.run(SystemServer.java:434)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.server.SystemServer.main(SystemServer.java:299)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at java.lang.reflect.Method.invoke(Native Method)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:493)
04-18 07:31:54.851   129   129 E AndroidRuntime: 	at com.android.internal.os.ZygoteInit.main(ZygoteInit.java:845)
`
	const exp = "Android failed to boot (system_server crashed): failed"

	verifyDiagnose(t, logcat, exp)
}

func TestDiagnoseNoCrash(t *testing.T) {
	const logcat = `
--------- beginning of system
11-14 17:04:41.785     6     6 I vold    : Vold 3.0 (the awakening) firing up
11-14 17:04:41.785     6     6 V vold    : Detected support for: ext4
11-14 17:04:41.818     6    16 D vold    : e4crypt_init_user0
11-14 17:04:41.819     6    16 D vold    : e4crypt_prepare_user_storage for volume null, user 0, serial 0, flags 1
11-14 17:04:41.819     6    16 D vold    : Preparing: /data/system/users/0
11-14 17:04:41.819     6    16 E vold    : Failed to prepare /data/system/users/0: No such file or directory
11-14 17:04:41.819     6    16 E vold    : Failed to prepare user 0 storage
11-14 17:04:49.100     6    16 D vold    : e4crypt_init_user0
11-14 17:04:49.101     6    16 D vold    : e4crypt_prepare_user_storage for volume null, user 0, serial 0, flags 1
`
	const exp = "Android failed to boot: failed"

	verifyDiagnose(t, logcat, exp)
}

func TestDiagnoseNoLogcat(t *testing.T) {
	const exp = "Android failed to boot (logcat unavailable): failed"

	msg := diagnose("/no-such-file", errors.New("failed")).Error()
	if msg != exp {
		t.Fatalf("diagnose returned %q; want %q", msg, exp)
	}
}
