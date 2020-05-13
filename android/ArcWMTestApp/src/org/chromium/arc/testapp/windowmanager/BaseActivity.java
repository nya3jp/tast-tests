/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowmanager;

import android.app.ActionBar;
import android.app.Activity;
import android.app.Dialog;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.hardware.Sensor;
import android.hardware.SensorEvent;
import android.hardware.SensorEventListener;
import android.hardware.SensorManager;
import android.net.Uri;
import android.os.Bundle;
import android.os.StrictMode;
import android.text.format.DateUtils;
import android.util.Log;
import android.view.Menu;
import android.view.MenuItem;
import android.view.Surface;
import android.view.View;
import android.widget.Button;
import android.widget.TextView;

import org.json.JSONException;
import org.json.JSONStringer;

/**
 * A {@link AppCompatActivity} subclass. Base class for all activities in this application. This
 * abstract class initializes all the common widgets, such as the OptionsMenu.
 */
abstract class BaseActivity extends Activity {
    static final String EXTRA_ACTIVITY_NUMBER = "ACTIVITY_NUMBER";
    private static final String TAG = "BaseActivity";
    private static final String URI_DOCUMENTATION = "https://go/arc++-wm-verifier-doc";

    private int mActivityNumber;
    private TextView mCaptionStatusView;

    // Accelerometer related
    private SensorManager mSensorManager;
    private Sensor mAccelSensor;
    private SensorEventListener mSensorEventListener;
    private float mSensorX;
    private float mSensorY;
    private float mSensorZ;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        mActivityNumber = getIntent().getIntExtra(EXTRA_ACTIVITY_NUMBER, 1);

        final ActionBar actionBar = getActionBar();
        if (actionBar != null) {
            actionBar.setTitle(getString(R.string.app_name) + " - Activity #" + mActivityNumber);
        }

        mCaptionStatusView = (TextView) findViewById(R.id.caption_text_view);

        initAccelSensor();
    }

    @Override
    protected void onStart() {
        super.onStart();
        updateCaptionStatusView();
    }

    @Override
    protected void onResume() {
        super.onResume();
        resumeAccelSensor();
    }

    @Override
    protected void onPause() {
        super.onPause();
        pauseAccelSensor();
    }

    @Override
    public void onConfigurationChanged(Configuration newConfig) {
        super.onConfigurationChanged(newConfig);
        updateCaptionStatusView();
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        // Inflate the menu; this adds items to the action bar if it is present.
        getMenuInflater().inflate(R.menu.menu_main, menu);

        String versionName;
        int versionCode;
        // Display versionCode + versionName in Options Menu.
        try {
            PackageInfo info = getPackageManager().getPackageInfo(getPackageName(), 0);
            versionName = info.versionName;
            versionCode = info.versionCode;
        } catch (PackageManager.NameNotFoundException e) {
            Log.e(TAG, "Package versionName and/or versionCode not found. Check Android.mk", e);
            versionName = "N/A";
            versionCode = -1;
        }

        final MenuItem item = menu.findItem(R.id.action_version);
        if (item != null) {
            item.setTitle(item.getTitle() + versionName + "(" + versionCode + ")");
        }

        return true;
    }

    @Override
    public boolean onOptionsItemSelected(MenuItem item) {
        // Handle action bar item clicks here. The action bar will automatically handle clicks on
        // the Home/Up button, so long as you specify a parent activity in AndroidManifest.xml.
        int id = item.getItemId();

        if (id == R.id.action_docs) {
            Intent browserIntent = new Intent(Intent.ACTION_VIEW, Uri.parse(URI_DOCUMENTATION));
            startActivity(browserIntent);
            return true;
        } else if (id == R.id.action_accel) {
            Intent intent = new Intent(getApplicationContext(), AccelerometerPlayActivity.class);
            startActivity(intent);
            return true;
        } else if (id == R.id.action_keyboard) {
            final Dialog dialog = new Dialog(this);
            dialog.setContentView(R.layout.edit_dialog);
            dialog.setTitle(R.string.test_keyboard_plain);

            final Button dialogButton = (Button) dialog.findViewById(R.id.button_ok);
            // if button is clicked, close the custom dialog
            dialogButton.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    dialog.dismiss();
                }
            });

            dialog.show();
            return true;
        } else if (id == R.id.action_display) {
            startActivity(new Intent(getApplicationContext(), DisplayTestActivity.class));
            return true;
        } else if (id == R.id.action_crash) {
            crash();
            return true;
        } else if (id == R.id.action_strict_mode) {
            showStrictModeViolation();
            return true;
        }

        return super.onOptionsItemSelected(item);
    }

    public int getActivityNumber() {
        return mActivityNumber;
    }

    /**
     * Updates {@link #mCaptionStatusView} with the following data: - app orientation - number of
     * activities - rotation - accelerometer
     *
     * <p>That information is fetched using official APIs.
     */
    public void updateCaptionStatusView() {
        final String orientation =
                orientationModeToString(getResources().getConfiguration().orientation);
        final int rotation = rotationToInt(getWindowManager().getDefaultDisplay().getRotation());
        try {
            // This JSON string is parsed by arc.WindowManagerCUJ Tast test.
            // Don't change the format without updating the test.
            final JSONStringer js = new JSONStringer();
            js.object();

            js.key(JsonHelper.JSON_KEY_ORIENTATION).value(orientation);
            js.key(JsonHelper.JSON_KEY_ACTIVITY_NR).value(mActivityNumber);
            js.key(JsonHelper.JSON_KEY_ROTATION).value(rotation);

            js.key(JsonHelper.JSON_KEY_ACCEL);
            parseAccelSensor(js);

            js.endObject();

            mCaptionStatusView.setText(js.toString());

        } catch (JSONException e) {
            Log.w(TAG, "Could not resolve certain methods", e);
            mCaptionStatusView.setText(
                    JsonHelper.reportError(getString(R.string.only_on_chromebooks)));
        }
    }

    public void crash() {
        throw new RuntimeException("Throw an exception to cause a crash!");
    }

    public void showStrictModeViolation() {
        Runnable showViolation =
                () -> {
                    StrictMode.setThreadPolicy(
                            new StrictMode.ThreadPolicy.Builder()
                                    .detectCustomSlowCalls()
                                    .penaltyLog()
                                    .penaltyFlashScreen()
                                    .build());
                    StrictMode.noteSlowCall("StrictMode violation test");
                    try {
                        // Sleep for 1 second on the UI thread while StrictModeFlash is shown to
                        // simulate a slow call.
                        Thread.sleep(DateUtils.SECOND_IN_MILLIS);
                    } catch (InterruptedException e) {
                        // no-op
                    }
                };
        new Thread(() -> runOnUiThread(showViolation)).start();
    }

    private void initAccelSensor() {
        // Ideally we should use TYPE_ROTATION_VECTOR, but it is not present on all devices.
        // Using TYPE_ACCELEROMETER instead.
        mSensorManager = (SensorManager) getSystemService(Context.SENSOR_SERVICE);
        mAccelSensor = mSensorManager.getDefaultSensor(Sensor.TYPE_ACCELEROMETER);
        if (mAccelSensor == null) {
            Log.e(TAG, "Fatal. Could not get default TYPE_ACCELEROMETER sensor");
            return;
        }

        mSensorEventListener =
                new SensorEventListener() {
                    @Override
                    public void onSensorChanged(SensorEvent event) {
                        if (event.sensor.getType() != Sensor.TYPE_ACCELEROMETER) return;

                        mSensorX = event.values[0];
                        mSensorY = event.values[1];
                        mSensorZ = event.values[2];
                    }

                    @Override
                    public void onAccuracyChanged(Sensor sensor, int accuracy) {}
                };

        resumeAccelSensor();
    }

    private void resumeAccelSensor() {
        if (mAccelSensor != null) {
            mSensorManager.registerListener(
                    mSensorEventListener, mAccelSensor, SensorManager.SENSOR_DELAY_NORMAL);
        }
    }

    private void pauseAccelSensor() {
        if (mAccelSensor != null) {
            mSensorManager.unregisterListener(mSensorEventListener);
        }
    }

    private static String orientationModeToString(int orientationMode) {
        switch (orientationMode) {
            case Configuration.ORIENTATION_PORTRAIT:
                return "portrait";
            case Configuration.ORIENTATION_LANDSCAPE:
                return "landscape";
        }
        return "unknown";
    }

    private static int rotationToInt(int rotation) {
        switch (rotation) {
            case Surface.ROTATION_0:
                return 0;
            case Surface.ROTATION_90:
                return 90;
            case Surface.ROTATION_180:
                return 180;
            case Surface.ROTATION_270:
                return 270;
        }
        Log.w(TAG, "Invalid rotation value: " + rotation);
        return -1;
    }

    private void parseAccelSensor(JSONStringer js) {
        try {
            // Cast float to int for status report. float resolution is not needed and also
            // generates noise in the report.
            js.object();
            js.key("x");
            js.value((int) mSensorX);
            js.key("y");
            js.value((int) mSensorY);
            js.key("z");
            js.value((int) mSensorZ);
            js.endObject();
        } catch (JSONException e) {
            Log.w(TAG, "Error creating JSON object while parsing sensor data", e);
        }
    }
}
