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

run check_funky_with_remap ./gsort_linux_amd64 -c example/remapchr.txt example/chra.bed example/3Y21.genome 
assert_equal "$(cut -f 1 $STDOUT_FILE | perl -pe 's/\n//')" "3Y2111"

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


run check_gff_parent ./gsort_linux_amd64 --parent test.gff test.gff.genome
assert_exit_code 0
exp="##gff-version 3
##sequence-region   CHROM1 1 20386
###
###
CHROM1	Cufflinks	mRNA	1473	16154	.	-	.	ID=XLOC_228.2;description=228
CHROM1	Cufflinks	mRNA	1473	16386	.	-	.	ID=XLOC_228.3
CHROM1	Cufflinks	exon	1473	1814	.	-	.	Parent=XLOC_228.2
CHROM1	Cufflinks	exon	1473	12024	.	-	.	Parent=XLOC_228.3
CHROM1	Cufflinks	exon	11626	12574	.	-	.	Parent=XLOC_228.2
CHROM1	Cufflinks	exon	12615	12721	.	-	.	Parent=XLOC_228.3
CHROM1	Cufflinks	exon	12695	12721	.	-	.	Parent=XLOC_228.2
CHROM1	Cufflinks	exon	13637	13726	.	-	.	Parent=XLOC_228.2
CHROM1	Cufflinks	exon	13637	13726	.	-	.	Parent=XLOC_228.3
CHROM1	Cufflinks	exon	15329	15408	.	-	.	Parent=XLOC_228.2
CHROM1	Cufflinks	exon	15329	16386	.	-	.	Parent=XLOC_228.3
CHROM1	Cufflinks	exon	15994	16154	.	-	.	Parent=XLOC_228.2"

assert_equal "$(cat $STDOUT_FILE)" "$exp"


run check_vcf_like_sort ./gsort_linux_amd64 test.vcf-like.tsv test.vcf-like.genome 
assert_exit_code 0
exp="#chrom	pos	ref	alt	strand	gene_symbol	prediction	class	score
1	12623	A	C	+	DDX11L9	benign	neutral	0.386
1	12624	T	C	+	DDX11L9	possiblydamaging	deleterious	0.89
1	12625	G	A	+	DDX11L9	possiblydamaging	deleterious	0.769
1	12626	C	G	+	DDX11L9	benign	neutral	0
1	12627	C	G	+	DDX11L9	benign	neutral	0
12	78791	A	G	+	DKFZp434K1323	possiblydamaging	deleterious	0.713
12	78792	T	A	+	DKFZp434K1323	possiblydamaging	deleterious	0.932
12	78793	G	A	+	DKFZp434K1323	possiblydamaging	deleterious	0.851
12	78794	A	C	+	DKFZp434K1323	possiblydamaging	deleterious	0.895
Y	59356108	G	T	+	WASH1	benign	neutral	0.026
Y	59356110	A	G	+	WASH1	benign	neutral	0
Y	59356111	G	C	+	WASH1	possiblydamaging	deleterious	0.501
Y	59356112	C	A	+	WASH1	benign	neutral	0.003
Y	59356113	A	G	+	WASH1	benign	neutral	0.004
Y	59356113	A	C	+	WASH1	possiblydamaging	deleterious	0.952
Y	59356114	G	A	+	WASH1	possiblydamaging	deleterious	0.736"

assert_equal "$(cat $STDOUT_FILE)" "$exp"

