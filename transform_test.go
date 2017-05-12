package transform

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

func Rot13(r io.Reader) *Transformer {
	buf := make([]byte, rand.Int()%256+1) // used to test varying slice sizes
	//buf := make([]byte, 256)
	return NewTransformer(func() ([]byte, error) {
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			if buf[i] >= 'a' && buf[i] <= 'z' {
				buf[i] = ((buf[i] - 'a' + 13) % 26) + 'a'
			} else if buf[i] >= 'A' && buf[i] <= 'Z' {
				buf[i] = ((buf[i] - 'A' + 13) % 26) + 'A'
			}
		}
		return buf[:n], nil
	})
}

func TestTransformer(t *testing.T) {
	// simple
	msg := "Hello\n13th Floor"
	data, err := ioutil.ReadAll(Rot13(Rot13(bytes.NewBufferString(msg))))
	if err != nil {
		t.Fatal(err)
	}
	if msg != string(data) {
		t.Fatalf("expected '%v', got '%v'\n", msg, string(data))
	}
	// random
	rand.Seed(time.Now().UnixNano())
	buf := make([]byte, 10000)
	for i := 0; i < 1000; i++ {
		_, err := rand.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		data, err := ioutil.ReadAll(Rot13(Rot13(bytes.NewBuffer(buf))))
		if err != nil {
			t.Fatal(err)
		}
		if string(buf) != string(data) {
			t.Fatalf("expected '%v', got '%v'\n", string(buf), string(data))
		}
	}
}

func ExampleTransformer_rot13() {
	// Rot13 transformation
	rot13 := func(r io.Reader) *Transformer {
		buf := make([]byte, 256)
		return NewTransformer(func() ([]byte, error) {
			n, err := r.Read(buf)
			if err != nil {
				return nil, err
			}
			for i := 0; i < n; i++ {
				if buf[i] >= 'a' && buf[i] <= 'z' {
					buf[i] = ((buf[i] - 'a' + 13) % 26) + 'a'
				} else if buf[i] >= 'A' && buf[i] <= 'Z' {
					buf[i] = ((buf[i] - 'A' + 13) % 26) + 'A'
				}
			}
			return buf[:n], nil
		})
	}
	// Pass the string though the transformer.
	out, err := ioutil.ReadAll(rot13(bytes.NewBufferString("Hello World")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
	// Output:
	// Uryyb Jbeyq
}

func ExampleTransformer_toUpper() {
	// Convert a string to uppper case. Unicode aware.
	toUpper := func(r io.Reader) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			c, _, err := br.ReadRune()
			if err != nil {
				return nil, err
			}
			return []byte(strings.ToUpper(string([]rune{c}))), nil
		})
	}
	// Pass the string though the transformer.
	out, err := ioutil.ReadAll(toUpper(bytes.NewBufferString("Hello World")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
	// Output:
	// HELLO WORLD
}

func ExampleTransformer_lineMatcherRegExp() {
	// Filter lines matching a pattern
	matcher := func(r io.Reader, pattern string) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			for {
				line, err := br.ReadBytes('\n')
				matched, _ := regexp.Match(pattern, line)
				if matched {
					return line, err
				}
				if err != nil {
					return nil, err
				}
			}
		})
	}

	logs := `
23 Apr 17:32:23.604 [INFO] DB loaded in 0.551 seconds
23 Apr 17:32:23.605 [WARN] Disk space is low
23 Apr 17:32:23.054 [INFO] Server started on port 7812
23 Apr 17:32:23.141 [INFO] Ready for connections
	`
	// Pass the string though the transformer.
	out, err := ioutil.ReadAll(matcher(bytes.NewBufferString(logs), "WARN"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
	// Output:
	// 23 Apr 17:32:23.605 [WARN] Disk space is low
}

func ExampleTransformer_trimmer() {
	// Trim space from all lines
	trimmer := func(r io.Reader) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			for {
				line, err := br.ReadBytes('\n')
				if len(line) > 0 {
					line = append(bytes.TrimSpace(line), '\n')
				}
				return line, err
			}
		})
	}

	phrases := "  lacy timber \n"
	phrases += "\t\thybrid gossiping\t\n"
	phrases += " coy radioactivity\n"
	phrases += "rocky arrow  \n"
	// Pass the string though the transformer.
	out, err := ioutil.ReadAll(trimmer(bytes.NewBufferString(phrases)))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
	// Output:
	// lacy timber
	// hybrid gossiping
	// coy radioactivity
	// rocky arrow
}

func ExampleTransformer_pipeline() {
	// Filter lines matching a pattern
	matcher := func(r io.Reader, pattern string) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			for {
				line, err := br.ReadBytes('\n')
				matched, _ := regexp.Match(pattern, line)
				if matched {
					return line, err
				}
				if err != nil {
					return nil, err
				}
			}
		})
	}

	// Trim space from all lines
	trimmer := func(r io.Reader) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			for {
				line, err := br.ReadBytes('\n')
				if len(line) > 0 {
					line = append(bytes.TrimSpace(line), '\n')
				}
				return line, err
			}
		})
	}

	// Convert a string to uppper case. Unicode aware. In this example
	// we only process one rune at a time. It works but it's not ideal
	// for production.
	toUpper := func(r io.Reader) *Transformer {
		br := bufio.NewReader(r)
		return NewTransformer(func() ([]byte, error) {
			c, _, err := br.ReadRune()
			if err != nil {
				return nil, err
			}
			return []byte(strings.ToUpper(string([]rune{c}))), nil
		})
	}
	phrases := "  lacy timber \n"
	phrases += "\t\thybrid gossiping\t\n"
	phrases += " coy radioactivity\n"
	phrases += "rocky arrow  \n"

	// create a transformer that matches lines on the letter 'o', trims the
	// space from the lines, and transforms to upper case.
	r := toUpper(trimmer(matcher(bytes.NewBufferString(phrases), "o")))

	// Pass the string though the transformer.
	out, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
	// Output:
	// HYBRID GOSSIPING
	// COY RADIOACTIVITY
	// ROCKY ARROW
}
