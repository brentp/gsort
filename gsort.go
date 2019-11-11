// Package gsort is a library for sorting a stream of tab-delimited lines ([]bytes) (from a reader)
// using the amount of memory requested.
//
// Instead of using a compare function as most sorts do, this accepts a user-defined
// function with signature: `func(line []byte) []int` where the []ints are used to
// determine ordering. For example if we were sorting on 2 columns, one of months and another of
// day of months, the function would replace "Jan" with 1 and "Feb" with 2 for the first column
// and just return the Atoi of the 2nd column.
//
// Header lines are assumed to start with '#'. To indicate other lines that are header lines, the
// user function to Sort() can return `[]int{gsort.HEADER_LINE}`.
package gsort

import (
	"bufio"
	"bytes"
	"compress/flate"
	"fmt"
	"io/ioutil"
	"math"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	gzip "github.com/klauspost/compress/gzip"
	"github.com/pkg/errors"

	//gzip "github.com/klauspost/pgzip"

	"container/heap"
	"io"
	"log"
	"os"
	"sort"
)

type chunk struct {
	lines [][]byte
	idxs  []int // used only in Heap
	Cols  [][]int
}

func (c chunk) Len() int {
	return len(c.lines)
}

func (c chunk) Less(i, j int) bool {

	for k := 0; k < len(c.Cols[i]); k++ {
		if c.Cols[j][k] == c.Cols[i][k] {
			continue
		}
		return c.Cols[i][k] < c.Cols[j][k]
	}
	return false
}
func (c *chunk) Swap(i, j int) {
	if i < len((*c).lines) {
		(*c).lines[j], (*c).lines[i] = c.lines[i], c.lines[j]
	}
	if i < len((*c).Cols) {
		(*c).Cols[j], (*c).Cols[i] = c.Cols[i], c.Cols[j]
	}
	if i < len((*c).idxs) {
		(*c).idxs[j], (*c).idxs[i] = c.idxs[i], c.idxs[j]
	}
}

// for Heap
type pair struct {
	line []byte
	idx  int
	cols []int
}

func (c *chunk) Push(i interface{}) {
	p := i.(pair)
	(*c).lines = append((*c).lines, p.line)
	(*c).idxs = append((*c).idxs, p.idx)
	(*c).Cols = append((*c).Cols, p.cols)
}

func (c *chunk) Pop() interface{} {
	n := len((*c).lines)
	if n == 0 {
		return nil
	}
	line := (*c).lines[n-1]
	(*c).lines = (*c).lines[:n-1]
	idx := (*c).idxs[n-1]
	(*c).idxs = (*c).idxs[:n-1]

	cols := (*c).Cols[n-1]
	(*c).Cols = (*c).Cols[:n-1]

	return pair{line, idx, cols}
}

// Processor is a function that takes a line and return a slice of ints that determine ordering
type Processor func(line []byte) []int

// Sort accepts a tab-delimited io.Reader and writes to wtr using prepocess to determine ordering
func Sort(rdr io.Reader, wtr io.Writer, preprocess Processor, memMB int, chromosomeMappings map[string]string) error {

	/*
		f, perr := os.Create("gsort.pprof")
		if perr != nil {
			panic(perr)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	*/

	brdr, bwtr := bufio.NewReader(rdr), bufio.NewWriter(wtr)
	defer bwtr.Flush()

	if err := writeHeader(bwtr, brdr); err == io.EOF {
		return nil
	} else if err != nil {
		return errors.Wrap(err, "error reading/writing header")
	}

	ch := make(chan [][]byte)
	go readLines(ch, brdr, memMB, chromosomeMappings)
	fileNames := writeChunks(ch, preprocess)

	for _, f := range fileNames {
		defer os.Remove(f)
	}

	if len(fileNames) == 1 {
		return writeOne(fileNames[0], bwtr)
	}
	// TODO have special merge for when stuff is already mostly sorted. don't need pri queue.
	return merge(fileNames, bwtr, preprocess)
}

func readLines(ch chan [][]byte, rdr *bufio.Reader, memMb int, chromosomeMappings map[string]string) {

	mem := int(1000000.0 * float64(memMb) * 0.7)

	lines := make([][]byte, 0, 500000)
	var line []byte
	var err error

	sum := 0
	k := 0

	for {

		line, err = rdr.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		if len(line) > 0 {
			if chromosomeMappings != nil {
				i := bytes.IndexRune(line, '\t')
				chrom := string(line[0:i])

				newChrom, ok := chromosomeMappings[chrom]
				if !ok {
					log.Printf("[gsort] WARNING: could not find mapping for chromosome: %s", chrom)
				} else {
					line = append([]byte(newChrom), line[i:]...)
				}
			}

			lines = append(lines, line)
			sum += len(line)
		}

		if len(line) == 0 || err == io.EOF {
			np := len(lines)
			last := lines[np-1]
			if len(last) == 0 || last[len(last)-1] != '\n' {
				lines[np-1] = append(last, '\n')
			}
			ch <- lines
			break
		}

		if sum >= mem {
			ch <- lines
			lines = make([][]byte, 0, 500000)
			if k == 0 {
				ch <- make([][]byte, 0, 0)
				mem /= 3
			}
			k++
			sum = 0
		}
	}
	close(ch)
}

// indicate that this is a header line, even if it doesn't have '#' prefix
const HEADER_LINE = math.MinInt32

func writeHeader(wtr *bufio.Writer, rdr *bufio.Reader) error {
	for {
		b, err := rdr.Peek(1)
		if err != nil {
			return errors.Wrap(err, "error peaking for header")
		}
		if b[0] != '#' {
			break
		}
		line, err := rdr.ReadBytes('\n')
		if err != nil {
			return err
		}
		wtr.Write(line)
	}
	return nil
}

// fast path where we don't use merge if it all fit in memory.
func writeOne(fname string, wtr io.Writer) error {
	rdr, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer rdr.Close()
	gz, err := gzip.NewReader(rdr)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(wtr, gz)
	return errors.Wrapf(err, "error copying from %s", fname)
}

func merge(fileNames []string, wtr io.Writer, process Processor) error {

	start := time.Now()

	fhs := make([]*bufio.Reader, len(fileNames))

	cache := chunk{lines: make([][]byte, len(fileNames)),
		Cols: make([][]int, len(fileNames)),
		idxs: make([]int, len(fileNames))}

	for i, fn := range fileNames {
		fh, err := os.Open(fn)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error opening: %s", fn))
		}
		defer fh.Close()
		//gz, err := newFastGzReader(fh)
		gz, err := gzip.NewReader(fh)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error reading %s as gzip", fn))
		}
		defer gz.Close()
		fhs[i] = bufio.NewReader(gz)

		line, err := fhs[i].ReadBytes('\n')
		if len(line) > 0 {
			cache.lines[i] = line
			cache.Cols[i] = process(line)
			cache.idxs[i] = i
		} else if err == io.EOF {
			continue
		} else if err != nil {
			return err
		}
	}

	heap.Init(&cache)

	for {
		o := heap.Pop(&cache)

		if o == nil {
			break
		}
		c := o.(pair)
		// refill from same file
		line, err := fhs[c.idx].ReadBytes('\n')
		if err != io.EOF && err != nil {
			return err
		}
		if len(line) != 0 {
			next := pair{line: line, idx: c.idx, cols: process(line)}
			heap.Push(&cache, next)
		} else {
			os.Remove(fileNames[c.idx])
		}
		wtr.Write(c.line)

	}

	log.Printf("time to merge %d files: %.3f", len(fileNames), time.Since(start).Seconds())
	return nil
}

func init() {
	// make sure we don't leave any temporary files.
	c := make(chan os.Signal, 1)
	pid := os.Getpid()
	signal.Notify(c,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-c
		matches, err := filepath.Glob(filepath.Join(os.TempDir(), fmt.Sprintf("gsort.%d.*", pid)))
		if err != nil {
			log.Fatal(err)
		}
		for _, m := range matches {
			os.Remove(m)
		}
		os.Exit(3)
	}()

}

func writeChunks(ch chan [][]byte, process Processor) []string {
	fileNames := make([]string, 0, 20)
	pid := os.Getpid()
	for lines := range ch {
		if len(lines) == 0 {
			continue
		}
		f, err := ioutil.TempFile("", fmt.Sprintf("gsort.%d.%d.", pid, len(fileNames)))
		if err != nil {
			log.Fatal(err)
		}
		achunk := chunk{lines: lines, Cols: make([][]int, len(lines))}
		for i, line := range achunk.lines {
			achunk.Cols[i] = process(line)
		}
		//lines = nil

		//sort.Stable(&achunk)
		sort.Sort(&achunk)

		gz, _ := gzip.NewWriterLevel(f, flate.BestSpeed)
		wtr := bufio.NewWriterSize(gz, 65536)
		for i, line := range achunk.lines {
			wtr.Write(line)
			achunk.lines[i] = nil
			lines[i] = nil
		}
		wtr.Flush()
		achunk.Cols, lines = nil, nil
		achunk.lines = nil
		gz.Close()
		f.Close()
		fileNames = append(fileNames, f.Name())
	}
	runtime.GC()
	return fileNames
}
