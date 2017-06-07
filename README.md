# gsort

[![Build Status](https://travis-ci.org/brentp/gsort.svg?branch=master)](https://travis-ci.org/brentp/gsort)
<!--
arch=amd64
for os in darwin linux windows; do
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o gsort_${os}_${arch} cmd/gsort/gsort.go
done
-->

[Binaries Available Here](https://github.com/brentp/gsort/releases)


`gsort` is a tool to sort genomic files according to a genomefile.

For example, for some reason, you may want to sort your VCF to
have order: `X,Y,2,1,3,...` and you want to **keep the header** at the top.

As a more likely example, you may want to sort your file to match GATK order
(1 ... X, Y, MT) which is not possible with any other sorting tool. With `gsort`
one can simply place MT as the last chrom in the .genome file.


Given a genome file (lines of chrom\tlength) With this tool, you can
sort a BED/VCF/GTF/... in the order dictated by that file with:

```
gsort --memory 1500 my.vcf.gz crazy.genome | bgzip -c > my.crazy-order.vcf.gz
```

where here, memory-use will be limited to 1500 megabytes.

We will use this to enforce chromosome ordering in [ggd](https://github.com/gogetdata/ggd).

It will also be useful for getting your files ready for use in **bedtools**.

# GFF parent

In GFF, the `Parent` attribute may refer to a row that would otherwise be sorted after it (based on the end position).
But, some programs require that the row referenced in a `Parent` attribute be sorted first. If this is required, used
the `--parent` flag introduced in version 0.0.6.

# Performance

gsort can sort the 2 million variants in ESP in 15 seconds. It takes a few minutes to sort
the ~10 million ExAC variants because of the huuuuge INFO strings in that file.

# Usage

`gsort` will error if your genome file has 'chr' prefix and your file does not (or vice-versa).

It will write temporary files to your $TMPDIR (usually /tmp/) as needed to avoid using too
much memory.


# TODO

+ Specify a VCF for the genome file and pull order from the @SQ tags
+ Avoid temp file when everything can fit in memory. (more universally, last chunk can always be kept in memory).

# API Documentation

--
    import "github.com/brentp/gsort"

Package gsort is a library for sorting a stream of tab-delimited lines ([]bytes)
(from a reader) using the amount of memory requested.

Instead of using a compare function as most sorts do, this accepts a
user-defined function with signature: `func(line []byte) []int` where the []ints
are used to determine ordering. For example if we were sorting on 2 columns, one
of months and another of day of months, the function would replace "Jan" with 1
and "Feb" with 2 for the first column and just return the Atoi of the 2nd
column.

#### func  Sort

```go
func Sort(rdr io.Reader, wtr io.Writer, preprocess Processor, memMB int) error
```
Sort accepts a tab-delimited io.Reader and writes to wtr using prepocess to
determine ordering

#### type Processor

```go
type Processor func(line []byte) []int
```

Processor is a function that takes a line and return a slice of ints that
determine ordering
