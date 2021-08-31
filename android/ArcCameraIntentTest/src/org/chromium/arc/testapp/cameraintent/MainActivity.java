/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.cameraintent;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Bitmap;
import android.net.Uri;
import android.os.Bundle;
import android.provider.MediaStore;
import android.view.View;
import android.widget.Button;
import android.widget.TextView;

/** App to listen for result of camera intent. */
public class MainActivity extends Activity {
    private static final String KEY_ACTION = "action";
    private static final String KEY_DATA = "data";
    private static final String KEY_URI = "uri";
    private static final int EXPECT_NOTHING = 0;
    private static final int EXPECT_IMAGE_DATA = 1;
    private static final int EXPECT_VIDEO_URI = 2;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final Button button = findViewById(R.id.send_intent);
        button.setOnClickListener(v -> {
            sendIntent();
            button.setOnClickListener(null);
        });
    }

    @Override
    public void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);

        if (!isValidRequestCode(requestCode)) {
            throw new IllegalArgumentException("Invalid request code: " + requestCode);
        }
        if (!isValidResultCode(resultCode)) {
            throw new IllegalArgumentException("Invalid result code: " + resultCode);
        }

        String msg = Integer.toString(resultCode);

        // Checks if the result is successfully returned and the result is valid or not.
        if (resultCode == Activity.RESULT_OK) {
            if (requestCode == EXPECT_IMAGE_DATA) {
                Bitmap bitmap = data.getParcelableExtra(KEY_DATA);
                if (bitmap == null) {
                    msg = "Failed to get returned bitmap";
                } else if (bitmap.getWidth() == 0 || bitmap.getHeight() == 0) {
                    msg = "Invalid width/height of bitmap";
                }
            } else if (requestCode == EXPECT_VIDEO_URI) {
                Uri uri = data.getData();
                if (uri == null) {
                    msg = "Failed to get returned video URI";
                } else if (Uri.EMPTY.equals(uri)) {
                    msg = "URI is empty";
                }
            }
        }
        setResult(msg);
    }

    private void sendIntent() {
        final String action = getIntent().getStringExtra(KEY_ACTION);
        if (!isSupportedAction(action)) {
            throw new IllegalArgumentException("Unsupported action: " + action);
        }

        final Uri uri = getIntent().getParcelableExtra(KEY_URI);
        final Intent intent = new Intent(action);
        if (uri != null) {
            intent.putExtra(MediaStore.EXTRA_OUTPUT, uri);
        }

        if ((MediaStore.ACTION_IMAGE_CAPTURE.equals(action)
                        || MediaStore.ACTION_IMAGE_CAPTURE_SECURE.equals(action))
                && uri == null) {
            startActivityForResult(intent, EXPECT_IMAGE_DATA);
        } else if (MediaStore.ACTION_VIDEO_CAPTURE.equals(action) && uri == null) {
            startActivityForResult(intent, EXPECT_VIDEO_URI);
        } else {
            startActivityForResult(intent, EXPECT_NOTHING);
        }
    }

    private void setResult(String text) {
        final TextView textView = findViewById(R.id.text);
        textView.setText(text);
        textView.setVisibility(View.VISIBLE);
    }

    private boolean isSupportedAction(String action) {
        return MediaStore.ACTION_IMAGE_CAPTURE.equals(action)
                || MediaStore.ACTION_IMAGE_CAPTURE_SECURE.equals(action)
                || MediaStore.ACTION_VIDEO_CAPTURE.equals(action)
                || MediaStore.INTENT_ACTION_STILL_IMAGE_CAMERA.equals(action)
                || MediaStore.INTENT_ACTION_STILL_IMAGE_CAMERA_SECURE.equals(action)
                || MediaStore.INTENT_ACTION_VIDEO_CAMERA.equals(action);
    }

    private boolean isValidRequestCode(int code) {
        return code == EXPECT_NOTHING || code == EXPECT_IMAGE_DATA || code == EXPECT_VIDEO_URI;
    }

    private boolean isValidResultCode(int code) {
        return code == Activity.RESULT_OK || code == Activity.RESULT_CANCELED;
    }
}
