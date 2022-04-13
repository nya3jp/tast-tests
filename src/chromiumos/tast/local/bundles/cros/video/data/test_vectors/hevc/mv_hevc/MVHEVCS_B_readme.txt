Name: MVHEVCS_B
Specification: All slices of the base view are coded as I slices and all slices of the non-base view are coded with only inter-view prediction, thus P slices. Only the first picture of each view is coded as IDR picture. Each picture contains only one slice. NumViews is equal to 2. NumDirectRefLayers of the non-base view is equal to 1. The two views are with the same spatial resolution. All NAL units are encapsulated into the byte stream format specified in ITU-T Rec. H.265Rec. ITU-T H.265 | ISO/IEC 23008-2.
Functional stage: Decoding of two views with only inter-view prediction and intra-prediction.
Purpose: To conform the all-Intra case.

Software: HTM-15.1
