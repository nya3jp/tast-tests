CAINIT_A_SHARP_4
Software version: HM-11.0

Description :
All slices are coded as I or B slices. Each picture contains only one slice. There is one picture parameter set.
cabac_init_present_flag is  equal to 0 in picture parameter set.

Purpose :
To verify whether setting cabac_init_present_flag to 0 disables transmission of cabac_init_flag in slice header referring to the picture parameter set.
if( slice_type = = I )
	initType =  0
if(slice_type = = B )
	initType =  2

