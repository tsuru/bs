//line parser.rl:1

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

//go:generate bash -c "ragel -Z -G2 -o parser.go parser.rl && gofmt -s -w parser.go && gofmt -s -w parser.go"

//line parser.go:13
const lineparser_start int = 1
const lineparser_first_final int = 16
const lineparser_error int = 0

const lineparser_en_main int = 1

//line parser.rl:12
func parseLogLine(data []byte) ([][]byte, bool, bool) {
	parts := make([][]byte, 6)
	start := 0
	pIdx := 0
	cs, p, pe := 0, 0, len(data)
	eof := pe
	withMsg := false
	withPID := false

//line parser.go:33
	{
		cs = lineparser_start
	}

//line parser.go:38
	{
		if p == pe {
			goto _test_eof
		}
		switch cs {
		case 1:
			goto st_case_1
		case 0:
			goto st_case_0
		case 2:
			goto st_case_2
		case 3:
			goto st_case_3
		case 4:
			goto st_case_4
		case 5:
			goto st_case_5
		case 6:
			goto st_case_6
		case 7:
			goto st_case_7
		case 8:
			goto st_case_8
		case 9:
			goto st_case_9
		case 10:
			goto st_case_10
		case 11:
			goto st_case_11
		case 16:
			goto st_case_16
		case 17:
			goto st_case_17
		case 12:
			goto st_case_12
		case 13:
			goto st_case_13
		case 14:
			goto st_case_14
		case 15:
			goto st_case_15
		}
		goto st_out
	st_case_1:
		if data[p] == 60 {
			goto st2
		}
		goto st0
	st_case_0:
	st0:
		cs = 0
		goto _out
	st2:
		if p++; p == pe {
			goto _test_eof2
		}
	st_case_2:
		if 48 <= data[p] && data[p] <= 57 {
			goto tr2
		}
		goto st0
	tr2:
//line parser.rl:23
		start = p
		goto st3
	st3:
		if p++; p == pe {
			goto _test_eof3
		}
	st_case_3:
//line parser.go:109
		if data[p] == 62 {
			goto tr4
		}
		if 48 <= data[p] && data[p] <= 57 {
			goto st3
		}
		goto st0
	tr4:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st4
	st4:
		if p++; p == pe {
			goto _test_eof4
		}
	st_case_4:
//line parser.go:126
		if data[p] == 32 {
			goto st4
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st4
		}
		goto tr5
	tr5:
//line parser.rl:23
		start = p
		goto st5
	st5:
		if p++; p == pe {
			goto _test_eof5
		}
	st_case_5:
//line parser.go:143
		if data[p] == 32 {
			goto tr8
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr8
		}
		goto st5
	tr8:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st6
	st6:
		if p++; p == pe {
			goto _test_eof6
		}
	st_case_6:
//line parser.go:160
		if data[p] == 32 {
			goto st6
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st6
		}
		goto tr9
	tr9:
//line parser.rl:23
		start = p
		goto st7
	st7:
		if p++; p == pe {
			goto _test_eof7
		}
	st_case_7:
//line parser.go:177
		if data[p] == 32 {
			goto tr12
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr12
		}
		goto st7
	tr12:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st8
	st8:
		if p++; p == pe {
			goto _test_eof8
		}
	st_case_8:
//line parser.go:194
		switch data[p] {
		case 32:
			goto st8
		case 91:
			goto st0
		case 93:
			goto st0
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st8
		}
		goto tr13
	tr13:
//line parser.rl:23
		start = p
		goto st9
	st9:
		if p++; p == pe {
			goto _test_eof9
		}
	st_case_9:
//line parser.go:216
		switch data[p] {
		case 32:
			goto st0
		case 58:
			goto tr16
		case 91:
			goto tr17
		case 93:
			goto st0
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st0
		}
		goto st9
	tr16:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st10
	st10:
		if p++; p == pe {
			goto _test_eof10
		}
	st_case_10:
//line parser.go:240
		switch data[p] {
		case 32:
			goto st11
		case 58:
			goto tr16
		case 91:
			goto tr17
		case 93:
			goto st0
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st11
		}
		goto st9
	st11:
		if p++; p == pe {
			goto _test_eof11
		}
	st_case_11:
		if data[p] == 32 {
			goto tr20
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr20
		}
		goto tr19
	tr19:
//line parser.rl:23
		start = p
//line parser.rl:25
		withMsg = true
		goto st16
	st16:
		if p++; p == pe {
			goto _test_eof16
		}
	st_case_16:
//line parser.go:278
		goto st16
	tr20:
//line parser.rl:23
		start = p
//line parser.rl:25
		withMsg = true
		goto st17
	st17:
		if p++; p == pe {
			goto _test_eof17
		}
	st_case_17:
//line parser.go:291
		if data[p] == 32 {
			goto tr20
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr20
		}
		goto tr19
	tr17:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st12
	st12:
		if p++; p == pe {
			goto _test_eof12
		}
	st_case_12:
//line parser.go:308
		if 48 <= data[p] && data[p] <= 57 {
			goto tr21
		}
		goto st0
	tr21:
//line parser.rl:23
		start = p
//line parser.rl:26
		withPID = true
		goto st13
	st13:
		if p++; p == pe {
			goto _test_eof13
		}
	st_case_13:
//line parser.go:324
		if data[p] == 93 {
			goto tr23
		}
		if 48 <= data[p] && data[p] <= 57 {
			goto st13
		}
		goto st0
	tr23:
//line parser.rl:24
		parts[pIdx] = data[start:p]
		pIdx++
		goto st14
	st14:
		if p++; p == pe {
			goto _test_eof14
		}
	st_case_14:
//line parser.go:341
		if data[p] == 58 {
			goto st15
		}
		goto st0
	st15:
		if p++; p == pe {
			goto _test_eof15
		}
	st_case_15:
		if data[p] == 32 {
			goto st11
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st11
		}
		goto st0
	st_out:
	_test_eof2:
		cs = 2
		goto _test_eof
	_test_eof3:
		cs = 3
		goto _test_eof
	_test_eof4:
		cs = 4
		goto _test_eof
	_test_eof5:
		cs = 5
		goto _test_eof
	_test_eof6:
		cs = 6
		goto _test_eof
	_test_eof7:
		cs = 7
		goto _test_eof
	_test_eof8:
		cs = 8
		goto _test_eof
	_test_eof9:
		cs = 9
		goto _test_eof
	_test_eof10:
		cs = 10
		goto _test_eof
	_test_eof11:
		cs = 11
		goto _test_eof
	_test_eof16:
		cs = 16
		goto _test_eof
	_test_eof17:
		cs = 17
		goto _test_eof
	_test_eof12:
		cs = 12
		goto _test_eof
	_test_eof13:
		cs = 13
		goto _test_eof
	_test_eof14:
		cs = 14
		goto _test_eof
	_test_eof15:
		cs = 15
		goto _test_eof

	_test_eof:
		{
		}
		if p == eof {
			switch cs {
			case 16, 17:
//line parser.rl:24
				parts[pIdx] = data[start:p]
				pIdx++
//line parser.go:382
			}
		}

	_out:
		{
		}
	}

//line parser.rl:33
	return parts, withMsg, withPID
}
