/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.volumefilter;

import android.app.Activity;
import android.content.ClipData;
import android.content.Intent;
import android.content.res.AssetFileDescriptor;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.view.View;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.Arrays;

public class MainActivity extends Activity {
    private static final String TAG = "ArcVolumeFilterTest";

    private static final String INTENT_ACTION_OPEN_MEDIA_FILES =
            "org.chromium.arc.file_system.action.OPEN_MEDIA_FILES";
    private static final String INTENT_EXTRA_VOLUMES = "org.chromium.arc.file_system.extra.VOLUMES";

    // Keep in sync with //ash/components/arc/mojom/file_system.mojom.
    private static final String MY_FILES_VOLUME = "myfiles";
    private static final String REMOVABLE_MEDIA_VOLUME = "removable_media";
    private static final String PLAY_FILES_VOLUME = "play_files";
    private static final String GOOGLE_DRIVE_VOLUME = "google_drive";
    private static final String INVALID_VOLUME = "invalid";

    private boolean mShowMyFiles = true;
    private boolean mShowRemovableMedia = true;
    private boolean mShowPlayFiles = false;
    private boolean mShowGoogleDrive = false;

    public void onCheckBoxClicked(View view) {
        boolean isChecked = ((CheckBox) view).isChecked();
        switch (view.getId()) {
            case R.id.show_myfiles:
                mShowMyFiles = isChecked;
                break;
            case R.id.show_removable_media:
                mShowRemovableMedia = isChecked;
                break;
            case R.id.show_play_files:
                mShowPlayFiles = isChecked;
                break;
            case R.id.show_google_drive:
                mShowGoogleDrive = isChecked;
                break;
            default:
                break;
        }
    }

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final CheckBox showMyFilesButton = findViewById(R.id.show_myfiles);
        showMyFilesButton.setChecked(mShowMyFiles);

        final CheckBox showRemovableMediaButton = findViewById(R.id.show_removable_media);
        showRemovableMediaButton.setChecked(mShowRemovableMedia);

        final CheckBox showPlayFilesButton = findViewById(R.id.show_play_files);
        showPlayFilesButton.setChecked(mShowPlayFiles);

        final CheckBox showGoogleDriveButton = findViewById(R.id.show_google_drive);
        showGoogleDriveButton.setChecked(mShowGoogleDrive);

        final Button openMediaFilesButton = findViewById(R.id.open_media_files);
        openMediaFilesButton.setOnClickListener(v -> {
            openMediaFiles();
        });
    }

    private void openMediaFiles() {
        Intent intent = new Intent(INTENT_ACTION_OPEN_MEDIA_FILES);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType("*/*");
        final ArrayList<String> volumes = new ArrayList<>();
        if (mShowMyFiles) {
            volumes.add(MY_FILES_VOLUME);
        }
        if (mShowRemovableMedia) {
            volumes.add(REMOVABLE_MEDIA_VOLUME);
        }
        if (mShowPlayFiles) {
            volumes.add(PLAY_FILES_VOLUME);
        }
        if (mShowGoogleDrive) {
            volumes.add(GOOGLE_DRIVE_VOLUME);
        }
        intent.putStringArrayListExtra(INTENT_EXTRA_VOLUMES, volumes);
        intent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true);
        startActivityForResult(intent, 0);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);

        if (resultCode != RESULT_OK) {
            return;
        }

        ArrayList<Uri> uris = new ArrayList<>();
        final ClipData clipData = data.getClipData();
        if (clipData == null) {
            uris.add(data.getData());
        } else {
            final int count = clipData.getItemCount();
            for (int i = 0; i < count; i++) {
                uris.add(clipData.getItemAt(i).getUri());
            }
        }

        String mediaStoreUris = "";
        for (final Uri uri : uris) {
            mediaStoreUris += "\n" + uri.toString();
            try {
                AssetFileDescriptor fd = getContentResolver().openAssetFileDescriptor(uri, "r");
                fd.close();
            } catch (Exception e) {
                mediaStoreUris += "\n" + e.toString();
            }
        }

        TextView view = findViewById(R.id.media_store_uris);
        view.setText(mediaStoreUris);
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
    }
}
