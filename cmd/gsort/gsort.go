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

var DEFAULT_MEM int = 2000

var FileCols map[string][]int = map[string][]int{
	"BED": []int{0, 1, 2},
	"VCF": []int{0, 1, -1},
	"GFF": []int{0, 3, 4},
	"GTF": []int{0, 3, 4},
}

var CHECK_ORDER = []string{"BED", "GTF"}

var args struct {
	Path   string `arg:"positional,help:a tab-delimited file to sort"`
	Genome string `arg:"positional,help:a genome file of chromosome sizes and order"`
	Memory int    `arg:"-m,help:megabytes of memory to use before writing to temp files."`
}

func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// the last function is used when a column is -1
func sortFnFromCols(cols []int, gf *ggd_utils.GenomeFile, getter *func(int, [][]byte) int) func([]byte) []int {
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
	fn := func(line []byte) []int {
		l := make([]int, len(cols))
		// handle chromosome column
		toks := bytes.SplitN(line, []byte{'\t'}, m)
		// TODO: only do this when needed.
		// avoids problems with Atoi('222\n')
		end := toks[len(toks)-1]
		for end[len(end)-1] == '\n' || end[len(end)-1] == '\r' {
			end = end[:len(end)-1]
		}
		var ok bool
		// TODO: use unsafe string
		l[0], ok = gf.Order[string(toks[cols[0]])]
		if !ok {
			log.Fatalf("unknown chromosome: %s", toks[cols[0]])
		}
		for k, col := range cols[1:] {
			i := k + 1
			if col == -1 {
				l[i] = (*getter)(l[i-1], toks)
			} else {
				if col == len(toks)-1 {
					toks[col] = bytes.TrimRight(toks[col], "\r\n")
				}
				v, err := strconv.Atoi(unsafeString(toks[col]))
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
					return "", nil, fmt.Errorf("file has fewer than 3 columns")
				}
				for _, t := range CHECK_ORDER {
					cols := FileCols[t]
					ok := true
					last := 0
					for _, c := range cols[1:] {
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
				if ftype == "" {
					return "", nil, fmt.Errorf("unknown file format: %s", string(line))
				} else {
					break
				}
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

var vcfEndGetter func(int, [][]byte) int = func(start int, toks [][]byte) int {
	if bytes.Contains(toks[4], []byte{'<'}) && (bytes.Contains(toks[4], []byte("<DEL")) ||
		bytes.Contains(toks[4], []byte("<DUP")) ||
		bytes.Contains(toks[4], []byte("<INV")) ||
		bytes.Contains(toks[4], []byte("<CN"))) {
		// need to look at INFO for this.
		var info []byte
		if len(toks) < 8 {
			// just grab everything since we look for end= anyway
			info = toks[len(toks)-1]
		} else {
			info = toks[7]
		}
		if s, e := find([]byte("END="), info); s != -1 {
			end, err := getMax(info[s:e])
			if err != nil {
				log.Fatal(err)
			}
			return end
		}
		s, e := find([]byte("SVLEN="), info)
		if s == -1 {
			log.Printf("warning: cant find end for %s", string(info))
			return start + len(toks[3])
		}
		svlen, err := getMax(info[s:e])
		if err != nil {
			log.Fatal(err)
		}
		return start + svlen

	} else {
		// length of reference.
		return start + len(toks[3])
	}

}

func main() {

	args.Memory = DEFAULT_MEM
	p := arg.MustParse(&args)
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

	getter := &vcfEndGetter
	if ftype != "VCF" {
		getter = nil
	}

	sortFn := sortFnFromCols(FileCols[ftype], gf, getter)
	wtr := bufio.NewWriter(os.Stdout)

	if err := gsort.Sort(brdr, wtr, sortFn, args.Memory); err != nil {
		log.Fatal(err)
	}
}
