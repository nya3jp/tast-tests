# Test case expectations

The expectations package provides functions for tests to open and write test
expectations. These are used to modify test behavior for individual test cases
using the DUT type. This is helpful for documenting and managing triaged
"known" test failures that may happen for a particular model, build name,
board, or platform.

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
build name, board, or platform based on their file name as follows.

| FileType       | Format | Source |
|----------------|--------|--------|
| Model          | model-<model>.yml | cros\_config / name |
| BuildBoard     | build-<build>.yml | lsbrelease Board |
| Board          | board-<board>.yml | lsbrelease Board with text processing to strip the build variant ("-kernelnext" and similar) |
| gpuChipsetFile | chipset-<GPU chipset>.yml | /usr/local/graphics/hardware_probe |
| Platform       | platform-<platform>.yml | cros\_config /identity platform-name |

## Expectations file priority

`GetTestExpectation` returns the contents of the most appropriate test
expectations file on a DUT. It looks in the default directory based on the
test name.

It tries the following file names in order:
1. `base_directory/model-<model>.yml`
2. `base_directory/buildboard-<buildboard>.yml`
3. `base_directory/board-<board>.yml`
4. `base_directory/chipset-<GPU chipset>.yml`
5. `base_directory/platform-<platform>.yml`

If more than one matching file exists, only the first will be used.

To open expectations in a specific directory, use the alternate function,
`GetTestExpectationsFromDirectory`.

## Expectations files
Expectations files are YAML. They contain a serialization of the
`expectations.Expectation` structure. For a non-parameterized test, the
content is:

```
expectation: FAILURE|SKIP
tickets:
- "b/12345"
- "crbug/67890"
comments: "The test has an expectation for the following reason: ..."
sinceBuild: "R100-14526.89.0"
```

The field `tickets`, `comments`, and `sinceBuild` are informative and may be
used for logging.

For tests that are parameterized. I.e. tast.<package>.<test name>.<test case>,
the YAML structure contains a map of the test name to an expectation. For
example:

```
<package>.<test name>.<test case>:
  expectation: FAILURE|SKIP
  tickets:
  - "b/12345"
  - "crbug/67890"
  comments: "The test has an expectation for the following reason: ..."
  sinceBuild: "R100-14526.89.0"
```

If there is no key for the test, then it is expected to pass.

## Using expectations files
To use test expectations, the test writer should implement

```
expectation, err := expectations.GetTestExpectation(ctx, s)
if err != nil {
	s.Fatal("Unable to get test expectation", err)
}
expectation.HandleSkip()
defer expectation.FailForMissingErrors(s)
```

In the test code, use the expectations instance for reporting errors. I.e.

```
expectation.Error(s, "Error message", err):
expectation.Errorf(s, "Error message %v: %v", var, err):
expectation.Fatal(s, "Error message", err):
expectation.Fatalf(s, "Error message %v: %v", var, err):
```
