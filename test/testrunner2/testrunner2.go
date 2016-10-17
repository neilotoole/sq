package main

import (
	"fmt"

	"os"

	"io"

	"bytes"
	"io/ioutil"

	"encoding/csv"

	"bufio"

	"github.com/neilotoole/sq/libsq/util"
)

func main() {

	path := "/Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/csv/user_header.tsv"

	file, err := os.Open(path)
	//_, err := os.Open(path)
	util.OrPanic(err)

	//csv.NewReader(file)
	//
	//rd := bufio.NewReader(file)
	////rd := NewCRFilterReader(file)
	//count := 0
	//
	////r2 :=
	//
	//for {
	//	r, _, err := rd.ReadRune()
	//	if err == io.EOF {
	//		break
	//	}
	//
	//	if err != nil {
	//		util.OrPanic(err)
	//	}
	//
	//	if r == '\r' {
	//		fmt.Printf("-->%v\n", r)
	//		fmt.Printf(" %d: found a CR\n", count)
	//	}
	//
	//	//fmt.Printf("%d: %s\n", count, string(r))
	//	fmt.Printf("%s", string(r))
	//
	//	count++
	//}
	//
	//fmt.Printf("\ncount: %d\n", count)
	//
	//n := '\n'
	//fmt.Printf("n: %v\n", n)
	//bytes, err := ioutil.ReadFile(path)
	//util.OrPanic(err)
	////ioutil.ReadFile()
	//
	//fmt.Println(bytes)
	//
	//fmt.Println("\n\n---\n\n")
	//fmt.Println(string(bytes))
	//
	//fmt.Println("doing it! \n\n\n")
	//
	crd := csv.NewReader(NewCRFilterReader(file))

	for {
		record, err := crd.Read()
		if err == io.EOF {
			break
		}
		util.OrPanic(err)
		fmt.Println(record)
	}

	fmt.Println("done with it! \n\n\n")

	text := "\ra\rb\r\nc\rd\n"

	fmt.Println([]byte(text))

	rd := (NewCRFilterReader(bytes.NewReader([]byte(text))))

	bytes, err := ioutil.ReadAll(rd)

	fmt.Println(bytes)
	util.OrPanic(err)
	fmt.Printf("\ndoing it again\n%s\nfinished\n", string(bytes))

	bufio.NewReader()

}

type crFilterReader struct {
	r io.Reader
}

func (r *crFilterReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)

	for i := 0; i < n; i++ {
		if p[i] == 13 {
			if i+1 < n && p[i+1] == 10 {
				continue // it's \r\n
			}
			// it's just \r by itself, replace
			p[i] = 10
		}
	}

	return n, err
}

// NewCRFilterReader returns a new reader whose Read() method converts standalone
// carriage return '\r' bytes to newline '\n'. CRLF \r\n sequences are untouched.
// This is useful for reading from DOS format, e.g. a TSV file exported by
// Microsoft Excel.
func NewCRFilterReader(r io.Reader) io.Reader {
	return &crFilterReader{r: r}
}

//
//func (r *CRFilterReader) ReadRune() (rune, error) {
//
//	r1, _, err := r.Reader.ReadRune()
//	if r1 == '\r' {
//		r1, _, err = r.Reader.ReadRune()
//		if err == nil {
//			if r1 != '\n' {
//
//				r1 = '\n'
//			}
//		}
//		r.UnreadRune()
//	}
//
//	return r1, err
//}
//type CRFilterReader struct {
//	*bufio.Reader
//}
//
//func NewCRFilterReader(r io.Reader) io.Reader {
//
//	//b := &bufio.Reader{}
//
//	cr := &CRFilterReader{}
//	cr.Reader = bufio.NewReader(r)
//	return cr
//}
//
//func (r *CRFilterReader) ReadRune() (rune, error) {
//
//	r1, _, err := r.Reader.ReadRune()
//	if r1 == '\r' {
//		r1, _, err = r.Reader.ReadRune()
//		if err == nil {
//			if r1 != '\n' {
//
//				r1 = '\n'
//			}
//		}
//		r.UnreadRune()
//	}
//
//	return r1, err
//}
