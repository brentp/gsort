# gsort

[![Build Status](https://travis-ci.org/brentp/gsort.svg?branch=master)](https://travis-ci.org/brentp/gsort)
<!--
for arch in 386 amd64; do
	for os in darwin linux windows; do
		GOOS=$os GOARCH=$arch go build -o gsort_${os}_${arch} cmd/gsort/gsort.go
	done
done
-->

gsort is a tool to sort genomic files according to a genomefile.

For example, for some reason, you may want to sort your genome to
have order: `X,Y,2,1,3,...`


Given a genome file (lines of chrom\tlength) With this tool, you can
sort it in that order with:

```
gsort --memory 1500 my.vcf.gz crazy.genome | bgzip -c > my.crazy-order.vcf.gz
```

where memory-use will be limited 1500 megabytes.

We will use this to enforce chromosome ordering in [ggd](https://github.com/gogetdata/ggd).

It will also be useful for getting your files ready for use in bedtools.

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
