// Copyright 2019 PayPal Inc.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysqlpackets

import (
	// "io"
	// "strings"
	"testing"
	"bytes"
	"github.com/paypal/hera/common"
	"github.com/paypal/hera/utility/encoding"
	"reflect"
	"math/rand"
)

var codes map[int]string

type nsCase struct {
	Serialized []byte
	ns         *MySQLPacket
}

func tcase(tcases []nsCase, t *testing.T) {
	for _, tcase := range tcases {
		t.Log("Testing for: ", tcase.Serialized)
		ns, _ := NewPacket(bytes.NewReader(tcase.Serialized))
		if ns.Length != tcase.ns.Length {
			t.Log("Length expected", tcase.ns.Length, "instead got", ns.Length)
		}
		if ns.Sequence_id != tcase.ns.Sequence_id {
			t.Log("Length expected", tcase.ns.Sequence_id, "instead got", ns.Sequence_id)
		}
		if ns.Cmd != tcase.ns.Cmd {
			t.Log("Command expected", tcase.ns.Cmd, "instead got", ns.Cmd)
			t.Fail()
		}
		if !reflect.DeepEqual(ns.Payload, tcase.ns.Payload) {
			t.Log("Payload expected", tcase.ns.Payload, "instead got", ns.Payload)
			t.Fail()
		}
		if !reflect.DeepEqual(ns.Serialized, tcase.ns.Serialized) {
			t.Log("Serialized expected", tcase.ns.Serialized, "instead got", ns.Serialized)
			t.Fail()
		}
		t.Log("Done testing for: ", tcase.Serialized)
	}
}

/* Make test cases for simple queries. */
func tmake() ([]nsCase) {

	cases := make([]nsCase, 6)
	// Initialize all the relevant codes.
	codes = make(map[int]string)
	codes[common.COM_SLEEP] =  "COM_SLEEP"
     codes[common.COM_QUIT] = "COM_QUIT"
     codes[common.COM_INIT_DB] = "COM_INIT_DB"
     codes[common.COM_QUERY] = "COM_QUERY"
     codes[common.COM_FIELD_LIST] = "COM_FIELD_LIST"
     codes[common.COM_CREATE_DB] = "COM_CREATE_DB"
     codes[common.COM_DROP_DB] = "COM_DROP_DB"
     codes[common.COM_REFRESH] = "COM_REFRESH"
     codes[common.COM_SHUTDOWN] = "COM_SHUTDOWN"

     codes[common.COM_STMT_PREPARE] = "COM_STMT_PREPARE"
     codes[common.COM_STMT_EXECUTE] = "COM_STMT_EXECUTE"
     codes[common.COM_STMT_SEND_LONG_DATA] = "COM_STMT_SEND_LONG_DATA"
     codes[common.COM_STMT_CLOSE] = "COM_STMT_CLOSE"
     codes[common.COM_STMT_FETCH] = "COM_STMT_FETCH"

	// COMMAND PACKETS
	var query, payload []byte



	query = []byte{0x12,  00,  00,  00,  3,  83,  84,  65,  82,  84,  32,  84,  82,  65,  78,  83,  65,  67,  84,  73,  79,  78}
	payload = []byte{3,  83,  84,  65,  82,  84,  32,  84,  82,  65,  78,  83,  65,  67,  84,  73,  79,  78}
	cases[0] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:3, Serialized:query, Payload:payload, Length:18, Sequence_id:0}}}


	query = []byte{ 0x2b,  00,  00,  00, 22,  105,  110,  115,  101,  114,  116,  32,  105,  110,  116,  111,  32,  116,  101,  115,  116,  49,  32,  40,  105,  100,  44,  32,  118,  97,  108,  41,  32,  118,  97,  108,  117,  101,  115,  32,  40,  63,  44,  32,  63,  41,  59}
	payload = []byte{22,  105,  110,  115,  101,  114,  116,  32,  105,  110,  116,  111,  32,  116,  101,  115,  116,  49,  32,  40,  105,  100,  44,  32,  118,  97,  108,  41,  32,  118,  97,  108,  117,  101,  115,  32,  40,  63,  44,  32,  63,  41,  59}
	cases[1] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:22, Serialized:query, Payload:payload, Length:43, Sequence_id:0}}}


	query = []byte{0x20, 00, 00, 00, 23, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 8, 0, 8, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0}
	payload = []byte{23, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 8, 0, 8, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0}
	cases[2] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:23, Serialized:query, Payload:payload, Length:32, Sequence_id:0}}}

	query = []byte{0x20, 00, 00, 00, 22, 100, 101, 108,  101,  116,  101,  32,  102,  114,  111,  109,  32,  116,  101,  115,  116,  49,  32,  119,  104,  101,  114,  101,  32,  105,  100,  32,  61,  32,  50, 59}
	payload = []byte{22, 100, 101, 108, 101, 116, 101,  32,  102,  114,  111,  109,  32,  116,  101,  115,  116,  49,  32,  119,  104,  101,  114,  101,  32,  105,  100,  32,  61,  32,  50,  59}
	cases[3] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:22, Serialized:query, Payload:payload, Length:32, Sequence_id:0}}}

	query = []byte{5, 0, 0, 0, 25, 1, 0, 0, 0}
	payload = []byte{25, 1, 0, 0, 0}
	cases[4] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:25, Serialized:query, Payload:payload, Length:5, Sequence_id:0}}}

	query = []byte{1, 00, 00, 00, 1}
	payload = []byte{01}
	cases[5] = nsCase{Serialized:query, ns:&MySQLPacket{encoding.Packet{Cmd:1, Serialized:query, Payload:payload, Length:1, Sequence_id:0}}}

	return cases
}

func TestBasic(t *testing.T) {
	t.Log("Start TestBasic ++++++++++++++")

	tcase(tmake(), t)

	t.Log("End TestBasic ++++++++++++++")
}


func TestReadNext(t *testing.T) {
	t.Log("Start TestReadNext +++++++++++++")

	numPackets := rand.Intn(48) + 2 // pick a random number between 2 and 50
	endPacketLength := rand.Intn(1 << 24 - 2) // pick length of terminal packet

	// Send this numPackets + 1 through to be read by ReadNext() in one stream
	big_payload := make([]byte, (1 << 24 - 1) * numPackets + endPacketLength)

	// Create a new packet reader
	reader := NewPacketReader(bytes.NewReader(big_payload))

	// Return the next packet from the string!
	ns, err := reader.ReadNext()


	t.Log("End TestReadNext +++++++++++++")
}


// /* Tests whether or not the server properly writes multiple packets to the
// conn. */
// func TestWriteMultiplePackets(t *testing.T) {
// 	numPackets := rand.Intn(49) + 1
// 	big_payload := make([]byte, 1 << 24 - 1)
//
// 	for numPackets > 0 {
// 		numPackets--
//
//
// 	}
// }
//
// // func TestWriteEmbedded(t *testing.T) {
// // 	nss := make([]*Netstring, 3)
// // 	nss[0] = NewPacketFrom(502, []byte("abc"))
// // 	nss[1] = NewPacketFrom(5, []byte(""))
// // 	nss[2] = NewPacketFrom(25, []byte("1234567890?1234567890?1234567890?"))
// //
// // 	ns := NewNetstringEmbedded(nss)
// // 	if ns.Cmd != 0 {
// // 		t.Log("Command expected '0' instead got", ns.Cmd)
// // 		t.Fail()
// // 	}
// // 	if strings.Compare(string(ns.Payload), "7:502 abc,1:5,36:25 1234567890?1234567890?1234567890?,") != 0 {
// // 		t.Log("Payload expected '7:502 abc,1:5,36:25 1234567890?1234567890?1234567890?,' instead got ", string(ns.Payload))
// // 		t.Fail()
// // 	}
// // 	if strings.Compare(string(ns.Serialized), "56:0 7:502 abc,1:5,36:25 1234567890?1234567890?1234567890?,,") != 0 {
// // 		t.Log("Serialized expected '56:0 7:502 abc,1:5,36:25 1234567890?1234567890?1234567890?,,' instead got", string(ns.Serialized))
// // 		t.Fail()
// // 	}
// // }
//
// /* Tests whether or not the server properly reads multiple pacets from the
// conn. */
// func TestReadMultiplePackets(t *testing.T) {
// 	length := 0xffffff
//
// 	for length == 0xffffff {
// 		// Read those boys
//
// 	}
// }
//
// // func TestReadEmbedded(t *testing.T) {
// // 	nss := make([]*Netstring, 3)
// // 	nss[0] = NewPacketFrom(502, []byte("xyzwx*abcdef"))
// // 	nss[1] = NewPacketFrom(5, []byte(""))
// // 	nss[2] = NewPacketFrom(25, []byte("1234567890*1234567890"))
// //
// // 	ns := NewNetstringEmbedded(nss)
// //
// // 	t.Log("NS::::", string(ns.Serialized))
// //
// // 	nss2, _ := SubNetstrings(ns)
// // 	if len(nss2) != 3 {
// // 		t.Log("Expected 3 sub-netstrings, instead got", len(nss2))
// // 		t.Fail()
// // 	}
// // 	for idx, i := range nss2 {
// // 		t.Log("Cmd:", i.Cmd, ", Payload:", string(i.Payload), ", Serialized:", string(i.Serialized))
// //
// // 		if i.Cmd != nss[idx].Cmd {
// // 			t.Log("Command expected", nss[idx].Cmd, "instead got", i.Cmd)
// // 			t.Fail()
// // 		}
// // 		if strings.Compare(string(i.Payload), string(nss[idx].Payload)) != 0 {
// // 			t.Log("Payload expected", string(nss[idx].Payload), "instead got", string(i.Payload))
// // 			t.Fail()
// // 		}
// // 		if strings.Compare(string(i.Serialized), string(nss[idx].Serialized)) != 0 {
// // 			t.Log("Payload expected", string(nss[idx].Serialized), "instead got", string(i.Serialized))
// // 			t.Fail()
// // 		}
// // 	}
// // }
//
// func TestPacketReader(t *testing.T) {
//
// }
//
// // func TestNetstringReader(t *testing.T) {
// // 	reader := NewPacketReader(bytes.NewReader([]byte{0x12, 00, 00, 00, 0x17, 0x01, 0x00, 00, 00, 00, 0, 00, 00, 00, 00, 01, 0x0f, 00, 03, 0x66, 0x6f, 0x6f}))
// // 	nss := make([]*Netstring, 6)
// // 	nss[0] = NewPacketFrom(502, []byte("xyzwx*abcdef"))
// // 	nss[1] = NewPacketFrom(5, []byte(""))
// // 	nss[2] = NewPacketFrom(25, []byte("1234567890*1234567890"))
// // 	nss[3] = NewPacketFrom(502, []byte("xyzwx*WWWWWWW"))
// // 	nss[4] = NewPacketFrom(5, []byte(""))
// // 	nss[5] = NewPacketFrom(25, []byte("1234567890*1234567890"))
// // 	idx := -1
// // 	var ns *Netstring
// // 	var err error
// // 	for {
// // 		ns, err = reader.ReadNext()
// // 		if err != nil {
// // 			if err != io.EOF {
// // 				t.Log("Error ReadNext: ", err.Error())
// // 				t.Fail()
// // 			}
// // 			break
// // 		}
// // 		idx++
// // 		t.Log("Cmd:", ns.Cmd, ", Payload:", string(ns.Payload), ", Serialized:", string(ns.Serialized))
// // 		if ns.Cmd != nss[idx].Cmd {
// // 			t.Log("Command expected", nss[idx].Cmd, "instead got", ns.Cmd)
// // 			t.Fail()
// // 		}
// // 		if strings.Compare(string(ns.Payload), string(nss[idx].Payload)) != 0 {
// // 			t.Log("Payload expected", string(nss[idx].Payload), "instead got", string(ns.Payload))
// // 			t.Fail()
// // 		}
// // 		if strings.Compare(string(ns.Serialized), string(nss[idx].Serialized)) != 0 {
// // 			t.Log("Payload expected", string(nss[idx].Serialized), "instead got", string(ns.Serialized))
// // 			t.Fail()
// // 		}
// // 	}
// // 	if idx != 5 {
// // 		t.Log("Expected 6 Netstrings to be read, instead found only:", idx+1)
// // 		t.Fail()
// // 	}
// // }
//
// func TestBadPacket(t *testing.T) {
//
// }
//
// // func TestBadInput(t *testing.T) {
// // 	reader := NewPacketReader(strings.NewReader("54:0 16:502 "))
// // 	_, err := reader.ReadNext()
// // 	if err != nil {
// // 		t.Log("OK: expected error:", err.Error())
// // 	} else {
// // 		t.Log("Bad input should have failed - incomplete Netstring")
// // 		t.Fail()
// // 	}
// // 	reader = NewPacketReader(strings.NewReader("55:0 16:502 xyzwx*abcdef,50:5,24:25 1234567890*1234567890,,"))
// // 	// first NS is fine
// // 	_, err = reader.ReadNext()
// // 	if err != nil {
// // 		t.Log("First Netstring should have been OK")
// // 		t.Fail()
// // 	}
// // 	// second is bad, length is "50" but much fewer bytes are available
// // 	_, err = reader.ReadNext()
// // 	if err != nil {
// // 		t.Log("OK: expected error:", err.Error())
// // 	} else {
// // 		t.Log("Bad input should have failed - incomplete embedded Netstring")
// // 		t.Fail()
// // 	}
// // }
// //
// // // per https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go, to avoid compiler optimizations
//
// var result *MySQLPacket
//
// func BenchmarkEncode(b *testing.B) {
//
// }

// func BenchmarkEncode(b *testing.B) {
// 	var ns *Netstring
// 	nss := make([]*Netstring, 10)
// 	for i := 0; i < b.N; i++ {
// 		nss[0] = NewPacketFrom(25, []byte("select id, int_val, str_val from test where id = :account_id and name = :name and address = :address  /*12345-123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890-123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901*/"))
// 		nss[1] = NewPacketFrom(4, []byte("account_id"))
// 		nss[2] = NewPacketFrom(3, []byte("1234567890"))
// 		nss[3] = NewPacketFrom(4, []byte("name"))
// 		nss[4] = NewPacketFrom(3, []byte("John Smith"))
// 		nss[5] = NewPacketFrom(4, []byte("address"))
// 		nss[6] = NewPacketFrom(3, []byte("2211 North First Street, San Jose"))
// 		nss[7] = NewPacketFrom(4, []byte(""))
// 		nss[8] = NewPacketFrom(22, []byte(""))
// 		nss[9] = NewPacketFrom(7, []byte("0"))
// 		ns = NewNetstringEmbedded(nss)
// 	}
// 	result = ns
// }


// func BenchmarkEncodeOne(b *testing.B) {
// 	var ns *Netstring
// 	for i := 0; i < b.N; i++ {
// 		ns = NewPacketFrom(25, []byte("select id, int_val, str_val from test where id = :account_id and name = :name and address = :address"))
// 	}
// 	result = ns
// }
//
// var results []*Netstring

func BenchmarkDecode(b *testing.B) {}
// func BenchmarkDecode(b *testing.B) {
// 	var nss2 []*Netstring
// 	nss := make([]*Netstring, 10)
// 	nss[0] = NewPacketFrom(25, []byte("select id, int_val, str_val from test where id = :account_id and name = :name and address = :address  /*12345-123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890-123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901*/"))
// 	nss[1] = NewPacketFrom(4, []byte("account_id"))
// 	nss[2] = NewPacketFrom(3, []byte("1234567890"))
// 	nss[3] = NewPacketFrom(4, []byte("name"))
// 	nss[4] = NewPacketFrom(3, []byte("John Smith"))
// 	nss[5] = NewPacketFrom(4, []byte("address"))
// 	nss[6] = NewPacketFrom(3, []byte("2211 North First Street, San Jose"))
// 	nss[7] = NewPacketFrom(4, []byte(""))
// 	nss[8] = NewPacketFrom(22, []byte(""))
// 	nss[9] = NewPacketFrom(7, []byte("0"))
// 	ns := NewNetstringEmbedded(nss)
// 	//	b.Log("Decoding:", len(ns.Serialized), ":", string(ns.Serialized))
// 	for i := 0; i < b.N; i++ {
// 		nss2, _ = SubNetstrings(ns)
// 	}
// 	results = nss2
// }
//
// func BenchmarkDecodeOne(b *testing.B) {
// 	var ns2 *Netstring
// 	ns := NewPacketFrom(25, []byte("select id, int_val, str_val from test where id = :account_id and name = :name and address = :address"))
// 	for i := 0; i < b.N; i++ {
// 		ns2, _ = NewPacket(strings.NewReader(string(ns.Serialized)))
// 	}
// 	result = ns2
// }

/* on hyper
BenchmarkEncode-24                 50000             29067 ns/op
BenchmarkEncodeOne-24             500000              3027 ns/op
BenchmarkDecode-24                200000             11055 ns/op
BenchmarkDecodeOne-24            1000000              1249 ns/op
*/
/* 1.10
goos: darwin
goarch: amd64
BenchmarkEncode-4      	  200000	      8188 ns/op
BenchmarkEncodeOne-4   	 2000000	       606 ns/op
BenchmarkDecode-4      	  500000	      2719 ns/op
BenchmarkDecodeOne-4   	 5000000	       319 ns/op
*/
/* 1.11
goos: darwin
goarch: amd64
BenchmarkEncode-4      	  200000	      6561 ns/op
BenchmarkEncodeOne-4   	 2000000	       638 ns/op
BenchmarkDecode-4      	  500000	      2771 ns/op
BenchmarkDecodeOne-4   	 5000000	       322 ns/op
*/
/* 1.12
goos: darwin
goarch: amd64
pkg: github.com/paypal/hera/utility/encoding/netstring
BenchmarkEncode-4      	  300000	      5160 ns/op
BenchmarkEncodeOne-4   	 3000000	       548 ns/op
BenchmarkDecode-4      	  500000	      2449 ns/op
BenchmarkDecodeOne-4   	 5000000	       299 ns/op
*/
