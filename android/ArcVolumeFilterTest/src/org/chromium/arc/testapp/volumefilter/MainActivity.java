/*
 * Copyright 2022 The ChromiumOS Authors.
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

    private static final String INTENT_ACTION_OPEN_MEDIA_STORE_FILES =
            "org.chromium.arc.file_system.action.OPEN_MEDIA_STORE_FILES";

    private static final String INTENT_EXTRA_FILE_NAME =
            "org.chromium.arc.file_system.extra.FILE_NAME";

    private static final int RESULT_URI_CONVERSION_ERROR = 100;

    private boolean mAllowMultiple = true;

    public void onCheckBoxClicked(View view) {
        boolean isChecked = ((CheckBox) view).isChecked();
        switch (view.getId()) {
            case R.id.allow_multiple:
                mAllowMultiple = isChecked;
                break;
            default:
                break;
        }
    }

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final CheckBox allowMultipleButton = findViewById(R.id.allow_multiple);
        allowMultipleButton.setChecked(mAllowMultiple);

        final Button openMediaFilesButton = findViewById(R.id.open_media_store_files);
        openMediaFilesButton.setOnClickListener(v -> {
            openMediaFiles();
        });
    }

    private void openMediaFiles() {
        Intent intent = new Intent(INTENT_ACTION_OPEN_MEDIA_STORE_FILES);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType("*/*");
        intent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, mAllowMultiple);
        startActivityForResult(intent, 0);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);

        TextView view = findViewById(R.id.media_store_uris);

        if (resultCode == RESULT_CANCELED) {
                view.setText("CANCELED");
            return;
        }

        if (resultCode == RESULT_URI_CONVERSION_ERROR) {
                final String fileName = data.getStringExtra(INTENT_EXTRA_FILE_NAME);
                view.setText("Failed to get MediaStore URI for " + fileName);
            return;
        }

        if (resultCode != RESULT_OK) {
            view.setText("Unexpected result code: " + resultCode);
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
            mediaStoreUris += uri.toString() + "\n";
            try {
                AssetFileDescriptor fd = getContentResolver().openAssetFileDescriptor(uri, "r");
                fd.close();
            } catch (Exception e) {
                mediaStoreUris += e.toString() + "\n";
            }
        }

        view.setText(mediaStoreUris);
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
    }
}
