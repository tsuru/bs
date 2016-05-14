//line parser.rl:1

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

//go:generate bash -c "ragel -Z -G2 -o parser.go parser.rl && gofmt -s -w parser.go && gofmt -s -w parser.go"

//line parser.go:13
const lineparser_start int = 1
const lineparser_first_final int = 24
const lineparser_error int = 0

const lineparser_en_main int = 1

//line parser.rl:12
func parseLogLine(data []byte) ([][]byte, bool, bool) {
	parts := make([][]byte, 8)
	start := 0
	pIdx := -1
	cs, p, pe := 0, 0, len(data)
	eof := pe
	withMsg := false
	withPID := false

//line parser.go:32
	{
		cs = lineparser_start
	}

//line parser.go:37
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
		case 24:
			goto st_case_24
		case 25:
			goto st_case_25
		case 12:
			goto st_case_12
		case 13:
			goto st_case_13
		case 14:
			goto st_case_14
		case 15:
			goto st_case_15
		case 16:
			goto st_case_16
		case 17:
			goto st_case_17
		case 18:
			goto st_case_18
		case 19:
			goto st_case_19
		case 20:
			goto st_case_20
		case 21:
			goto st_case_21
		case 22:
			goto st_case_22
		case 23:
			goto st_case_23
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
//line parser.rl:22
		pIdx++
		start = p
		goto st3
	st3:
		if p++; p == pe {
			goto _test_eof3
		}
	st_case_3:
//line parser.go:124
		if data[p] == 62 {
			goto tr4
		}
		if 48 <= data[p] && data[p] <= 57 {
			goto st3
		}
		goto st0
	tr4:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st4
	st4:
		if p++; p == pe {
			goto _test_eof4
		}
	st_case_4:
//line parser.go:141
		if data[p] == 32 {
			goto st4
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st4
		}
		goto tr5
	tr5:
//line parser.rl:22
		pIdx++
		start = p
		goto st5
	st5:
		if p++; p == pe {
			goto _test_eof5
		}
	st_case_5:
//line parser.go:158
		if data[p] == 32 {
			goto tr8
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr8
		}
		goto st5
	tr8:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st6
	st6:
		if p++; p == pe {
			goto _test_eof6
		}
	st_case_6:
//line parser.go:175
		if data[p] == 32 {
			goto st6
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr11
			}
		case data[p] >= 9:
			goto st6
		}
		goto tr9
	tr9:
//line parser.rl:22
		pIdx++
		start = p
		goto st7
	st7:
		if p++; p == pe {
			goto _test_eof7
		}
	st_case_7:
//line parser.go:197
		if data[p] == 32 {
			goto tr13
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr13
		}
		goto st7
	tr13:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st8
	st8:
		if p++; p == pe {
			goto _test_eof8
		}
	st_case_8:
//line parser.go:214
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
		goto tr14
	tr14:
//line parser.rl:22
		pIdx++
		start = p
		goto st9
	st9:
		if p++; p == pe {
			goto _test_eof9
		}
	st_case_9:
//line parser.go:236
		switch data[p] {
		case 32:
			goto st0
		case 58:
			goto tr17
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st0
		}
		goto st9
	tr17:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st10
	st10:
		if p++; p == pe {
			goto _test_eof10
		}
	st_case_10:
//line parser.go:260
		switch data[p] {
		case 32:
			goto st11
		case 58:
			goto tr17
		case 91:
			goto tr18
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
			goto tr21
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr21
		}
		goto tr20
	tr20:
//line parser.rl:22
		pIdx++
		start = p
//line parser.rl:24
		withMsg = true
		goto st24
	st24:
		if p++; p == pe {
			goto _test_eof24
		}
	st_case_24:
//line parser.go:298
		goto st24
	tr21:
//line parser.rl:22
		pIdx++
		start = p
//line parser.rl:24
		withMsg = true
		goto st25
	st25:
		if p++; p == pe {
			goto _test_eof25
		}
	st_case_25:
//line parser.go:311
		if data[p] == 32 {
			goto tr21
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto tr21
		}
		goto tr20
	tr18:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st12
	st12:
		if p++; p == pe {
			goto _test_eof12
		}
	st_case_12:
//line parser.go:328
		if 48 <= data[p] && data[p] <= 57 {
			goto tr22
		}
		goto st0
	tr22:
//line parser.rl:22
		pIdx++
		start = p
//line parser.rl:25
		withPID = true
		goto st13
	st13:
		if p++; p == pe {
			goto _test_eof13
		}
	st_case_13:
//line parser.go:344
		if data[p] == 93 {
			goto tr24
		}
		if 48 <= data[p] && data[p] <= 57 {
			goto st13
		}
		goto st0
	tr24:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st14
	st14:
		if p++; p == pe {
			goto _test_eof14
		}
	st_case_14:
//line parser.go:361
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
	tr11:
//line parser.rl:22
		pIdx++
		start = p
		goto st16
	st16:
		if p++; p == pe {
			goto _test_eof16
		}
	st_case_16:
//line parser.go:387
		if data[p] == 32 {
			goto tr26
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st16
			}
		case data[p] >= 9:
			goto tr26
		}
		goto st7
	tr26:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st17
	st17:
		if p++; p == pe {
			goto _test_eof17
		}
	st_case_17:
//line parser.go:409
		switch data[p] {
		case 32:
			goto st8
		case 91:
			goto st0
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr28
			}
		case data[p] >= 9:
			goto st8
		}
		goto tr14
	tr28:
//line parser.rl:22
		pIdx++
		start = p
		goto st18
	st18:
		if p++; p == pe {
			goto _test_eof18
		}
	st_case_18:
//line parser.go:436
		switch data[p] {
		case 32:
			goto st0
		case 58:
			goto tr30
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st18
			}
		case data[p] >= 9:
			goto st0
		}
		goto st9
	tr30:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st19
	st19:
		if p++; p == pe {
			goto _test_eof19
		}
	st_case_19:
//line parser.go:465
		switch data[p] {
		case 32:
			goto st11
		case 58:
			goto tr17
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st20
			}
		case data[p] >= 9:
			goto st11
		}
		goto st9
	st20:
		if p++; p == pe {
			goto _test_eof20
		}
	st_case_20:
		switch data[p] {
		case 32:
			goto st0
		case 58:
			goto tr32
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st20
			}
		case data[p] >= 9:
			goto st0
		}
		goto st9
	tr32:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st21
	st21:
		if p++; p == pe {
			goto _test_eof21
		}
	st_case_21:
//line parser.go:518
		switch data[p] {
		case 32:
			goto st11
		case 58:
			goto tr17
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st22
			}
		case data[p] >= 9:
			goto st11
		}
		goto st9
	st22:
		if p++; p == pe {
			goto _test_eof22
		}
	st_case_22:
		switch data[p] {
		case 32:
			goto tr34
		case 58:
			goto tr17
		case 91:
			goto tr18
		case 93:
			goto st0
		}
		switch {
		case data[p] > 13:
			if 48 <= data[p] && data[p] <= 57 {
				goto st22
			}
		case data[p] >= 9:
			goto tr34
		}
		goto st9
	tr34:
//line parser.rl:23
		parts[pIdx] = data[start:p]
		goto st23
	st23:
		if p++; p == pe {
			goto _test_eof23
		}
	st_case_23:
//line parser.go:571
		if data[p] == 32 {
			goto st23
		}
		if 9 <= data[p] && data[p] <= 13 {
			goto st23
		}
		goto tr9
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
	_test_eof24:
		cs = 24
		goto _test_eof
	_test_eof25:
		cs = 25
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
	_test_eof16:
		cs = 16
		goto _test_eof
	_test_eof17:
		cs = 17
		goto _test_eof
	_test_eof18:
		cs = 18
		goto _test_eof
	_test_eof19:
		cs = 19
		goto _test_eof
	_test_eof20:
		cs = 20
		goto _test_eof
	_test_eof21:
		cs = 21
		goto _test_eof
	_test_eof22:
		cs = 22
		goto _test_eof
	_test_eof23:
		cs = 23
		goto _test_eof

	_test_eof:
		{
		}
		if p == eof {
			switch cs {
			case 24, 25:
//line parser.rl:23
				parts[pIdx] = data[start:p]
//line parser.go:611
			}
		}

	_out:
		{
		}
	}

//line parser.rl:33
	return parts, withMsg, withPID
}
