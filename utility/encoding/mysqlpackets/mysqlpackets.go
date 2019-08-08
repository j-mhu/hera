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

// Package mysqlpackets contains encoding and decoding functions in MySQL protocol
// format
package mysqlpackets

import (
	"bytes"
	// "errors"
	// "fmt"
	"io"
	"github.com/paypal/hera/utility/encoding"
)

// const (
//
// )

// MySQLPacket is a mysql protocol packet, which will have a command byte,
// payload, size, and sequence id.
type MySQLPacket struct {
	encoding.Packet			// Cmd, Serialized, Payload

}

// Reader decodes MySQL protocol command packets from a buffer or stores information
// about the state of the sequence id when packets are exchanged between
// server and client.
type Reader struct {
     reader  io.Reader
     ns  *MySQLPacket
     nss []*MySQLPacket
     next    int
}

// NewPacket creates a MySQLPacket from the reader, reading exactly as many
// bytes as necessary. Assumes that the packet being read is a COMMAND PACKET
// only.
func NewPacket(_reader io.Reader) (*MySQLPacket, error) {
	ns := &MySQLPacket{}

	var buff bytes.Buffer
	var tmp = make([]byte, INT4)
	var err error

	// A MySQL packet is formatted such that there is a four header
	// storing length of the payload (3 bytes little endian) and sequence id (1 byte)
	_, err = _reader.Read(tmp)
	if err != nil {
		return nil, err
	}

	idx := 0
	length := ReadFixedLenInt(tmp, INT3, &idx)
	sqid := ReadFixedLenInt(tmp, INT1, &idx)

	buff.Write(tmp)

	// The total length is the header + payload, given by buff.Len() + payload
	// length read from the packet
	totalLen := length + INT4
	ns.Length = length
	ns.Sequence_id = sqid
	ns.Serialized = make([]byte, totalLen)
	// Copy the header over into ns.Serialized
	copy(ns.Serialized, buff.Bytes())
	// Mark number of bytes already read
	bytesRead := buff.Len()

	// Read in the payload
	var n int
	for bytesRead < totalLen {
		n, err = _reader.Read(ns.Serialized[INT4:])
		if err != nil {
			return nil, err
		}
		bytesRead += n
	}

	// Read command byte, which is the first byte after the header
	next := buff.Len()
	ns.Cmd = int(ns.Serialized[next])

	// Pack the entire payload into the payload field of the MySQLPacket
	ns.Payload = ns.Serialized[next:]
	return ns, nil
}

// NewPacketFrom creates a packet from command and payload.
// Although, I don't know when this would ever be used by the server, but maybe
// it will be of use from the client!
/* NOTE: READ THIS
* There's some sorcery behind the scenes here. In the netstring implementation
* of the Packaging interface, the argument passed in for _cmd is genuinely
* used as a command int / opcode. However, since MySQL packets contain
* the command byte already in the payload and NewPacketFrom is likely to be
* used for /sending/ packets instead, we've jury-rigged this so that it
* follows the interface but allows us to keep track of the sequence_id (which
* notably netstring doesn't have). Then we can return other kinds of response
* packets! So note that NewPacketFrom is used by the server to construct
* packets to send to the client.
*/
func NewPacketFrom(_cmd int, _payload []byte) *MySQLPacket {

	payloadLen := len(_payload)
	ns := new(MySQLPacket)

	if (payloadLen == 0) {
		// throw error, maybe?
		return &MySQLPacket{}
	}

	// Read the command byte from the payload! ;)
	ns.Cmd = int(_payload[0])
	// Assign the payload
	ns.Payload = _payload
	// Create the full packet which has the header and the payload.
	ns.Serialized = make([]byte, INT4 /* header length */ + payloadLen)

	// Write in header
	idx := 0
	// 3 bytes indicating payload length
	WriteFixedLenInt(ns.Serialized, payloadLen, INT3, &idx)
	// 1 byte indicating the sequence_id
	WriteFixedLenInt(ns.Serialized, _cmd /* actually sequence id */, INT1, &idx)
	// Copy the payload
	copy(ns.Serialized[idx:], ns.Payload)
	return ns
}

/* BEGIN: MAY NOT BE RELEVANT TO MySQLPackets. */

// NewPacketEmbedded embedds a set of packets into a netstring
// func NewNetstringEmbedded(_netstrings []*Netstring) *Netstring {
// 	// TODO: optimize
// 	payloadLen := 0
// 	for _, i := range _netstrings {
// 		payloadLen += len(i.Serialized)
// 	}
// 	lenStr := fmt.Sprintf("%d:", payloadLen+2 /*len("0 ")*/)
// 	totalLen := len(lenStr) + payloadLen + 2 /*len("0 ")*/ + 1 /*ending comma*/
// 	ns := new(Netstring)
// 	ns.Serialized = make([]byte, totalLen)
// 	copy(ns.Serialized, []byte(lenStr))
// 	next := len(lenStr)
// 	copy(ns.Serialized[next:], []byte{CodeSubCommand, space})
// 	next += 2
// 	for _, i := range _netstrings {
// 		copy(ns.Serialized[next:], i.Serialized)
// 		next += len(i.Serialized)
// 	}
// 	ns.Serialized[next] = byte(comma)
// 	ns.Payload = ns.Serialized[totalLen-payloadLen-1 : totalLen-1]
// 	return ns
// }

// // SubNetstrings parses incoming
// func SubNetstrings(_ns *Netstring) ([]*Netstring, error) {
// 	//  TODO: optimize for zero-copy
// 	var nss []*Netstring
// 	reader := bytes.NewReader(_ns.Payload)
// 	var ns *Netstring
// 	var err error
// 	for {
// 		ns, err = NewPacket(reader)
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return nil, err
// 		}
// 		nss = append(nss, ns)
// 	}
// 	return nss, nil
// }
/* END: MAY NOT BE RELEVANT TO MySQLPackets. */


// NewPacketReader creates a Reader, that maintains the state / aka sequence_id
// for packets sent to the server
func NewPacketReader(_reader io.Reader) *Reader {
	nsr := new(Reader)
	nsr.reader = _reader
	return nsr
}

// ReadNext returns the next packet from the stream.
// Note: in case of multiple packets bigger than 16 MB the Reader will buffer
// some packets, a different function will probably have to be used. This is
// just for grabbing one packet from the stream. MySQLPackets are not embedded.
func (reader *Reader) ReadNext() (ns *MySQLPacket, err error) {
	for {
		if reader.ns != nil {
			ns = reader.ns
			reader.ns = nil
			return
		}
		if reader.next < len(reader.nss) {
			ns = reader.nss[reader.next]
			reader.next++
			return
		}

		reader.ns, err = NewPacket(reader.reader)
		if err != nil {
			return nil, err
		}

		// .....skeleton code that might be fixed later to include
		// data spread across multiple packets
		// var more *MySQLPacket
		//
		// if (reader.ns.length == 0xffffff) {
		// 	for {
		// 		more, err = NewPacket(reader.reader)
		// 		if err != nil {
		// 			return nil, err
		// 		}
		// 		if err == io.EOF {
		// 			break
		// 		}
		// 		reader.nss.append(reader.nss, more)
		// 		reader.next++
		// 		if more.length != 0xffffff {
		// 			break
		// 		}
		// 	}
		// }
	}
}

// IsComposite returns if the MySQLPacket has more packets following it,
// i.e. the payload length is 0xffffff.
func (ns *MySQLPacket) IsComposite() bool {
	idx := 0
	length := ReadFixedLenInt(ns.Serialized, INT3, &idx)
	return length == 0xffffff
}
