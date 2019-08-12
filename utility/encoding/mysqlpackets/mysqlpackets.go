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
	"io"
	"log"
	"fmt"
)

/* ==== CONSTANTS ============================================================*/
/* ---- STRING TYPES. ----------------------------------------------------------
* Strings are sequences of bytes and can be categorized into the following
* types below.
*     https://dev.mysql.com/doc/internals/en/string.html
* Note that VARSTR is currently NOT SUPPORTED.
*/
type string_t uint
const (
	EOFSTR string_t = iota   // rest of packet string
	NULLSTR                  // null terminated string
     FIXEDSTR                 // fixed length string with known hardcoded length
     VARSTR                   // variable length string
     LENENCSTR                // length encoded string prefixed with lenenc int
)

/* ---- Data sizes. ------------------------------------------------------------
* Integers can be stored in 1, 2, 3, 4, 6, or 8 bytes.
* The maximum packet size that can be sent between client and server
* is (1 << 24) - 1 bytes. The header size (of a packet) is always 4 bytes.
* See WritePacket() in connection.go for what a packet looks like.
*     https://dev.mysql.com/doc/internals/en/integer.html
*/
const (
     MAX_PACKET_SIZE     int = (1 << 24) - 1
     HEADER_SIZE         int = 4
     INT1                int = 1
     INT2                int = 2
     INT3                int = 3
     INT4                int = 4
     INT6                int = 6
     INT8                int = 8
)


/* ==== FUNCTIONS ============================================================*/

/* ---- HERA USE -------------------------------------------------------------*/
// MySQLPacket is a mysql protocol packet, which will have a command byte,
// payload, size, and sequence id.
type MySQLPacket struct {
	Cmd       	 int
	Length 	 	 int
	Sequence_id 	 int
	Serialized    []byte // the content section of a netstring. e.g. "xxx...yyy"
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
	ns := &MySQLPacket{}

	if (payloadLen == 0) {
		// throw error, maybe?
		return ns
	}

	// Read the command byte from the payload! ;)
	ns.Cmd = int(_payload[0])
	// Create the full packet which has the header and the payload.
	ns.Serialized = make([]byte, INT4 /* header length */ + payloadLen)
	ns.Length = payloadLen
	ns.Sequence_id = _cmd


	// Write in header
	idx := 0
	// 3 bytes indicating payload length
	WriteFixedLenInt(ns.Serialized, INT3, payloadLen, &idx)
	// 1 byte indicating the sequence_id
	WriteFixedLenInt(ns.Serialized, INT1, _cmd /* actually sequence id */, &idx)
	// Copy the payload
	copy(ns.Serialized[idx:], _payload)

	return ns
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

// MultiplePackets creates a new Reader and reads all of the incoming packets.
// It stores all packets in nss. A pointer to the 'current' packet that
// is being read is stored in 'ns'. next is the index of the current + 1 packet.
func (r *Reader) ReadMultiplePackets(_p *MySQLPacket) ([]*MySQLPacket, error) {
	// Initialize array of MySQLPackets
	var nss []*MySQLPacket

	// Variable for storing MySQLPacket
	var ns *MySQLPacket
	var err error

	// Add the first packet to the array of MySQLPackets
	nss = append(nss, _p)
	curr_sq := _p.Sequence_id

	// Keep reading from the connection until EOF
	for {
		curr_sq++
		ns, err = NewPacket(r.reader)
		// Multiple packets are sent by padding the payload until the length
		// is 0xffffff until we reach the last packet with length less
		// than 0xffffff.
		// https://dev.mysql.com/doc/internals/en/sending-more-than-16mbyte.html
		if ns.Length != MAX_PACKET_SIZE {
			break
		}
		if err != nil {
			return nil, err
		}
		// ns.Sequence_id = curr_sq ? might be unnecessary

		// Add the NewPacket to the next.
		nss = append(nss, ns)
	}
	return nss, nil
}


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
		// If packets have already been loaded into the reader,
		// return the one that is currently being pointed to
		if reader.ns != nil {

			ns = reader.ns
			reader.ns = nil
			reader.next++
			return
		}

		// Otherwise, move to the next packet in the packet array
		if reader.next < len(reader.nss) {

			ns = reader.nss[reader.next]
			reader.next++
			return
		}

		// If there are no packets, read them in from the connection.
		reader.ns, err = NewPacket(reader.reader)

		if err != nil {
			return nil, err
		}

		// Check for packet size of the first incoming packet.
		if (reader.ns.Length == MAX_PACKET_SIZE) {

			// This means there are more packets on the way.
			reader.nss, err = reader.ReadMultiplePackets(reader.ns)
			if err != nil {
				return nil, err
			}

			// Start at the 0th packet when the next loop comes around so
			// the reader returns the first packet.
			reader.next = 0
		}
	}
}


// IsComposite returns if the MySQLPacket has more packets following it,
// i.e. the payload length is 0xffffff.
func (ns *MySQLPacket) IsComposite() bool {
	idx := 0
	length := ReadFixedLenInt(ns.Serialized, INT3, &idx)
	return length == MAX_PACKET_SIZE
}


/*---- MISC. FUNCTIONS ---------------------------------------------------------
* Miscellaneous functions that perform common operations. Includes mostly
* arithmetic.
*/

/* min returns the minimum of two functions. */
func min(a int, b int) (int) {
     if a < b { return a }
     return b
}

/* Checks bitmask capability flag against server/client/connection capabilities
* and returns true if the bit is set, otherwise false.
*/
func Supports(cflags uint32, c int) (bool) {
     if (cflags & uint32(c)) != 0 {
          return true
     }
     return false
}

/*  (tentative if this is needed) *******
* Checks that size of slice is enough for the incoming data. */
func checkSize(sz1 int, sz2 int) {
	if sz1 < sz2 {
		log.Fatal(fmt.Sprintf("Array size %d, expected %d", sz1, sz2))
	}
}



/*---- WRITING BASIC DATA ------------------------------------------------------
* There are three functions. They are mostly useful in writing communication
* packets.
* There are two basic data types in the MySQL protocol: integers and strings.
* An integer is either fixed length or length encoded.
* There are two separate methods for integers.
*
* For strings, there is one method, but you must provide the type of string
* being written as a method argument.
*/

/* Writes an unsigned integer n as a fixed length integer int<l> into the slice
* data. The intptr pos keeps track of where in the buffer (data) we are
* before and after writing to the buffer.
*/
func WriteFixedLenInt(data []byte, l int, n int, pos *int) {
     // Check that the length of data is enough to accomodate the length
     // of the encoding.
	// checkSize(len(data[*pos:]), l)
	switch l {
          case INT8:
			data[*pos + 7] = byte(n >> 56)
			data[*pos + 6] = byte(n >> 48)
			fallthrough
		case INT6:
			data[*pos + 5] = byte(n >> 40)
			data[*pos + 4] = byte(n >> 32)
			fallthrough
		case INT4:
			data[*pos + 3] = byte(n >> 24)
			fallthrough
		case INT3:
			data[*pos + 2] = byte(n >> 16)
			fallthrough
		case INT2:
			data[*pos+1] = byte(n >> 8)
			fallthrough
		case INT1:
			data[*pos] = byte(n)
		default:
               // if log.V(logger.Warning) {
               //      log.Log(logger.Warning,
               //           fmt.Sprintf("Unexpected fixed int size %d", l))
               // }
			log.Fatal(fmt.Sprintf("Unexpected size %d", l))
	}

     // Move the index tracker.
	*pos += l
}

/* Writes an unsigned integer n as a length encoded integer
* into the slice data. Checks that the data byte array is big enough.
* The intptr pos keeps track of where in the buffer (data) we are
* before and after writing to the buffer.
*/
func WriteLenEncInt(data []byte, n uint64, pos *int) {
     // Determine the length encoded integer.
	l := 1
	if n >= 251 && n < (1 << 16) {
		l = 3
	} else if n >= (1 << 16) && n < (1 << 24) {
		l = 4
	} else if n >= (1 << 24) {
		l = 9
	}

     // Check that the data byte array is big enough for the desired length.
	// checkSize(len(data[*pos:]), l)

     // Write the length encoded integer into the data array.
	if l == 1 {
		WriteFixedLenInt(data, l, int(n), pos)
	} else {
		switch l {
			case 3:
				data[*pos] = byte(0xfc)
			case 4:
				data[*pos] = byte(0xfd)
			case 9:
				data[*pos] = byte(0xfe)
		}
		*pos++
		WriteFixedLenInt(data, l-1, int(n), pos)
	}

}

/* Writes a string str into the slice data. The method of writing is different
* depending on the string type. l is supposed to be an optional argument
* for when the length needs to specified (i.e. FIXEDSTR, EOFSTR). The intptr
* pos keeps track of where in the buffer (data) we are before and after writing
* to the buffer.
*/
func WriteString(data []byte, str string, stype string_t, pos *int, l int) {
     switch stype {
          case NULLSTR:
               // checkSize(len(data[*pos:]), len(str))
               // Write the string and then terminate with 0x00 byte.
               copy(data[*pos:], str)
               // checkSize(len(data[*pos:]), len(str) + 1)
               *pos += len(str)
               data[*pos] = 0x00
               *pos++

          case LENENCSTR:
               // Write the encoded length.
               WriteLenEncInt(data, uint64(len(str)), pos)
               // Then write the string as a FIXEDSTR.
               WriteString(data, str, FIXEDSTR, pos, l)

          case FIXEDSTR:

               // checkSize(len(data[*pos:]), l)
               // Pads the string with 0's to fill the specified length l.
               copy(data[*pos:*pos+l], str)
               *pos += l

          case EOFSTR:

               // checkSize(len(data[*pos:]), len(str))
               // Copies the string into the data.
               *pos += copy(data[*pos:], str)
     }
}

/*---- READING DATA ------------------------------------------------------------
* These functions are mostly useful in reading communication packets. There
* are two functions for integer types, and one function for all string types.
*/

/* Reads an unsigned integer n as a fixed length integer
* int<l> from the slice data. The intptr pos keeps track of where in the buffer
* (data) we are before and after writing to the buffer.
* Basically what happens is that this bit-shifts the elements accordingly
* and bit-wise ORs all of them together to get the original integer back.
*/
func ReadFixedLenInt(data []byte, l int, pos *int) int {
	checkSize(len(data[*pos:]), l)
     n := uint(0)
	switch l {
          case INT8:
			n |= uint(data[*pos + 7]) << 56
			n |= uint(data[*pos + 6]) << 48
			fallthrough
		case INT6:
			n |= uint(data[*pos + 5]) << 40
			n |= uint(data[*pos + 4]) << 32
			fallthrough
		case INT4:
			n |= uint(data[*pos + 3]) << 24
			fallthrough
		case INT3:
			n |= uint(data[*pos + 2]) << 16
			fallthrough
		case INT2:
			n |= uint(data[*pos+1]) << 8
			fallthrough
		case INT1:
			n |= uint(data[*pos])
		default:
			log.Fatal(fmt.Sprintf("Unexpected size %d", l))
	}
	*pos += l

     return int(n)
}


/* Reads an unsigned integer n as a length encoded integer
* from the slice data. */
func ReadLenEncInt(data []byte, pos *int) int {
     l := 0         // length of the length encoded integer

     // Check the first byte to determine the length.
     fb := byte(data[*pos])

     // If the first byte is < 0xfb, then l = 1.
     if fb < 0xfb {
          l = 1
     }

	if l == 1 {
          // Read 1 byte for lenenc<1>.
		return ReadFixedLenInt(data, INT1, pos)
	}

	*pos++

     // Otherwise read the appropriate length according to the
     // encoded length.
	switch fb {
		case 0xfc: // 2-byte integer
               return ReadFixedLenInt(data, INT2, pos)
		case 0xfd: // 3-byte integer
			return ReadFixedLenInt(data, INT3, pos)
          default : // 8-byte integer
               return ReadFixedLenInt(data, INT8, pos)
	}
}


/* Reads a string str from the slice data. The method of reading is different
* depending on the string type. l is supposed to be an optional argument
* for when the length needs to specified (i.e. FIXEDSTR (specified), and
* EOFSTR, where the length of the string to be read in is calculated from
* current position and remaining length of packet).
*/
func ReadString(data []byte, stype string_t, pos *int, l int) string {
     buf := bytes.NewBuffer(data[*pos:])
     switch stype {
          case NULLSTR:
               line, err := buf.ReadBytes(byte(0x00))
     		if err != nil {
                    // if log.V(logger.Warning) {
          		// 	log.Log(logger.Warning, err)
          		// }
     		    log.Fatal(err)
     		}
               *pos += len(line)
               return string(line)

          case LENENCSTR:
               n := ReadLenEncInt(data, pos)
               if n == 0 {
                    break
               }
               buf.ReadByte()
               temp := make([]byte, n)
               n2, err := buf.Read(temp)
               if err != nil {
                    // log.Fatal(err)
                    // if log.V(logger.Warning) {
          		// 	log.Log(logger.Warning, err)
          		// }
               } else if n2 != n {
                    // if log.V(logger.Warning) {
          		// 	log.Log(logger.Warning,
                    //            fmt.Sprintf("Read %d, expected %d", n2, n))
          		// }
                    // log.Fatal(fmt.Sprintf("Read %d, expected %d", n2, n))
               }
               *pos += n
               return string(temp)

          case FIXEDSTR, EOFSTR:
               temp := make([]byte, l)
               n2, err := buf.Read(temp)
               if err != nil {
                    // log.Fatal(err)
                    // if log.V(logger.Warning) {
          		// 	log.Log(logger.Warning, err)
          		// }
               } else if n2 != l {
                    // if log.V(logger.Warning) {
          		// 	log.Log(logger.Warning,
                    //            fmt.Sprintf("Read %d, expected %d", n2, l))
          		// }
                    // log.Fatal(fmt.Sprintf("Read %d, expected %d", n2, l))
               }
               *pos += l
               return string(temp)
     }
     return ""
}
