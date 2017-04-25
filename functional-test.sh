#!/bin/bash

test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

set -o nounset

go build -o ./gsort_linux_amd64 cmd/gsort/gsort.go

run check_usage ./gsort_linux_amd64
assert_exit_code 255
assert_in_stderr "Usage"

run check_funky ./gsort_linux_amd64 example/a.bed example/3Y21.genome 
assert_exit_code 0

assert_equal "$(cut -f 1 $STDOUT_FILE | perl -pe 's/\n//')" "3Y2111"
assert_equal "$(cut -f 2 $STDOUT_FILE | perl -pe 's/\n//')" "12342224233145567556"
assert_equal "$(cut -f 3 $STDOUT_FILE | perl -pe 's/\n//')" "12353335555255668566"


run check_normal ./gsort_linux_amd64 example/a.bed example/123Y.genome 
assert_exit_code 0
exp="1	1	2
1	4556	5566
1	7556	8566
2	4233	5555
3	1234	1235
Y	222	333"

assert_equal "$(cat $STDOUT_FILE)" "$exp"


run check_bed_header ./gsort_linux_amd64 example/track-browser.bed example/123Y.genome
assert_exit_code 0
exp="track name=\"ItemRGBDemo\" description=\"Item RGB demonstration\" visibility=2 itemRgb=\"On\"
browser position chr7:127471196-127495720
browser hide all
1	127473530	127474697	Pos3	0	+	127473530	127474697	255,0,0
1	127474697	127475864	Pos4	0	+	127474697	127475864	255,0,0
1	127478198	127479365	Neg3	0	-	127478198	127479365	0,0,255
1	127479365	127480532	Pos5	0	+	127479365	127480532	255,0,0
1	127480532	127481699	Neg4	0	-	127480532	127481699	0,0,255
2	127475864	127477031	Neg1	0	-	127475864	127477031	0,0,255
2	127477031	127478198	Neg2	0	-	127477031	127478198	0,0,255
Y	127471196	127472363	Pos1	0	+	127471196	127472363	255,0,0
Y	127472363	127473530	Pos2	0	+	127472363	127473530	255,0,0"
assert_equal "$(cat $STDOUT_FILE)" "$exp"

# TODO: vcf
