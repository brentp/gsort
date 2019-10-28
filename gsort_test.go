package gsort_test

import (
	"bytes"
	"log"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/brentp/gsort"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type GSortTest struct{}

var _ = Suite(&GSortTest{})

func (s *GSortTest) TestSort1(c *C) {

	data := strings.NewReader(`a	1
b	2
a	3
`)

	pp := func(line []byte) []int {
		l := make([]int, 2)
		toks := bytes.Split(line, []byte{'\t'})
		l[0] = int(toks[0][0])
		if len(toks) > 1 {
			v, err := strconv.Atoi(string(toks[1]))
			if err != nil {
				l[1] = -1
			} else {
				l[1] = v
			}
		} else {
			l[1] = -1
		}
		return l

	}
	b := make([]byte, 0, 20)
	wtr := bytes.NewBuffer(b)

	err := gsort.Sort(data, wtr, pp, 22, nil)
	c.Assert(err, IsNil)

	c.Assert(wtr.String(), Equals, `a	1
a	3
b	2
`)

}

func (s *GSortTest) TestSort2(c *C) {
	// sort by number, then reverse letter

	data := strings.NewReader(`a	1
b	2
a	3
g	1
`)

	pp := func(line []byte) []int {
		l := make([]int, 2)
		toks := bytes.Split(line, []byte{'\t'})
		l[1] = -int(toks[0][0])
		if len(toks) > 1 {
			toks[1] = bytes.TrimSuffix(toks[1], []byte{'\n'})
			v, err := strconv.Atoi(string(toks[1]))
			if err != nil {
				l[0] = -1
			} else {
				l[0] = v
			}
		} else {
			l[0] = math.MinInt32
		}
		return l

	}
	b := make([]byte, 0, 20)
	wtr := bytes.NewBuffer(b)

	err := gsort.Sort(data, wtr, pp, 22, nil)
	c.Assert(err, IsNil)

	c.Assert(wtr.String(), Equals, `g	1
a	1
b	2
a	3
`)

	// sort numbers in reverse
	rev := func(line []byte) []int {
		l := make([]int, 2)
		toks := bytes.Split(line, []byte{'\t'})
		l[1] = -int(toks[0][0])
		if len(toks) > 1 {
			toks[1] = bytes.TrimSuffix(toks[1], []byte{'\n'})
			v, err := strconv.Atoi(string(toks[1]))
			if err != nil {
				log.Println(err)
				l[0] = 1
			} else {
				// NOTE added negative here
				l[0] = -v
			}
		} else {
			l[0] = math.MaxInt32
		}
		return l

	}

	b = make([]byte, 0, 20)
	wtr = bytes.NewBuffer(b)
	data = strings.NewReader(`a	1
b	2
a	3
g	1`)

	err = gsort.Sort(data, wtr, rev, 22, nil)
	c.Assert(err, IsNil)

	c.Assert(wtr.String(), Equals, `a	3
b	2
g	1
a	1
`)

}
