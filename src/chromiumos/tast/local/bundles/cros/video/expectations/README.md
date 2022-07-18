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

| FileType   | Format | Source |
|------------|--------|--------|
| Model      | model-<model>.yml | cros\_config / name |
| BuildBoard | build-<build>.yml | lsbrelease Board |
| Board      | board-<board>.yml | lsbrelease Board with text processing to strip the build variant ("-kernelnext" and similar) |
| Platform   | platform-<platform>.yml | cros\_config /identity platform-name |

## Expectations file priority

`ReadTestExpectations` returns the contents of the most appropriate test
expectations file on a DUT. It looks in the default directory based on the
test name.

It tries the following file names in order:
1. `base_directory/model-<model>.yml`
2. `base_directory/buildboard-<buildboard>.yml`
3. `base_directory/board-<board>.yml`
4. `base_directory/platform-<platform>.yml`

The contents of the first of these files will be returned. If more than one
matching file exists, only the first will be read.

To open expectations in a specific directory, use the alternate function,
`ReadTestExpectationsFromDirectory`.

## Expectations structure
A test or test case expectation should utilize the `Type` enumeration to
describe the expected behavior. By default, tests are expected to PASS. The
relevant ticket string (E.g. `b/12345` or `crrbug/12345`) should be included
for every exectation file. This facilitates ease of logging and maintenance.
