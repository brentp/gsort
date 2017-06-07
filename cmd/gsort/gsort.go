package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/alexflint/go-arg"
	"github.com/brentp/gsort"
	"github.com/brentp/xopen"
	"github.com/gogetdata/ggd-utils"
)

// DEFAULT_MEM is the number of megabytes of mem to use.
var DEFAULT_MEM = 1300

// VERSION is the program version number
const VERSION = "0.0.6"

var FileCols map[string][]int = map[string][]int{
	"BED": []int{0, 1, 2},
	"VCF": []int{0, 1, -1},
	"GFF": []int{0, 3, -1, 4},
	"GTF": []int{0, 3, -1, 4},
}

var CHECK_ORDER = []string{"BED", "GTF"}

var args struct {
	Path   string `arg:"positional,help:a tab-delimited file to sort"`
	Genome string `arg:"positional,help:a genome file of chromosome sizes and order"`
	Memory int    `arg:"-m,help:megabytes of memory to use before writing to temp files."`
	Parent bool   `arg:"-p,help:for gff only. given rows with same chrom and start put those with a 'Parent' attribute first"`
}

func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// get the start and end of a column given the index
func getAt(line []byte, idx int) (int, int) {
	if idx == 0 {
		return 0, bytes.IndexRune(line, '\t')
	}
	off := 0
	for i := 0; i < idx; i++ {
		off += 1 + bytes.IndexRune(line[off:], '\t')
	}
	e := bytes.IndexRune(line[off:], '\t')
	if e == -1 {
		e = len(line)
		for line[e-1] == '\n' || line[e-1] == '\r' {
			e--
		}
	} else {
		e = off + e
	}
	return off, e
}

// the last function is used when a column is -1
func sortFnFromCols(cols []int, gf *ggd_utils.GenomeFile, getter endGetter) func([]byte) []int {
	m := 0
	for _, c := range cols {
		if c > m {
			m = c
		}
	}
	m += 2
	if getter != nil && m < 6 {
		m = 6
	}
	H := 0 // keep order of header
	fn := func(line []byte) []int {
		l := make([]int, len(cols))
		s, e := getAt(line, cols[0])
		var ok bool
		if s < 0 || e < 0 {
			ok = false
		} else {
			l[0], ok = gf.Order[string(line[s:e])]
		}
		if !ok {
			if line[0] == '#' || hasAnyHeader(string(line)) {
				H++
				return []int{gsort.HEADER_LINE, H}
			}
			log.Fatalf("unknown chromosome: %s (known: %v)", line[s:e], gf.Order)
		}
		for k, col := range cols[1:] {
			i := k + 1
			if col == -1 {
				l[i] = getter(l[i-1], line)
			} else {
				s, e := getAt(line, col)
				subset := line[s:e]
				v, err := strconv.Atoi(unsafeString(subset))
				if err != nil {
					log.Fatal(err)
				}
				l[i] = v
			}
		}
		return l
	}
	return fn
}

var allowedHeaders = []string{"browser", "track"}

func hasAnyHeader(line string) bool {
	for _, a := range allowedHeaders {
		if strings.HasPrefix(line, a) {
			return true
		}
	}
	return false
}

func sniff(rdr *bufio.Reader) (string, *bufio.Reader, error) {
	lines := make([]string, 0, 200)
	var ftype string
	for len(lines) < 50000 {
		line, err := rdr.ReadString('\n')
		if len(line) > 0 {
			lines = append(lines, line)
			if line[0] == '#' {
				if strings.HasPrefix(line, "##fileformat=VCF") || strings.HasPrefix(line, "#CHROM\tPOS\tID") {
					ftype = "VCF"
					break
				} else {
					continue
				}
			} else {
				toks := strings.Split(line, "\t")
				if len(toks) < 3 {
					if hasAnyHeader(string(line)) {
						continue
					}
					return "", nil, fmt.Errorf("file has fewer than 3 columns")
				}
				for _, t := range CHECK_ORDER {
					cols := FileCols[t]
					ok := true
					last := 0
					for _, c := range cols[1:] {
						if c == -1 {
							continue
						}
						if c >= len(toks) {
							ok = false
							break
						}
						v, err := strconv.Atoi(strings.TrimRight(toks[c], "\r\n"))
						if err != nil {
							ok = false
							break
						}
						// check that 0 <= start col <= end_col
						if v < last {
							ok = false
							break
						}
						last = v
					}
					if ok {
						ftype = t
						break
					}
				}
				if hasAnyHeader(string(line)) {
					continue
				}
				if ftype == "" {
					return "", nil, fmt.Errorf("unknown file format: %s", string(line))
				}
				break
			}
		}
		if err != nil {
			return "", nil, err
		}

	}
	nrdr := io.MultiReader(strings.NewReader(strings.Join(lines, "")), rdr)
	return ftype, bufio.NewReader(nrdr), nil
}

func find(key []byte, info []byte) (int, int) {
	l := len(key)
	if pos := bytes.Index(info, key); pos != -1 {
		var end int
		for end = pos + l + 1; end < len(info); end++ {
			if info[end] == ';' {
				break
			}
		}
		return pos + l, end
	}
	return -1, -1

}

func getMax(i []byte) (int, error) {
	if !bytes.Contains(i, []byte(",")) {
		return strconv.Atoi(unsafeString(i))
	}
	all := bytes.Split(i, []byte{','})
	max := -1
	for _, b := range all {
		v, err := strconv.Atoi(unsafeString(b))
		if err != nil {
			return max, err
		}
		if v > max {
			max = v
		}
	}
	return max, nil
}

type endGetter func(start int, line []byte) int

var vcfEndGetter = endGetter(func(start int, line []byte) int {

	col4s, col4e := getAt(line, 4)
	col4 := line[col4s:col4e]
	if bytes.Contains(col4, []byte{'<'}) && (bytes.Contains(col4, []byte("<DEL")) ||
		bytes.Contains(col4, []byte("<DUP")) ||
		bytes.Contains(col4, []byte("<INV")) ||
		bytes.Contains(col4, []byte("<CN"))) {
		// need to look at INFO for this.

		is, ie := getAt(line, 7)
		info := line[is:ie]
		if s, e := find([]byte("END="), info); s != -1 {
			end, err := getMax(info[s:e])
			if err != nil {
				log.Fatal(err)
			}
			return end
		}
		s, e := find([]byte("SVLEN="), info)
		if s == -1 {
			log.Printf("warning: cant find end for %s", string(line))
			s3, e3 := getAt(line, 3)
			return start + e3 - s3
		}
		svlen, err := getMax(info[s:e])
		if err != nil {
			log.Fatal(err)
		}
		return start + svlen

	}
	// length of reference.
	s3, e3 := getAt(line, 3)
	return start + e3 - s3

})

func main() {

	args.Memory = DEFAULT_MEM
	p := arg.MustParse(&args)
	fmt.Fprintf(os.Stderr, "> gsort version %s\n", VERSION)
	if args.Path == "" || args.Genome == "" {
		p.Fail("must specify a tab-delimited file and a genome file")
	}

	rdr, err := xopen.Ropen(args.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer rdr.Close()

	ftype, brdr, err := sniff(rdr.Reader)
	if err != nil {
		log.Fatal(err)
	}

	gf, err := ggd_utils.ReadGenomeFile(args.Genome)
	if err != nil {
		log.Fatal(err)
	}
	var getter endGetter

	if ftype == "VCF" {
		getter = vcfEndGetter
	} else if args.Parent && (ftype == "GFF" || ftype == "GTF") {

		seen := make(map[string]int, 20)
		cnt := 2
		getter = endGetter(func(start int, line []byte) int {
			ix := bytes.Index(line, []byte("\tID="))
			if ix == -1 {
				ix = bytes.Index(line, []byte(";ID="))
			}
			if ix != -1 {
				ix += 4
				ixEnd := bytes.IndexByte(line[ix:], ';')
				if ixEnd == -1 {
					seen[string(line[ix:len(line)-1])] = cnt
				} else {
					seen[string(line[ix:ix+ixEnd])] = cnt
				}
				cnt++
			}
			// want parent lines to come first. so lines containing a parent come last.

			if ix := bytes.Index(line, []byte("Parent=")); ix != -1 {
				ix += 7
				ie := bytes.IndexByte(line[ix:], ';')
				if ie == -1 {
					ie = len(line) - 1 - ix
				}
				if o, ok := seen[string(line[ix:ix+ie])]; ok {
					return o
				}
				return 1
			}
			return 0
		})
	} else if ftype == "GFF" || ftype == "GTF" {
		FileCols[ftype] = []int{0, 3, 4}
	}

	sortFn := sortFnFromCols(FileCols[ftype], gf, getter)
	wtr := bufio.NewWriter(os.Stdout)

	if err := gsort.Sort(brdr, wtr, sortFn, args.Memory); err != nil {
		log.Fatal(err)
	}
}
