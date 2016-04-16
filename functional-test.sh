#!/bin/bash

test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

set -o nounset

go build -o ./gsort_linux_amd64 cmd/gsort/gsort.go

run check_usage ./gsort_linux_amd64
assert_exit_code 255
assert_in_stderr "usage"

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


# TODO: vcf
