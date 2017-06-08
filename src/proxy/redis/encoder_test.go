// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/wfxiang08/cyutils/utils/assert"
)

var tmap = make(map[int64][]byte)

func init() {
	var n = len(itoamap)*2 + 100000
	for i := -n; i <= n; i++ {
		tmap[int64(i)] = []byte(strconv.Itoa(int(i)))
	}
	for i := math.MinInt64; i != 0; i = int(float64(i) / 1.1) {
		tmap[int64(i)] = []byte(strconv.Itoa(int(i)))
	}
	for i := math.MaxInt64; i != 0; i = int(float64(i) / 1.1) {
		tmap[int64(i)] = []byte(strconv.Itoa(int(i)))
	}
}

func TestItob(t *testing.T) {
	for i, b := range tmap {
		assert.Must(string(b) == string(itob(i)))
	}
}

func TestItoa(t *testing.T) {
	for i, b := range tmap {
		assert.Must(string(b) == itoa(i))
	}
}

func TestEncodeString(t *testing.T) {
	resp := NewString([]byte("OK"))
	testEncodeAndCheck(t, resp, []byte("+OK\r\n"))
}

func TestEncodeError(t *testing.T) {
	resp := NewError([]byte("Error"))
	testEncodeAndCheck(t, resp, []byte("-Error\r\n"))
}

func TestEncodeInt(t *testing.T) {
	for _, v := range []int{-1, 0, 1024 * 1024} {
		s := strconv.Itoa(v)
		resp := NewInt([]byte(s))
		testEncodeAndCheck(t, resp, []byte(":"+s+"\r\n"))
	}
}

func TestEncodeBulkBytes(t *testing.T) {
	resp := NewBulkBytes(nil)
	testEncodeAndCheck(t, resp, []byte("$-1\r\n"))
	resp.Value = []byte{}
	testEncodeAndCheck(t, resp, []byte("$0\r\n\r\n"))
	resp.Value = []byte("helloworld!!")
	testEncodeAndCheck(t, resp, []byte("$12\r\nhelloworld!!\r\n"))
}

func TestEncodeArray(t *testing.T) {
	resp := NewArray(nil)
	testEncodeAndCheck(t, resp, []byte("*-1\r\n"))
	resp.Array = []*Resp{}
	testEncodeAndCheck(t, resp, []byte("*0\r\n"))
	resp.Append(NewInt([]byte(strconv.Itoa(0))))
	testEncodeAndCheck(t, resp, []byte("*1\r\n:0\r\n"))
	resp.Append(NewBulkBytes(nil))
	testEncodeAndCheck(t, resp, []byte("*2\r\n:0\r\n$-1\r\n"))
	resp.Append(NewBulkBytes([]byte("test")))
	testEncodeAndCheck(t, resp, []byte("*3\r\n:0\r\n$-1\r\n$4\r\ntest\r\n"))
}

func testEncodeAndCheck(t *testing.T, resp *Resp, expect []byte) {
	b, err := EncodeToBytes(resp)
	assert.MustNoError(err)
	assert.Must(bytes.Equal(b, expect))
}