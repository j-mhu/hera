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

package lib

import (
	"bufio"
	"context"
	"fmt"
	"github.com/paypal/hera/common"
	"github.com/paypal/hera/utility/encoding"
	"github.com/paypal/hera/utility/encoding/mysqlpackets"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/paypal/hera/cal"
	"github.com/paypal/hera/utility/encoding/netstring"
	"github.com/paypal/hera/utility/logger"
)

var connection_id = 0

// Spawns a goroutine which blocks waiting for a message on conn. When a message is received it writes
// to the channel and exit. It basically wrapps the net.Conn in a channel
func wrapNewNetstring(conn net.Conn, isMySQL bool) <-chan *encoding.Packet {
	ch := make(chan *encoding.Packet, 1)
	go func() {
		var ns *encoding.Packet
		var err error

		if isMySQL {
			ns, err = mysqlpackets.NewInitSQLPacket(conn)

		} else {
			ns, err = netstring.NewNetstring(conn)
		}
		if err != nil {
			if err == io.EOF {
				if logger.GetLogger().V(logger.Debug) {
					logger.GetLogger().Log(logger.Debug, conn.RemoteAddr(), ": Connection closed (eof) ")
				}
			} else {
				if logger.GetLogger().V(logger.Info) {
					logger.GetLogger().Log(logger.Info, conn.RemoteAddr(), ": Connection handler read error", err.Error(), ns.Serialized)
				}
			}
			ch <- nil
		} else {
			if ns.Serialized != nil && len(ns.Serialized) > 64*1024 {
				evt := cal.NewCalEvent("MUX", "large_payload_in", cal.TransOK, "")
				evt.AddDataInt("len", int64(len(ns.Serialized)))
				evt.Completed()
			}
			ch <- ns
		}
		close(ch)
	}()

	return ch
}

// sendHandshake sends a MySQLProtocol Handshakev10 to the client. Handshakev10 was chosen because
// go-sql-driver requires CLIENT_PROTOCOL_41 compatibility.
// https://dev.mysql.com/doc/dev/mysql-server/8.0.12/page_protocol_connection_phase_packets_protocol_handshake_v10.html
/*=== HANDSHAKE FUNCTIONS ====================================================*/

/* Sends handshake over connection. Only writes Handshakev10 packets. */
func sendHandshake(conn net.Conn) {
	scramble := "ham&eggs" // temporary authentication plugin data
	pos := 0

	// The max packet size is overkill.
	writeBuf := make([]byte, mysqlpackets.MAX_PACKET_SIZE)
	// protocol version
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT1, 0xa, &pos)

	// server version
	mysqlpackets.WriteString(writeBuf, "hera_server", mysqlpackets.NULLSTR, &pos, 0)

	cflags := uint32(mysqlpackets.CLIENT_PROTOCOL_41)

	// thread id
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT4, connection_id, &pos)
	connection_id++

	// Write first 8 bytes of plugin provided data (scramble)
	mysqlpackets.WriteString(writeBuf, scramble, mysqlpackets.FIXEDSTR, &pos, 8)

	// filler
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT1, 0x00, &pos)

	// capability_flags_1
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT2, int(cflags), &pos)

	// character_set
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT1, 0xff, &pos)

	// status_flags
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT2, 0x00, &pos)

	// capability_flags_2
	mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT2, int(cflags) >> 16, &pos)

	if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_PLUGIN_AUTH) {
		// authin_plugin_data_len. Temp: 0xaa
		mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT1, 0xaa, &pos)
	} else {
		// 00
		mysqlpackets.WriteFixedLenInt(writeBuf, mysqlpackets.INT1, 0x00, &pos)
	}
	// reserved
	mysqlpackets.WriteString(writeBuf, strings.Repeat("0", 10), mysqlpackets.FIXEDSTR, &pos, 10)

	// auth-plugin-data-part-2
	mysqlpackets.WriteString(writeBuf, scramble, mysqlpackets.LENENCSTR, &pos, 13)

	if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_PLUGIN_AUTH) {
		plugin_name := "temp_auth"
		mysqlpackets.WriteString(writeBuf, plugin_name, mysqlpackets.NULLSTR, &pos, 0)
	}
	handshake := mysqlpackets.NewMySQLPacketFrom(0, writeBuf[0:pos])
	_, err := conn.Write(handshake.Serialized[1:])
	logger.GetLogger().Log(logger.Info, ": Writing handshake to MySQL client >>>", handshake.Serialized[1:])
	if err != nil {
		logger.GetLogger().Log(logger.Verbose, ": Failed to write handshake to MySQL client >>>", DebugString(handshake.Serialized))
	}
}

/* READS THE HANDSHAKE RESPONSE SENT BY THE CLIENT. */
func readHandshakeResponse(conn net.Conn) {

	reader := bufio.NewReader(conn)

	// Read in the header and sequence id of the packet.
	a, err := reader.ReadByte()
	b, err := reader.ReadByte()
	d, err := reader.ReadByte()
	length := uint32(d) << 16 | uint32(b) << 8 | uint32(a)

	// Increase the sequence id by 1 because a packet was just received
	// from the client.
	sqid, err := reader.ReadByte()
	sqid++

	// Read in the payload.
	packet := make([]byte, length)
	n, err := io.ReadFull(reader, packet)

	// Check that the length of the payload is correct.
	if n != int(length) {
		logger.GetLogger().Log(logger.Verbose,fmt.Sprintf("Expected %d bytes, read %d", length, n))
	} else if err != nil {
		logger.GetLogger().Log(logger.Verbose, err.Error())
	}

	pos := 0  // index tracker
	cflags := uint32(mysqlpackets.CLIENT_PROTOCOL_41)
	if !mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_PROTOCOL_41) {

		// log : Reading HANDSHAKE_RESPONSE_320
		// lflags := ReadFixedLenInt(packet, INT2, &pos)
		// mpsize := ReadFixedLenInt(packet, INT3, &pos)
		mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT2, &pos)
		mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT3, &pos)

		// Username (null-terminated string)
		// user := ReadString(packet, NULLSTR, &pos, 0)
		mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)

		if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_CONNECT_WITH_DB) {
			// auth_response := ReadString(packet, NULLSTR, &pos, 0)
			mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)
			// dbname := ReadString(packet, NULLSTR, &pos, 0)
			mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)
		} else {
			// auth_response := ReadString(packet, EOFSTR, &pos, int(packetLen) - pos)
			mysqlpackets.ReadString(packet, mysqlpackets.EOFSTR, &pos, int(length) - pos)
		}
	} else {
		// log : Reading HANDSHAKE_RESPONSE_41

		// client flags
		flags := uint32(mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT4, &pos))
		cflags &= flags

		// maximum packet size, 0xFFFFFF max
		// mpsize := ReadFixedLenInt(packet, INT4, &pos)
		mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT4, &pos)

		// character set
		mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT1, &pos)

		// filler string
		mysqlpackets.ReadString(packet, mysqlpackets.FIXEDSTR, &pos, 23)

		// username
		// user := ReadString(packet, NULLSTR, &pos, 0)
		mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)

		if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA) {
			// auth_response := ReadString(packet, LENENCSTR, &pos, 0)
			mysqlpackets.ReadString(packet, mysqlpackets.LENENCSTR, &pos, 0)
		} else {
			// auth_response_length := ReadFixedLenInt(packet, INT1, &pos)
			n := mysqlpackets.ReadFixedLenInt(packet, mysqlpackets.INT1, &pos)

			mysqlpackets.ReadString(packet, mysqlpackets.FIXEDSTR, &pos, n)
		}

		if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_CONNECT_WITH_DB) {
			// dbname := ReadString(packet, NULLSTR, &pos, 0)
			mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)
		}

		if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_PLUGIN_AUTH) {
			// client_plugin_name := ReadString(packet, NULLSTR, &pos, 0)
			mysqlpackets.ReadString(packet, mysqlpackets.NULLSTR, &pos, 0)
		}

		if mysqlpackets.Supports(cflags, mysqlpackets.CLIENT_CONNECT_ATTRS) {
			// key_val_len := ReadLenEncInt(packet, &pos)
			mysqlpackets.ReadLenEncInt(packet, &pos)
		}
	}

	OK := mysqlpackets.NewMySQLPacketFrom(int(sqid), mysqlpackets.OKPacket(0, 0, uint32(0), "Welcome to Hera!"))

	// Write OK packet to signify handshake response has been processed.
	conn.Write(OK.Serialized[1:])
}




// HandleConnection runs as a go routine handling a client connection.
// It creates the coordinator go-routine and the one way channel to communicate
// with the coordinator. Then it sits in a loop for the life of the connection
// reading data from the connection. Once a complete netstring is read, the
// netstring object (which can contain nested sub-netstrings) is passed on
// to the coordinator for processing
func HandleConnection(conn net.Conn) {
	//
	// proxy just took a new connection. increment the idel connection count.
	//
	GetStateLog().PublishStateEvent(StateEvent{eType: ConnStateEvt, shardID: 0, wType: wtypeRW, instID: 0, oldCState: Close, newCState: Idle})

	clientchannel := make(chan *encoding.Packet, 1)
	// closing of clientchannel will notify the coordinator to exit
	defer func() {
		close(clientchannel)
		GetStateLog().PublishStateEvent(StateEvent{eType: ConnStateEvt, shardID: 0, wType: wtypeRW, instID: 0, oldCState: Idle, newCState: Close})
	}()

	//TODO: create a context with timeout
	ctx, cancel := context.WithCancel(context.Background())

	IsMySQL := true
	// For MySQL clients, the connection expects a handshake packet from the server. We'll send this outside
	// of the coordinator in order to keep coordinator code limited to the command phase.

	if IsMySQL {
		logger.GetLogger().Log(logger.Info, "Sending handshake")
		sendHandshake(conn)
		logger.GetLogger().Log(logger.Info, "Reading handshake response")
		readHandshakeResponse(conn)
		//ns, err := mysqlpackets.NewInitSQLPacket(conn)
		//if err != nil {
		//	logger.GetLogger().Log(logger.Info, "Error from reading SQLPacket,", err.Error())
		//}
		//if ns != nil {
		//	logger.GetLogger().Log(logger.Info, ns.Serialized[1:])
		//}
	}

	logger.GetLogger().Log(logger.Info, "Created coordinator in connection handler")

	crd := NewCoordinator(ctx, clientchannel, conn)
	go crd.Run()
	// crd.Run()

	//
	// clientchannel is a mechanism for request handler to pass over the client netstring
	// this loop blocks on the client connection.
	// - when receiving a netstring, it writes the netstring to the channel
	// - when receiving a connection error, it closes the clientchannel which is a
	//   detectable event in coordinator such that coordinator can clean up and exit too
	//
	addr := conn.RemoteAddr()
	for {
		var ns *encoding.Packet
		select {
		case ns = <-wrapNewNetstring(conn, true): /* Set this to false if you expect a client with netstring */
		case timeout := <-crd.Done():
			if logger.GetLogger().V(logger.Info) {
				logger.GetLogger().Log(logger.Info, "Connection handler idle timeout", addr)
			}
			evt := cal.NewCalEvent("MUX", "idle_timeout_"+strconv.Itoa(int(timeout)), cal.TransOK, "")
			evt.Completed()

			conn.Close() // this forces netstring.NewNetstring() conn.Read to exit with err=read tcp 127.0.0.1:8081->127.0.0.1:57968: use of closed network connection
			ns = nil
		}
		if ns == nil {
			break
		}
		if logger.GetLogger().V(logger.Verbose) {
			logger.GetLogger().Log(logger.Verbose, addr, ": Connection handler read <<<", DebugString(ns.Serialized))
		}

		//
		// coordinator is ready to go, send over the new netstring.
		// this could block when client close the connection abruptly. e.g. when coordinator write
		// is the first one to encounter the closed connection, coordinator exits. meanwhile there
		// could still be a last pending message from client that is blocked since there is not one
		// listening to clientchannel anymore. to avoid blocking, give clientchannel a buffer.
		//
		clientchannel <- ns
		if ns.IsMySQL && ns.Cmd == common.COM_QUIT {
			break
		}
	}
	if logger.GetLogger().V(logger.Info) {
		logger.GetLogger().Log(logger.Info, "======== Connection handler exits", addr)
	}
	conn.Close()
	conn = nil
	cancel()
}
