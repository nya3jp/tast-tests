Bitstream file name: VPSID_A_VIDYO_2.hevc
Conformance point: HM-12.1
Bitstream feature:
The bitstream is testing vps_video_parameter_set_id. This bitstream contains two vps's. The correct one has the value "4". The bitstream has 3 temporal layers and the correct VPS has the temporal_nesting_flag turned off.

picture size: 416x240 (BasketballPass_416x240_50.yuv)
frame rate: 50
number of encoded frames: 33

revision 2:
	fixes an issue where a zero_byte is missing for the second VPS nal unit.