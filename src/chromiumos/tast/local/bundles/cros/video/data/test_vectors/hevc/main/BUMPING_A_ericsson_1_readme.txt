BUMPING_A_ericsson_1

This stream tests output order conformance, in particular the “bumping” process. Four temporal layers are used and IRAP pictures with no_output_of_prior_pics_flag equal to 1 are present in the bitstream.


All pictures with POC 0-65 are to be output except the pictures with the following POC values:
4, 5, 6, 7, 15, 21, 22, 23, 30, 31, 36, 37, 38, 39, 54, 55, 56. Those pictures are not output since they have not been output yet when IRAPs with no_output_of_prior_pics_flag equal to 1 are encountered in the bitstream.
