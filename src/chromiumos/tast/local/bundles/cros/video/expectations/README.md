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

## Expectations file names

The test expectations files themselves are YAML and are named such that they
matched a particular DUT type. They can be specified for a particular model,
build name, board, or platform based on their file name as follows.

| FileType | Format | Source |
|----------|--------|--------|
| Model    | model-<model>.yml | cros\_config / name |
| Build    | build-<build>.yml | lsbrelease Board |
| Board    | board-<board>.yml | lsbrelease Board with text processing to strip the build variant ("-kernelnext" and similar) |
| Platform | platform-<platform>.yml | cros\_config /identity platform-name |

## Expectations structure
A test or test case expectation should utilize the expectations.Type
enumeration to describe the expected behavior. By default, tests are expected
to PASS. The ticket string should be included per file. This facilitates
ease of logging and maintenance.
