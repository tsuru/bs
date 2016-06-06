// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

//go:generate bash -c "ragel -Z -G2 -o parser.go parser.rl && gofmt -s -w parser.go && gofmt -s -w parser.go"

%%{
    machine lineparser;
    write data;
}%%
func parseLogLine(data []byte) [][]byte {
    entries := make([][]byte, 7)
    starts := make([]int, 7)
    cs, p, pe := 0, 0, len(data)
    eof := pe
    push := func(v int) {
        starts[v] = p
        entries[v] = nil
    }
    pop := func(v int) {
        entries[v] = data[starts[v]:p]
    }
    %%{
        main :=
          '<' ( digit )+ >{push(0)} %{pop(0)} '>' space* ( any - space )+ >{push(1)} %{pop(1)} space+
          ( ( digit+ space digit+ ':' digit+ ':' digit+ ) >{push(2)} %{pop(2)} space+ )?
          (( alpha|digit|'-'|'.'|':' )+ >{push(3)} %{pop(3)} space+)? ( any - space - '[' - ']' )+ >{push(4)} %{pop(4)}
          ('[' ( digit )+ >{push(5)} %{pop(5)} ']')? ':' space+ ( any )+ >{push(6)} %{pop(6)};
        write init;
        write exec;
    }%%
    return entries
}
