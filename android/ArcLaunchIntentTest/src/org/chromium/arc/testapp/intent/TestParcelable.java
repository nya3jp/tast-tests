/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.intent;

import android.os.Parcel;
import android.os.Parcelable;

public class TestParcelable implements Parcelable {
    public static final Creator<TestParcelable> CREATOR = new Creator<TestParcelable>() {
            @Override
            public TestParcelable createFromParcel(Parcel in) {
                return new TestParcelable(in);
            }

            @Override
            public TestParcelable[] newArray(int size) {
                return new TestParcelable[size];
            }
        };

    final String mText;

    public TestParcelable(String string) {
        mText = string;
    }

    protected TestParcelable(Parcel in) {
        mText = in.readString();
    }

    @Override
    public int describeContents() {
        return 0;
    }

    @Override
    public void writeToParcel(Parcel parcel, int i) {
        parcel.writeString(mText);
    }
}
