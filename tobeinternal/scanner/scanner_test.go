package scanner

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

const arbitraryLabel = "heybuddy"

type stalledReader struct {
	bytes.Buffer
}

func (stalledReader) Read(p []byte) (n int, err error) {
	time.Sleep(5 * time.Second)
	return 0, nil
}
func (stalledReader) Close() error { return nil }

func TestStalledReader(t *testing.T) {
	chOut := BuffScanner(1*time.Second, arbitraryLabel, stalledReader{})

	line, ok := <-chOut
	if !ok {
		t.Fail()
	}
	want := MsgTimeout
	if line != want {
		t.Errorf("got \n\t%v\nwant\n\t%v", line, want)
	}

	_, ok = <-chOut
	if ok {
		t.Fail()
	}
}

type bustedReader struct {
	bytes.Buffer
}

func (bustedReader) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (bustedReader) Close() error { return nil }

func TestBustedReader(t *testing.T) {
	chOut := BuffScanner(1*time.Second, arbitraryLabel, bustedReader{})

	line, ok := <-chOut
	if !ok {
		t.Fail()
	}
	want := MsgError + " : multiple Read calls return no data or error"
	if line != want {
		t.Errorf("got \n\t%v\nwant\n\t%v", line, want)
	}

	_, ok = <-chOut
	if ok {
		t.Fail()
	}
}

type simpleReader struct {
	io.Reader
}

func (simpleReader) Close() error { return nil }

func TestSimpleReader(t *testing.T) {
	chOut := BuffScanner(1*time.Second, arbitraryLabel,
		simpleReader{bytes.NewBufferString("beans and\nrice")})

	line, ok := <-chOut
	if !ok {
		t.Fail()
	}
	want := "beans and"
	if line != want {
		t.Errorf("got \n\t%v\nwant\n\t%v", line, want)
	}

	line, ok = <-chOut
	if !ok {
		t.Fail()
	}
	want = "rice"
	if line != want {
		t.Errorf("got \n\t%v\nwant\n\t%v", line, want)
	}

	_, ok = <-chOut
	if ok {
		t.Fail()
	}
}

// An example main.
func main() {
	{
		chOut := BuffScanner(
			1*time.Second, arbitraryLabel,
			simpleReader{bytes.NewBufferString("beans and\nrice")})
		for line := range chOut {
			fmt.Println(line)
		}
		fmt.Println("-----------------------")
	}
	{
		chOut := BuffScanner(1*time.Second, arbitraryLabel, stalledReader{})
		for line := range chOut {
			fmt.Println(line)
		}
		fmt.Println("-----------------------")
	}
	{
		chOut := BuffScanner(1*time.Second, arbitraryLabel, bustedReader{})
		for line := range chOut {
			fmt.Println(line)
		}
		fmt.Println("-----------------------")
	}
}
