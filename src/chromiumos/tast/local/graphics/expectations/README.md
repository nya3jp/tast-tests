# Test case expectations

The expectations package provides functions for tests to open and write test
expectations. These are used to modify test behavior for individual test cases
using the DUT type. This is helpful for documenting and managing triaged
"known" test failures that may happen for a particular model, build name,
board, or GPU chipset.

## Expectations folder

The expectations files can be found in the [graphics expectations folder](https://chromium.googlesource.com/chromiumos/platform/graphics/+/refs/heads/main/expectations/)
and are deployed to the DUT at `/usr/local/graphics/expectations`. Within this
directory, test expectations directories are organized by test name. An
expectation for tast.video.PlatformDecoding will, by default, be in the
directory `/usr/local/graphics/expectations/tast/video/PlatformDecoding/`.

For parameterized tests, only "main" test name is used. The expectations file
may contain an expectation for each parameterized test as needed.

## Expectations file names

The test expectations files themselves are YAML and are named such that they
matched a particular DUT type. They can be specified for a particular model,
build name, board, or GPU chipset based on their file name as follows.

| FileType       | Format | Source |
|----------------|--------|--------|
| model          | model-<model>.yml | cros\_config / name |
| buildBoard     | build-<build>.yml | lsbrelease Board |
| board          | board-<board>.yml | lsbrelease Board with text processing to strip the build variant ("-kernelnext" and similar) |
| gpuChipset     | chipset-<GPU chipset>.yml | /usr/local/graphics/hardware\_probe |
| allDevices     | all.yml | N/A |

## Expectations file priority

`GetTestExpectation` returns the contents of the most appropriate test
expectations file on a DUT. It looks in the default directory based on the
test name.

It tries the following file names in order:
1. `base_directory/model-<model>.yml`
2. `base_directory/buildboard-<buildboard>.yml`
3. `base_directory/board-<board>.yml`
4. `base_directory/chipset-<GPU chipset>.yml`
5. `base_directory/all.yml`

If more than one matching file exists, only the first will be used.

To open expectations in a specific directory, use the alternate function,
`GetTestExpectationsFromDirectory`.

## Expectations files
Expectations files are YAML.  The file structure is a map from the test case
name to a serilization of the `expectations.Expectation` structure. For
parameterized tests, there may be many keys for different test cases. For
non-parameterized tests, there will be at most one key.

For example:

```
<package>.<test name>.<test case>:
  expectation: FAIL
  tickets:
  - "b/12345"
  - "crbug/67890"
  comments: "The test has an expectation for the following reason: ..."
  sinceBuild: "R100-14526.89.0"
```

The field `tickets`, `comments`, and `sinceBuild` are informative and may be
used for logging.

If there is no key for the test, then it is expected to pass.

## Using expectations files
To use test expectations, the test writer should implement

```
expectation, err := expectations.GetTestExpectation(ctx, s.TestName())
if err != nil {
	s.Fatal("Unable to get test expectation", err)
}
// Schedules a post-test expectations handling. If the test is expected to
// fail, but did not, then this generates an error.
defer func() {
	if err := expectation.HandleFinalExpectation(); err != nil {
		s.Error("Unmet expectation: ", err)
	}
}()
```

If a test passes when it is expected to fail, then `FailForMissingErrors` will
cause the test to fail with a message containing the tickets provided in the
`Expectations` structure.

In the test code, make the following substitutions.

Replace `s.Error(args...)` with

```
if expErr := expectation.ReportError(args...); expErr != nil {
	s.Error("Unexpected error: ", expErr)
}
```

Replace `s.Errorf(formatString, args...)` with

```
if expErr := expectation.ReportErrorf(formatString, args...); expErr != nil {
	s.Error("Unexpected error: ", expErr)
}
```

Replace `s.Fatal(args...)` with

```
if expErr := expectation.ReportError(args...); expErr != nil {
	s.Fatal("Unexpected error: ", expErr)
}
return
```

Replace `s.Fatalf(formatString, args...)` with

```
if expErr := expectation.ReportErrorf(formatString, args...); expErr != nil {
	s.Fatal("Unexpected error: ", expErr)
}
return
```

Note: it is up to the test code to stop the test for "expected" fatal errors.
In the above examples, `return` is used to stop the test function.
