CAINIT_F_SHARP_3
Software version: HM-11.0

Description :
All slices are coded as I or uni-directionally predicted B slices. Each picture contains only one slice. Each slice contains 4 tiles (2 columns of tiles and 2 rows of tiles with uniform spacing). There is one picture parameter set.
cabac_init_present_flag is  equal to 1 in picture parameter set. cabac_init_flag is signaled for uni-directionally predicted B slices in the slice header referring the picture parameter set. cabac_init_flag can take on values 0 or 1.

Purpose :
To verify that the CABAC initialization type is correctly determined based on cabac_init_flag in presence of four tiles as follows:
if( slice_type = = I )
	initType =  0
if( slice_type = = B )
	initType = cabac_init_flag ? 1 : 2

