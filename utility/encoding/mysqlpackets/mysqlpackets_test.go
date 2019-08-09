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
	"math/rand"
	"testing"
	"bytes"
	"github.com/paypal/hera/common"
	"github.com/paypal/hera/utility/encoding"
	"reflect"
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

// Tests whether or not NewPacket properly reads in a single packet
// from a buffered reader
func TestBasic(t *testing.T) {
	t.Log("Start TestBasic ++++++++++++++")

	tcase(tmake(), t)

	t.Log("End TestBasic ++++++++++++++")
}

// Tests whether or not packets get their headers properly prepended
// before they're written out to the net.Conn for the client.
func TestNewPacketFrom(t *testing.T) {

	t.Log("Start TestNewPacketFrom +++++++++++++")
	// Get those go-to queries
	tcases := tmake()

	for _, tcase := range tcases {
		t.Log("Testing for: ", tcase.Serialized)
		ns := NewPacketFrom(0, tcase.ns.Payload)
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

	t.Log("End TestNewPacketFrom +++++++++++++")

}

/* Tests the read next function which reads multiple packets from a stream. */
func TestReadNext(t *testing.T) {
	t.Log("Start TestReadNext +++++++++++++")

	// Pick random number of packets to be 'sent' over the reader
	numPackets := rand.Intn(48) + 2 		// Rand between 2 and 50

	// Pick length of terminal packet + header
	endPacketLength := rand.Intn(MAX_PACKET_SIZE - 1)

	// Create expected test packet! Note that everything is all 0s
	buf := make([]byte, MAX_PACKET_SIZE)
	expectedPacket := NewPacketFrom(0, buf) // Stream packet


	buf = make([]byte, endPacketLength)
	endPacket := NewPacketFrom(numPackets - 1, buf) // Terminal packet

	t.Log("Running with ", numPackets, " packets and ", endPacketLength, " length end packet")

	big_payload := make([]byte, 0)
	idx := 0
	for i := 0; i < numPackets; i++ {
		big_payload = append(big_payload, expectedPacket.Serialized...)
		expectedPacket.Serialized[3]++
		t.Log(expectedPacket.Serialized[3])
		idx += expectedPacket.Length
	}
	big_payload = append(big_payload, endPacket.Serialized...)
	if (len(big_payload) != numPackets * (MAX_PACKET_SIZE + 4) + endPacketLength + 4) {
		t.Log("Unexpected big payload length ", len(big_payload))
	}

	// Reset sequence id
	expectedPacket.Serialized[3] = 0

	// Create a new packet reader
	reader := NewPacketReader(bytes.NewReader(big_payload))

	// Since we have two packets, use a general variable for test packet
	var testPacket *MySQLPacket

	// Return the next packet from the string!
	for {
		t.Log("reader.ReadNext() in mysql_packets test")
		ns, err := reader.ReadNext()
		if err != nil {
			break
		}
		if ns.Length != MAX_PACKET_SIZE {
			testPacket = endPacket
		} else {
			testPacket = expectedPacket
		}
		t.Log("Packet number: ", expectedPacket.Serialized[3])

		// Test that the next packet read is as expected!
		if ns.Length != testPacket.Length {
			t.Log("Length expected", testPacket.Length, "instead got", ns.Length)
		}
		if ns.Sequence_id != testPacket.Sequence_id {
			t.Log("Sequence id expected", testPacket.Sequence_id, "instead got", ns.Sequence_id)
		}
		if ns.Cmd != testPacket.Cmd {
			t.Log("Command expected", testPacket.Cmd, "instead got", ns.Cmd)
			t.Fail()
		}
		if !reflect.DeepEqual(ns.Payload, testPacket.Payload) {
			t.Log("Payload expected", testPacket.Payload, "instead got", ns.Payload)
			// t.Log("Wrong payload")
			t.Fail()
		}
		if !reflect.DeepEqual(ns.Serialized, testPacket.Serialized) {
			t.Log("Serialized expected", testPacket.Serialized, "instead got", ns.Serialized)
			// t.Log("Wrong serialized")
			t.Fail()
		}
		expectedPacket.Serialized[3]++
		expectedPacket.Sequence_id++
	}

	if int(expectedPacket.Serialized[3]) != numPackets {
		t.Log("Expected number of packets", numPackets, "instead got", int(expectedPacket.Serialized[3]))
		t.Fail()
	}

	t.Log("End TestReadNext +++++++++++++")
}

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
