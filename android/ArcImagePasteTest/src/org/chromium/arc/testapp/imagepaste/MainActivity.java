/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.imagepaste;

import android.app.Activity;
import android.content.Context;
import android.graphics.BitmapFactory;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.widget.ImageView;
import android.widget.TextView;

import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.InputStream;

public class MainActivity extends Activity implements ImageEditText.Listener {
    private static final String TAG = "ArcImagePasteTest";

    private TextView mCounterView;
    private ImageView mImageView;
    private ImageEditText mInputField;

    private int mCounter = 0;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mCounterView = findViewById(R.id.counter);
        mImageView = findViewById(R.id.image_view);
        mInputField = findViewById(R.id.input_field);
        mInputField.setListener(this);
    }

    @Override
    public void onContentUri(Uri uri) {
        try (InputStream in = getContentResolver().openInputStream(uri)) {
            mImageView.setImageBitmap(BitmapFactory.decodeStream(in));
            mImageView.invalidate();
        } catch (FileNotFoundException e) {
            Log.e(TAG, "FileNotFoundException", e);
            return;
        } catch (IOException e) {
            Log.e(TAG, "IOException", e);
            return;
        } catch (SecurityException e) {
            Log.e(TAG, "SecurityException ", e);
            return;
        }
        ++mCounter;
        mCounterView.setText(Integer.toString(mCounter));
    }
}
