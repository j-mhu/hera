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

package common

// contains protocol constants shared by the client and the server


// Return codes
const (
	RcSQLError       = 1
	RcError          = 2
	RcValue          = 3
	RcHello          = 4
	RcOK             = 5
	RcNoMoreData     = 6
	RcStillExecuting = 7
)

// Commands
const (
	CmdPrepare          = 1
	CmdBindName         = 2
	CmdBindValue        = 3
	CmdExecute          = 4
	CmdRows             = 5
	CmdCols             = 6
	CmdFetch            = 7
	CmdCommit           = 8
	CmdRollback         = 9
	CmdBindType         = 10
	CmdClientInfo       = 11
	CmdBacktrace        = 12
	CmdBindOutName      = 13
	CmdPrepareSpecial   = 14
	CmdColsInfo         = 22
	CmdBindNum          = 23
	CmdBindValueMaxSize = 24
	CmdPrepareV2        = 25
	CmdShardKey         = 27
	CmdGetNumShards     = 28
	CmdSetShardID       = 29
)

// DataType defines Bind data types
type DataType int

// DataType constants
const (
	DataTypeString      = 0
	DataTypeRaw         = 3
	DataTypeBlob        = 4
	DataTypeClob        = 5
	DataTypeTimestamp   = 6
	DataTypeTimestampTZ = 7
)

// ServerCommands
const (
	CmdServerChallenge                     = 1001
	CmdServerConnectionAccepted            = 1002
	CmdServerConnectionRejectedProtocol    = 1003
	CmdServerConnectionRejectedUnknownUser = 1004
	CmdServerConnectionRejectedFailedAuth  = 1005
	CmdServerUnexpectedCommand             = 1006
	CmdServerInternalError                 = 1007
	CmdServerPingCommand                   = 1008
	CmdServerAlive                         = 1009
	CmdServerConnectionRejectedClientTime  = 1010
	CmdServerInfo                          = 1011
	CmdServerIntInfo                       = 1012

	CmdClientProtocolNameNoAuth = 2001
	CmdClientProtocolName       = 2002
	CmdClientUsername           = 2003
	CmdClientChallengeResponse  = 2004
	CmdClientCurrentClientTime  = 2005

	// For Cal correlation id during handshahing
	CmdClientCalCorrelationID = 2006

	CmdProtocolVersion = 2008
)

/* ==== MySQL  ===============================================================
* Below defines constants for all the MySQL protocol packets, basic types
* (fixed length integers, EOF strings, etc.), capability flag bitfields, and
* command bytes. Functions for encoding them into connection phase packets
* are located in utility/encoding/mysqlpackets.
 */

/* ---- COMMAND BYTES. ---------------------------------------------------------
* Command byte is the first byte in a command packet. Signifies
* what command the client wants the server to carry out. The command bytes
* are consistent with MySQL 4.1.
*    https://dev.mysql.com/doc/internals/en/command-phase.html
 */
const (
	COM_SLEEP int = iota 	// -------------------------------- 0
	COM_QUIT
	COM_INIT_DB
	COM_QUERY
	COM_FIELD_LIST
	COM_CREATE_DB 			// -------------------------------- 5
	COM_DROP_DB
	COM_REFRESH
	COM_SHUTDOWN
	COM_STATISTICS
	COM_PROCESS_INFO 		// -------------------------------- 10
	COM_CONNECT
	COM_PROCESS_KILL
	COM_DEBUG
	COM_PING
	COM_TIME 				// -------------------------------- 15
	COM_DELAYED_INSERT
	COM_CHANGE_USER
	COM_BINLOG_DUMP
	COM_TABLE_DUMP
	COM_CONNECT_OUT  		// -------------------------------- 20
	COM_REGISTER_SLAVE
	COM_STMT_PREPARE
	COM_STMT_EXECUTE
	COM_STMT_SEND_LONG_DATA
	COM_STMT_CLOSE 		// -------------------------------- 25
	COM_STMT_RESET
	COM_SET_OPTION
	COM_STMT_FETCH
	COM_RESET_CONNECTION
	COM_DAEMON 			// -------------------------------- 30
)

/* SQL commands in their string form that can be printed in error messages. */
var SQLcmds = map[int]string{
	COM_SLEEP: "COM_SLEEP", // 0
	COM_QUIT:  "COM_QUIT",
	COM_INIT_DB: "COM_INIT_DB",
	COM_QUERY: "COM_QUERY",
	COM_FIELD_LIST: "COM_FIELD_LIST",
	COM_CREATE_DB: "COM_CREATE_DB", // 5
	COM_DROP_DB: "COM_DROP_DB",
	COM_REFRESH: "COM_REFRESH",
	COM_SHUTDOWN: "COM_SHUTDOWN",
	COM_STATISTICS: "COM_STATISTICS",
	COM_PROCESS_INFO: "COM_PROCESS_INFO", // 10
	COM_CONNECT: "COM_CONNECT",
	COM_PROCESS_KILL: "COM_PROCESS_KILL",
	COM_DEBUG: "COM_DEBUG",
	COM_PING: "COM_PING",
	COM_TIME: "COM_TIME", // 15
	COM_DELAYED_INSERT:  "COM_DELAYED_INSERT",
	COM_CHANGE_USER: "COM_CHANGE_USER",
	COM_BINLOG_DUMP: "COM_BINLOG_DUMP",
	COM_TABLE_DUMP: "COM_TABLE_DUMP",
	COM_CONNECT_OUT: "COM_CONNECT_OUT", // 20
	COM_REGISTER_SLAVE: "COM_REGISTER_SLAVE",
	COM_STMT_PREPARE: "COM_STMT_PREPARE",
	COM_STMT_EXECUTE: "COM_STMT_EXECUTE",
	COM_STMT_SEND_LONG_DATA: "COM_STMT_SEND_LONG_DATA",
	COM_STMT_CLOSE: "COM_STMT_CLOSE", // 25
	COM_STMT_RESET: "COM_STMT_RESET",
	COM_SET_OPTION: "COM_SET_OPTION",
	COM_STMT_FETCH: "COM_STMT_FETCH",
	COM_RESET_CONNECTION: "COM_RESET_CONNECTION",
	COM_DAEMON: "COM_DAEMON" } // 30