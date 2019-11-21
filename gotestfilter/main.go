// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const modeUsage = `strip:
  omit output for non-failing tests, but print run/pass/skip events for all tests
omit:
  only emit failing tests
convert:
  don't perform any filtering, simply convert the json back to original test format'
`

var mode = flag.String("mode", "strip", modeUsage)

type testEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string
}

func main() {
	flag.Parse()
	if err := filter(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func filter(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	type tup struct {
		pkg  string
		test string
	}
	type ent struct {
		first, last     string // RUN and (SKIP|PASS|FAIL)
		strings.Builder        // output
	}
	m := map[tup]*ent{}
	ev := &testEvent{}
	for scanner.Scan() {
		line := scanner.Text() // has no EOL marker
		if len(line) <= 2 || line[0] != '{' || line[len(line)-1] != '}' {
			// Not test2json output.
			continue
		}
		*ev = testEvent{}
		if err := json.Unmarshal([]byte(line), ev); err != nil {
			return err
		}

		if *mode == "convert" {
			if ev.Action == "output" {
				fmt.Fprint(out, ev.Output)
			}
			continue
		}

		if ev.Test == "" {
			// Skip all package output. Unfortunately package events aren't
			// always well-formed. For example, if a test panics, the package
			// will never receive a fail event (arguably a bug), so it's not
			// trivial to print only failing packages. Besides, there's not much
			// package output typically (only init functions), so not worth
			// getting fancy about.
			continue
		}
		key := tup{ev.Package, ev.Test}
		buf := m[key]
		if buf == nil {
			buf = &ent{first: line}
			m[key] = buf
		}
		if _, err := fmt.Fprintln(buf, line); err != nil {
			return err
		}
		switch ev.Action {
		case "pass", "skip", "fail":
			buf.last = line
			delete(m, key)
			if ev.Action == "fail" {
				fmt.Fprint(out, buf.String())
			} else if *mode == "strip" {
				// Output only the start and end of test so that we preserve the
				// timing information. However, the output is omitted.
				fmt.Fprintln(out, buf.first)
				fmt.Fprintln(out, buf.last)
			}
		case "run", "pause", "cont", "bench", "output":
		default:
			// We must have parsed some JSON that wasn't a testData.
			return fmt.Errorf("unknown input: %s", line)
		}
	}
	// Some scopes might still be open. To the best of my knowledge, this is due
	// to a panic/premature exit of a test binary. In that case, it seems that
	// neither is the package scope closed, nor the scopes for any tests that
	// were running in parallel, so we pass that through if stripping, but not
	// when omitting.
	if *mode == "strip" {
		for key := range m {
			fmt.Fprintln(out, m[key].String())
		}
	}
	if n := len(m); n != 0 {
		return fmt.Errorf("%d tests did not terminate (a package likely exited prematurely)", n)
	}
	return nil
}
