# Dumpsys maintenance guide

Rule of thumb:

*   Whenever possible, prefer Ash APIs, than using Dumpsys: Ash APIs are
    portable and don't require maintenance.
*   Dumpsys must work on all supported Android versions.
*   Prefer to parse the Protobuf output over the text one: less error prone,
    easier to maintain.

## Quick intro to Protobuf

`dumpsys` has the option to generate "text" output, and since Android P it also
has the "protobuf" output.

*   "text" is used for human readability, for developers mostly.
*   "protobuf" is [Protocol Buffers][protocol-buffers] output.

With the `--proto` parameter, `dumpsys` outputs the binary serialized protobuf
data, which is structured, so it is easy and reliable to parse. The schema is
defined in `.proto` files. The `.proto` files used by `dumpsys` are located
here:

*   [frameworks/base/core/proto][frameworks-base]

[protocol-buffers]: https://developers.google.com/protocol-buffers/
[frameworks-base]: https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/proto/

These `.proto` files can be "compiled" to generate bindings for different
languages. For example, Android uses them to generate Java bindings. In Tast, we
use them to generate Go bindings.

We use the Go-generated bindings to parse the `dumpsys` output.

## Protobuf forward compatibility

The protobuf schema of `dumpsys` is maintained to keep forward compatibility. It
means, if a field in a message is present on older Android, it will be present
on newer Android, too, with same type and same tag. However, such fields could
be deprecated. Thus, even if such fields are present in `.proto` files, actual
return value from `dumpsys` command may not include them.

When a message gets deprecated, usually a message that returns similar data is
added.

So our `DumpsysActivityActivities()` function might look like:

```go
switch SDKVersion() {
case SDKP:
    // parse field "X".
case SDKQ:
    // field "X" got deprecated on Q.
    // parse field "Y" instead.
}
```

## Debugging protobuf messages

Under normal circumstances, Protobuf should work without any issue. But if a
test reports something like the following:

    Protobuf message saved in test out directory. Filename: "activity-activities-protobuf-message-379545421.bin"
    failed to parse activity manager protobuf: proto: bad wiretype for field server.KeyguardControllerProto.KeyguardOccluded: got wiretype 2, want 0

...it means that the serialized data returned from `dumpsys` could not be parsed
with the current protobuf schema. Probably due to an incompatibility between
Android versions in the `.proto` files. This, in theory, should never happen.
But in practice you might see this error.

In any case, it is good to debug what's happening. So grab the file referenced
in the error report and do:

```sh
# Switch to the latest supported Android. And "cd" to Android root directory.
cd $ANDROID_SRC

# From there, decode the protobuf message using as input the file from the error report.
protoc --decode com.android.server.am.ActivityManagerServiceDumpActivitiesProto frameworks/base/core/proto/android/server/activitymanagerservice.proto < protobuf-message-490183657.bin
```

Similarly, if you want to decode the output of `dumpsys activity --proto
activities`, do:

```sh
# As before, make sure that you are in the correct Android repo and directory:
cd $ANDROID_SRC
adb shell dumpsys activity --proto activities | protoc --decode com.android.server.am.ActivityManagerServiceDumpActivitiesProto frameworks/base/core/proto/android/server/activitymanagerservice.proto

# For "dumpsys window" you should do:
adb shell dumpsys window --proto | protoc --decode com.android.server.wm.WindowManagerServiceDumpProto frameworks/base/core/proto/android/server/windowmanagerservice.proto
```

If the output looks incomplete, like the following:

```proto
window_configuration {
    app_bounds {
        right: 2400
        bottom: 1600
    }
    windowing_mode: 5

    # These results are incomplete. Value can be seen, but the var name not.
    # Something is wrong here!
    4 {
        3: 2400
        4: 1600
    }
}
```

...it means that there is a mismatch between the running Android and the version
from which the files were taken. If that is the case, you should update the
`.proto` files by updating
[aosp-frameworks-base-proto.ebuild][aosp-frameworks-proto].

[aosp-frameworks-proto]: http://cs/chromeos_public/src/third_party/chromiumos-overlay/chromeos-base/aosp-frameworks-base-proto/

## AOSP-frameworks-base-proto

[aosp-frameworks-base-proto.ebuild][aosp-frameworks-proto] is the ebuild file
that does:

*   Fetches the `.proto` files from a specified Android Open Source Project
    (AOSP) version.
*   "Compiles" the `.proto` files to generate Go bindings.
*   Places the generated Go bindings in the Go path so that it can be used by
    Tast tests.

If you need to upgrade the `.proto` files to a newer Android version, you need
to update the `.ebuild` file. In particular you will need to:

*   Grab the newer `.proto` from `frameworks/base/core/proto`. E.g: For
    android-10, we grab them from [here][android10-dev-proto].
*   Rename it to `aosp-frameworks-base-proto-proto-$DATE.tar.gz`.
*   Upload it to the [ChromeOS mirror][gsutil-doc]. E.g: `gsutil.py cp -n -a
    public-read aosp-frameworks-base-core-proto-20190805.tar.gz
    gs://chromeos-localmirror/distfiles/`.
*   Update the `GIT_COMMIT` in the [.ebuild file][aosp-frameworks-proto].

For reference, this CL upgraded the `.proto` files from P to Q:
https://crrev.com/c/1802618

To test it, do:

```sh
# From ChromeOS SDKROOT
sudo emerge aosp-frameworks-base-proto

# To be safe, run any Tast test that parses dumpsys protobuf output.
```

*Caveat*: The `.proto` files are taken from AOSP. If we need to start testing
Android R before it is published to AOSP, we might need to take `.proto` files
from Android internal (TBD).

[android10-dev-proto]: https://android.googlesource.com/platform/frameworks/base/+/refs/heads/android10-dev/core/proto/
[gsutil-doc]: https://chromium.googlesource.com/chromiumos/docs/+/main/archive_mirrors.md#updating-localmirror-localmirror_private-getting-files-onto-localmirror-command-line-interface
