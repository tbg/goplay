// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package statbench

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

var seq int

func BenchmarkStat(b *testing.B) {
	b.Run("found", func(b *testing.B) {
		dir, err := ioutil.TempDir("", "")
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			_ = os.RemoveAll(dir)
		}()

		seq++
		n := seq

		file := func(i int) string {
			return filepath.Join(dir, strconv.Itoa(n), strconv.Itoa(i))
		}

		for i := 0; i < b.N; i++ {
			if err := os.MkdirAll(file(i), 0755); err != nil {
				b.Fatal(err)
			}
		}

		b.ResetTimer()
		defer b.StopTimer()

		for i := 0; i < b.N; i++ {
			if _, err := os.Stat(file(i)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("notFound", func(b *testing.B) {
		seq++
		n := seq
		for i := 0; i < b.N; i++ {
			file := filepath.Join(os.TempDir(), strconv.Itoa(n), strconv.Itoa(i))
			_, err := os.Stat(file)
			if !os.IsNotExist(err) {
				b.Fatal(err)
			}
		}
	})
}
