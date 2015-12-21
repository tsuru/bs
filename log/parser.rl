// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

//go:generate bash -c "ragel -Z -G2 -o parser.go parser.rl && gofmt -s -w parser.go && gofmt -s -w parser.go"

%%{
    machine lineparser;
    write data;
}%%

func parseLogLine(data []byte) ([][]byte, bool, bool) {
    parts := make([][]byte, 6)
    start := 0
    pIdx := 0
    cs, p, pe := 0, 0, len(data)
    eof := pe
    withMsg := false
    withPID := false
    %%{
        action di { start = p }
        action dd { parts[pIdx] = data[start:p]; pIdx++ }
        action msgok { withMsg = true }
        action withpid { withPID = true }
        main :=
          '<' ( digit )+ >di %dd '>' space* ( any - space )+ >di %dd space+
          ( any - space )+ >di %dd space+ ( any - space - '[' - ']' )+ >di %dd
          ('[' ( digit )+ >di %dd >withpid ']')? ':' space+ ( any )+ >di %dd >msgok;
        write init;
        write exec;
    }%%
    return parts, withMsg, withPID
}
