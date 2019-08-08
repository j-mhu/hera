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
     // "github.com/paypal/hera/utility/logger"

     // "bufio"
     "fmt"
     // "net"
     // "os"
     // "strings"
     // "time"
     "bytes"
     "log"
)

// var log = logger.GetLogger()

/* ==== UTILS ==================================================================
* Utils defines constants for all the MySQL protocol packets, basic types
* (fixed length integers, EOF strings, etc.), and capability flag bitfields.
* It provides functions for encoding them into connection phase packets.
*/

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
     // MAX_PACKET_SIZE     int = (1 << 24) - 1
     HEADER_SIZE         int = 4
     INT1                int = 1
     INT2                int = 2
     INT3                int = 3
     INT4                int = 4
     INT6                int = 6
     INT8                int = 8
)

/* ---- CAPABILITY FLAGS -------------------------------------------------------
* Bitwise '|' the desired capabilities together
* when configuring the server, like this, to give the server those capabilities:
*
*   capabilities := CLIENT_LONG_PASSWORD | CLIENT_SSL | CLIENT_PROTOCOL_41
*
* Since this is a dummy server, none of the capabilities will actually 'mean'
* or 'do' anything. You should just set the minimum flags so that
* the client is compatible.
*
* https://dev.mysql.com/doc/internals/en/capability-flags.html#packet-Protocol::CapabilityFlags
*/
type cflag uint
const (
     CLIENT_LONG_PASSWORD                    cflag = 1 << (iota)
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
     CLIENT_SSL_VERIFY_SERVER_CERT 	    cflag = 1 << 30
     CLIENT_OPTIONAL_RESULTSET_METADATA     cflag = 1 << 25
     CLIENT_REMEMBER_OPTIONS	              cflag = 1 << 31
)

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
func Supports(cflags uint32, c cflag) (bool) {
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
//
// /*---- CONNECTION PHASE UTILS --------------------------------------------------
// * These functions are the bulk of the connection phase. Some important
// * functions include sendHandshake(), receiveHandshakeResponse(),
// * sendOKPacket, etc. They are divided by reading / writing from the
// * connection with the client.
// */
//
// // READING .....................................................................
//
//
//
//
//
//
// // WRITING .....................................................................
//
// /* Sends handshake over connection. Only writes Handshakev10 packets. */
// func (c *Conn) sendHandshake(p packet_t) {
//      scramble := "ham&eggs" // temporary authentication plugin data
//      pos := 0
//      switch p {
//           case HANDSHAKEv10:
//                // protocol version
//                WriteFixedLenInt(c.writeBuf, INT1, 0xa, &pos)
//
//                // server version
//                WriteString(c.writeBuf, c.server_ver, NULLSTR, &pos, 0)
//
//                // thread id
//                WriteFixedLenInt(c.writeBuf, INT4, c.connection_id, &pos)
//
//                // Write first 8 bytes of plugin provided data (scramble)
//                WriteString(c.writeBuf, scramble, FIXEDSTR, &pos, 8)
//
//                // filler
//                WriteFixedLenInt(c.writeBuf, INT1, 0x00, &pos)
//
//                // capability_flags_1
//                WriteFixedLenInt(c.writeBuf, INT2, int(c.cflags), &pos)
//
//                // character_set
//                WriteFixedLenInt(c.writeBuf, INT1, 0xff, &pos)
//
//                // status_flags
//                WriteFixedLenInt(c.writeBuf, INT2, 0x00, &pos)
//
//                // capability_flags_2
//                WriteFixedLenInt(c.writeBuf, INT2, int(c.cflags) >> 16, &pos)
//
//                if isFlagSet(c.cflags, CLIENT_PLUGIN_AUTH) {
//                     // authin_plugin_data_len. Temp: 0xaa
//                     WriteFixedLenInt(c.writeBuf, INT1, 0xaa, &pos)
//                } else {
//                     // 00
//                     WriteFixedLenInt(c.writeBuf, INT1, 0x00, &pos)
//                }
//                // reserved
//                WriteString(c.writeBuf, strings.Repeat("0", 10), FIXEDSTR, &pos, 10)
//
//                // auth-plugin-data-part-2
//                WriteString(c.writeBuf, scramble, LENENCSTR, &pos, 13)
//
//                if isFlagSet(c.cflags, CLIENT_PLUGIN_AUTH) {
//                     plugin_name := "temp_auth"
//                     WriteString(c.writeBuf, plugin_name, NULLSTR, &pos, 0)
//                }
//           default:
//                log.Fatal("Unsupported handshake version")
//
//      }
//      c.WritePacket(c.writeBuf[0:pos], pos)
// }
//
//
// /*---- PROTOCOL MAIN -----------------------------------------------------------
// * This function gets its own header because it's the head honcho of all the
// * other functions. This guy is the function that runs all other functions
// * in this file.
// */
//
// /* Initiates handshake exchange and performs authentication to verify
// * and secure connection with client. Each instance of a server should have one
// * of these guys. Returns true if successful; false otherwise.  */
// func (srv *server) HandleProtocol(conn net.Conn) bool {
//      // TODO: Fill in connection phase protocol handling.
//      // Also ask Kenneth where best to add this in the hera code.
//      return false
// }
