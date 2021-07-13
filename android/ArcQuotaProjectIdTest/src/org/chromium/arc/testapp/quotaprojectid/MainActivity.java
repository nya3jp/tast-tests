/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.quotaprojectid;

import android.app.Activity;
import android.content.ContentValues;
import android.graphics.Bitmap;
import android.net.Uri;
import android.os.Bundle;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.OutputStream;

/**
 * Main activity for the ArcQuotaProjectIdTest app.
 *
 * <p>Used by tast test to create files.
 */
public class MainActivity extends Activity {
    public static final String TAG = "ArcQuotaProjectIdTest";

    public static final String FILE_NAME = "test.png";

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Prepare PNG image data.
        Bitmap bitmap = Bitmap.createBitmap(100, 100, Bitmap.Config.ARGB_8888);
        byte[] png = null;
        try (ByteArrayOutputStream stream = new ByteArrayOutputStream()) {
            if (!bitmap.compress(Bitmap.CompressFormat.PNG, 100, stream)) {
                Log.e(TAG, "Failed to compress the bitmap.");
                finish();
            }
            png = stream.toByteArray();
        } catch (IOException e) {
            Log.e(TAG, "Failed to compress the bitmap " + e);
            finish();
        }

        // Save the data in the external files dir.
        File file = new File(getExternalFilesDir(Environment.DIRECTORY_PICTURES), FILE_NAME);
        try (FileOutputStream stream = new FileOutputStream(file)) {
            stream.write(png);
        } catch (IOException e) {
            Log.e(TAG, "Failed to write " + e);
            finish();
        }
        Log.i(TAG, "Wrote to " + file.getPath());

        // Save the data in the primary external volume.
        ContentValues values = new ContentValues();
        values.put(MediaStore.MediaColumns.DISPLAY_NAME, FILE_NAME);

        final Uri targetUri = getContentResolver().insert(
                MediaStore.Images.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY), values);
        if (targetUri == null) {
            Log.e(TAG, "Failed to insert to the primary external volume.");
            finish();
        }
        try (OutputStream out = getContentResolver().openOutputStream(targetUri)) {
            out.write(png);
        } catch (IOException e) {
            Log.e(TAG, "Failed to write " + e);
            finish();
        }
        Log.i(TAG, "Wrote to " + targetUri);
    }
}
