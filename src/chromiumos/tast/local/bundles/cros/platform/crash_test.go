// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

type CrashTest struct {
}

type RunCrasherParam struct {
	username   string
	causeCrash bool
}

// runCrasherProcess() runs the crasher process.
// Will wait up to 10 seconds for crash_reporter to report the crash.
// crash_reporter_caught will be marked as true when the "Received crash
// notification message..." appears. While associated logs are likely to be
// available at this point, the function does not guarantee this.
func (t *CrashTest) RunCrasherProcess(params RunCrasherParam) {
	if params.causeCrash == nil {
		params.causeCrash = true
	}
	//(self, username, cause_crash=True, consent=True,
	// 	crasher_path=None, run_crasher=None,
	// 	expected_uid=None, expected_gid=None,
	// 	expected_exit_code=None, expected_reason=None):

	// @param username: Unix user of the crasher process.
	// @param cause_crash: Whether the crasher should crash.
	// @param consent: Whether the user consents to crash reporting.
	// @param crasher_path: Path to which the crasher should be copied before
	// 	execution. Relative to |_root_path|.
	// @param run_crasher: A closure to override the default |crasher_command|
	//    invocation. It should return a tuple describing the
	//    process, where |pid| can be None if it should be
	//    parsed from the |output|:
	// def run_crasher(username, crasher_command):
	// ...
	// return (exit_code, output, pid)

	// @param expected_uid: The uid the crash happens under.
	// @param expected_gid: The gid the crash happens under.
	// @param expected_exit_code:
	// @param expected_reason:
	// Expected information in crash_reporter log message.

	// @returns:
	// A dictionary with keys:
	// returncode: return code of the crasher
	// crashed: did the crasher return segv error code
	// crash_reporter_caught: did crash_reporter catch a segv
	// output: stderr output of the crasher process
	// """
	if param.crasherPath == nil {
		// crasher_path = self._crasher_path
	} else {
		// dest = os.path.join(self._root_path,
		//     crasher_path[os.path.isabs(crasher_path):])
	}

	// utils.system('cp -a "%s" "%s"' % (self._crasher_path, dest))

	// self.enable_crash_filtering(os.path.basename(crasher_path))

	// crasher_command = []

	// if username == 'root':
	// if expected_exit_code is None:
	// expected_exit_code = -signal.SIGSEGV
	// else:
	// if expected_exit_code is None:
	// expected_exit_code = 128 + signal.SIGSEGV

	// if not run_crasher:
	// crasher_command.extend(['su', username, '-c'])

	// crasher_command.append(crasher_path)
	// basename = os.path.basename(crasher_path)
	// if not cause_crash:
	// crasher_command.append('--nocrash')
	// self._set_consent(consent)

	// logging.debug('Running crasher: %s', crasher_command)

	// if run_crasher:
	// (exit_code, output, pid) = run_crasher(username, crasher_command)

	// else:
	// crasher = subprocess.Popen(crasher_command,
	// 			  stdout=subprocess.PIPE,
	// 			  stderr=subprocess.PIPE)

	// output = crasher.communicate()[1]
	// exit_code = crasher.returncode
	// pid = None

	// logging.debug('Crasher output:\n%s', output)

	// if pid is None:
	// # Get the PID from the output, since |crasher.pid| may be su's PID.
	// match = re.search(r'pid=(\d+)', output)
	// if not match:
	// raise error.TestFail('Missing PID in crasher output')
	// pid = int(match.group(1))

	// if expected_uid is None:
	// expected_uid = pwd.getpwnam(username).pw_uid

	// if expected_gid is None:
	// expected_gid = pwd.getpwnam(username).pw_gid

	// if expected_reason is None:
	// expected_reason = 'handling' if consent else 'ignoring - no consent'

	// expected_message = (
	// ('[%s] Received crash notification for %s[%d] sig 11, user %d '
	// 'group %d (%s)') %
	// (self._expected_tag, basename, pid, expected_uid, expected_gid,
	// expected_reason))

	// # Wait until no crash_reporter is running.
	// utils.poll_for_condition(
	// lambda: utils.system('pgrep -f crash_reporter.*:%s' % basename,
	// 		ignore_status=True) != 0,
	// timeout=10,
	// exception=error.TestError(
	// 'Timeout waiting for crash_reporter to finish: ' +
	// self._log_reader.get_logs()))

	// is_caught = False
	// try:
	// utils.poll_for_condition(
	// lambda: self._log_reader.can_find(expected_message),
	// timeout=5,
	// desc='Logs contain crash_reporter message: ' + expected_message)
	// is_caught = True
	// except utils.TimeoutError:
	// pass

	// result = {'crashed': exit_code == expected_exit_code,
	// 'crash_reporter_caught': is_caught,
	// 'output': output,
	// 'returncode': exit_code}
	// logging.debug('Crasher process result: %s', result)
	// return result
}
