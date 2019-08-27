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
	"errors"
	"fmt"
	"github.com/paypal/hera/utility/encoding"
	"io"
	"log"
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
	VARSTR                   // variable length string -- as of right now, unused
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

const (
	CLIENT_LONG_PASSWORD                    int = 1 << (iota)
	CLIENT_FOUND_ROWS
	CLIENT_LONG_FLAG
	CLIENT_CONNECT_WITH_DB
	CLIENT_NO_SCHEMA
	CLIENT_COMPRESS
	CLIENT_ODBC
	CLIENT_LOCAL_FILES
	CLIENT_IGNORE_SPACE
	CLIENT_PROTOCOL_41
	CLIENT_INTERACTIVE
	CLIENT_SSL
	CLIENT_IGNORE_SIGPIPE
	CLIENT_TRANSACTIONS
	CLIENT_RESERVED
	CLIENT_RESERVED2
	CLIENT_MULTI_STATEMENTS
	CLIENT_MULTI_RESULTS
	CLIENT_PS_MULTI_RESULTS
	CLIENT_PLUGIN_AUTH
	CLIENT_CONNECT_ATTRS
	CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA
	CLIENT_CAN_HANDLE_EXPIRED_PASSWORDS
	CLIENT_SESSION_TRACK
	CLIENT_DEPRECATE_EOF
	CLIENT_SSL_VERIFY_SERVER_CERT 	    int = 1 << 30
	CLIENT_OPTIONAL_RESULTSET_METADATA     int = 1 << 25
	CLIENT_REMEMBER_OPTIONS	              int = 1 << 31
)

type Packager struct {
	reader 		io.Reader
	writer 		io.Writer
	sqid 		int			// Keeps track
}


/* ==== FUNCTIONS ============================================================*/

/* ---- HERA USE -------------------------------------------------------------*/
// Creates a Packet from the reader, reading exactly as many
// bytes as necessary. Assumes that the encoding.Packet being read is a COMMAND PACKET
// only. Used for incoming requests from client.
func NewInitSQLPacket(_reader io.Reader) (*encoding.Packet, error) {
	ns := &encoding.Packet{}

	var tmp = make([]byte, INT4)
	var err error

	// Read in the header
	_, err = _reader.Read(tmp)

	// A MySQL packet is formatted such that there is a four header
	// storing length of the payload (3 bytes little endian) and sequence id (1 byte)
	idx := 0
	// Encode payload_length
	payloadLength := ReadFixedLenInt(tmp, INT3, &idx)
	// Encode sequence id
	sqid := ReadFixedLenInt(tmp, INT1, &idx)

	if payloadLength == 0 {
		return nil, nil
	}

	// The total length is the header + payload, given by buff.Len() + payload
	// length read from the packet
	totalLen := payloadLength + HEADER_SIZE
	ns.Length = payloadLength
	ns.Sqid = sqid
	ns.Serialized = make([]byte, totalLen + 1)
	bytesRead := 1
	// Copy the header over into ns.Serialized
	copy(ns.Serialized[bytesRead:], tmp)
	// Mark number of bytes already read
	bytesRead += len(tmp)

	// Read in the payload
	var n int
	for bytesRead < totalLen + 1 {
		n, err = _reader.Read(ns.Serialized[bytesRead:])
		if err != nil {
			return nil, err
		}
		bytesRead += n
	}
	if bytesRead - 1 != totalLen {
		return nil, errors.New(fmt.Sprintf("Expected %d bytes, instead got %d,", totalLen, bytesRead - 1))
	}

	// Read command byte, which is the first byte after the header
	ns.Cmd = int(ns.Serialized[HEADER_SIZE+1])
	ns.Payload = ns.Serialized[HEADER_SIZE+1:]
	ns.IsMySQL = true

	return ns, nil
}

// Creates a Packet from the reader, reading exactly as many
// bytes as necessary. Assumes that the encoding.Packet being read is a COMMAND PACKET
// only. Used for internal Hera communication only.
func NewMySQLPacket(_reader io.Reader) (*encoding.Packet, error) {
	ns := &encoding.Packet{}

	var ptype = make([]byte, INT1)
	var tmp = make([]byte, INT4)
	var err error

	// Read in the indicator byte
	_, err = _reader.Read(ptype)

	// Check packet indicator byte.
	if len(ptype) != 0 && ptype[0] != 0 {
		if int(ptype[0]) == 1 {
			return nil, encoding.WRONGPACKET
		}
		return nil, encoding.UNKNOWNPACKET
	}

	// Read the header into tmp
	_, err = _reader.Read(tmp)

	idx := 0
	// Encode payload_length
	payload_length := ReadFixedLenInt(tmp, INT3, &idx)
	// Encode sequence id
	sqid := ReadFixedLenInt(tmp, INT1, &idx)


	// The total length is the header + payload, given by HEADER_SIZE + payload
	// length read from the packet
	totalLen := payload_length + HEADER_SIZE
	ns.Length = payload_length
	// Sequence id is as specified by the header
	ns.Sqid = sqid

	ns.Serialized = make([]byte, totalLen + 1) // + 1 is for the indicator byte
	ns.Serialized[0] = ptype[0]

	bytesRead := 1
	// Copy the header over into ns.Serialized
	copy(ns.Serialized[bytesRead:], tmp)
	// Mark number of bytes already read
	bytesRead += len(tmp)

	// Read in the payload
	var n int
	for bytesRead < totalLen + 1 {
		n, err = _reader.Read(ns.Serialized[bytesRead:])
		if err != nil {
			return nil, err
		}
		bytesRead += n
	}
	if (bytesRead - 1) != totalLen {
		return nil, errors.New(fmt.Sprintf("Expected %d bytes, instead got %d,", totalLen, bytesRead - 1))
	}

	// Read command byte, which is the first byte after the header
	ns.Cmd = int(ns.Serialized[HEADER_SIZE + 1])
	// Set the payload of the packet.
	ns.Payload = ns.Serialized[HEADER_SIZE + 1:]

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
func NewMySQLPacketFrom(sqid int, _payload []byte) *encoding.Packet {

	// Grab the payload length
	payloadLen := len(_payload)

	// Create an empty encoding.Packet
	ns := &encoding.Packet{}

	if payloadLen == 0 {
		// throw error, maybe?
		return ns
	}

	// Read the command byte from the payload! ;)
	ns.Cmd = int(_payload[0])

	// Create the full packet which has the header and the payload.
	ns.Serialized = make([]byte, INT4 /* header length */ + payloadLen + 1)
	ns.Serialized[0] = 0 				// to indicate MySQLPacket
	ns.Length = payloadLen
	ns.Sqid = sqid
	ns.Payload = _payload
	ns.IsMySQL = true

	// Write in header
	idx := 1
	// 3 bytes indicating payload length
	WriteFixedLenInt(ns.Serialized, INT3, payloadLen, &idx)
	// 1 byte indicating the sequence_id
	WriteFixedLenInt(ns.Serialized, INT1, sqid, &idx)
	// Copy the payload
	copy(ns.Serialized[idx:], _payload)

	return ns
}

// Write multiple (or one) packets. Copied this over from mocksqlsrv WritePacket code.
func (p *Packager) WritePacket(_payload []byte) ([]*encoding.Packet, error) {

	/* Set current payload length. */
	length := len(_payload) 	// Keeps track of the remaining length to be written in _payload
	pidx := 0					// Keeps track of reading position in _payload

	numPackets := 0

	var packets []*encoding.Packet

	for length > 0 {
		/* Determine packetLength, capped by MAX_PACKET_SIZE. */
		packetsize := min(length, MAX_PACKET_SIZE)
		numPackets++

		packets = append(packets, NewMySQLPacketFrom(p.sqid, _payload[pidx:pidx+packetsize]))

		pidx += packetsize
		if pidx > len(_payload) {
			return packets, errors.New("Index range exceeds payload; length out of bonds")
		}

		length -= packetsize
		p.sqid++
	}

	return packets, nil
}

// NewPacketReader creates a Reader, that maintains the state / aka sequence_id
// for packets sent to the server
func NewPackager(_reader io.Reader, _writer io.Writer) *Packager {
	return &Packager{reader:_reader, writer:_writer}
}


// ReadNext returns the next packet from the stream.
// Note: in case of multiple packets bigger than 16 MB the Reader will buffer
// some packets, a different function will probably have to be used. This is
// just for grabbing one packet from the stream. encoding.Packets are not embedded.
func (p *Packager) ReadNext() (ns *encoding.Packet, err error) {
	// Read in a packet from the packager's reader.
	pkt, err := NewMySQLPacket(p.reader)
	if err != nil {
		return nil, err
	}
	// Set the sequence id to what is already in the packet
	p.sqid = pkt.Sqid
	return pkt, err
}
//
//func (p *Packager) COM_STMT_RESPONSE_OK() ([]*encoding.Packet, err error) {
//	// TODO: COM_STMT_RESPONSE_OK is the sequence of packets that is sent back to the client after COM_STMT_PREPARE
//}

// Result sets function
// https://dev.mysql.com/doc/dev/mysql-server/8.0.12/page_protocol_com_query_response_text_resultset_column_definition.html
// This is specifically for ColumnDefinition41 packets.
//func (p *Packager) ReconstructColumnDefinition() ([]*encoding.Packet, err error) {
//
//}

//
//// Result sets function
//func (p *Packager) ReconstructResultsetRow() ([]*encoding.Packet, err error) {
//	// TODO: Write code out for sending binary protocol result sets and COM_QUERY_RESPONSE, and the old fashioned result sets too.
//}


/*---- COMMON PACKETS ----------------------------------------------------------
* Packets that are frequently used, like ERR packet or OK packet or EOF packet
* are written below.
 */

// https://dev.mysql.com/doc/internals/en/packet-OK_Packet.html
func OKPacket(affectedRows int, lastInsertId int, msg string) []byte {
	payload := make([]byte, 1 + calculateLenEnc(uint64(affectedRows)) + calculateLenEnc(uint64(lastInsertId)))
	pos := 0
	// Write OK packet header
	WriteFixedLenInt(payload, INT1, 0x00, &pos)

	// Write affected_rows
	WriteLenEncInt(payload, uint64(affectedRows), &pos)
	// Write last_insert_id
	WriteLenEncInt(payload, uint64(lastInsertId), &pos)

	/* There's several things to do with client capabilities....that are all ignored
	*
	*  if capabilities & CLIENT_PROTOCOL_41 { write status_flags int<2> and warnings int<2>}
	*  elseif capabilities & CLIENT_TRANSACTIONS { status_flags <2> }
	*  if capabilities & CLIENT_SESSION_TRACK { info string<lenenc> ;
	*     if status_flags & SERVER_SESSION_STATE_CHANGED { session_state_changes string<lenenc> }
	*  }
	*  else { do what is written below }
	 */
	WriteString(payload, msg, EOFSTR, &pos, 0)
	return payload
}

// https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html
func ERRPacket(errcode int, msg string) []byte {
	payload := make([]byte, 1 + 2 + len(msg))
	pos := 0
	// Write ERR packet header
	WriteFixedLenInt(payload, INT1, 0xff, &pos)
	// Write error code
	WriteFixedLenInt(payload, INT2, errcode, &pos)
	/* There's one thing to do with client capabilities....that are all ignored
	*
	*  if capabilities & CLIENT_PROTOCOL_41 { write sql_state_marker string<1> and sql_state string<5>}
	 */

	// Write human readable error message
	WriteString(payload, msg, EOFSTR, &pos, 0)
	return payload
}

// https://dev.mysql.com/doc/internals/en/packet-EOF_Packet.html
func EOFPacket() []byte {
	payload := make([]byte, 1)
	pos := 0
	// Write EOF packet header
	WriteFixedLenInt(payload, INT1, 0xfe, &pos)
	// if capabilities & CLIENT_PROTOCOL_41 { warnings int<2>, status_flags <int2> }
	return payload
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

func calculateLenEnc(n uint64) int {
	// Determine the length encoded integer.
	l := 1
	if n >= 251 && n < (1 << 16) {
		l = 3
	} else if n >= (1 << 16) && n < (1 << 24) {
		l = 4
	} else if n >= (1 << 24) {
		l = 9
	}
	return l
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
	checkSize(len(data[*pos:]), l)
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
	l := calculateLenEnc(n)

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