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

// Internal commands between worker and proxy
const (
	CmdControlMsg = 501
	CmdEOR        = 502 // end of response
)

// EOR codes
const (
	EORFree                     = 0
	EORInTransaction            = 1
	EORInCursorNotInTransaction = 2 /* not in transaction but not free because the cursor is open for ex */
	EORInCursorInTransaction    = 3 /* not in transaction but not free because the cursor is open for ex */
	EORMoreIncomingRequests     = 4 /* worker would be free, but it is not because there are more requests on the incomming buffer because
	they were pipelined by the client */
	EORBusyOther = 5 /* not used yet */
	EORRestart   = 6
)

// Reasons for stranded child
const (
	StrandedClientClose       = 4
	StrandedSaturationRecover = 5
	StrandedSwitch            = 6
	StrandedTimeout           = 7
	StrandedErr               = 8
)


/* ==== MySQL ================================================================*/

/* ---- CAPABILITY FLAGS -------------------------------------------------------
* Bitwise '|' the desired capabilities together when configuring the server,
* like follows, to give the server those capabilities, e.g.
*
*   capabilities := CLIENT_LONG_PASSWORD | CLIENT_SSL | CLIENT_PROTOCOL_41
*
* Since Hera maintains a connection to a MySQL database, it's likely that
* Hera only needs to be set to have the most basic capabilities so that the
* client is able to connect.
*
*   https://dev.mysql.com/doc/internals/en/capability-flags.html#packet-Protocol::CapabilityFlags
*/
const (
     CLIENT_LONG_PASSWORD                    int = 1 << (iota) // 1 << 0
     CLIENT_FOUND_ROWS
     CLIENT_LONG_FLAG
     CLIENT_CONNECT_WITH_DB
     CLIENT_NO_SCHEMA
     CLIENT_COMPRESS					// 1 << 5
     CLIENT_ODBC
     CLIENT_LOCAL_FILES
     CLIENT_IGNORE_SPACE
     CLIENT_PROTOCOL_41
     CLIENT_INTERACTIVE					// 1 << 10
     CLIENT_SSL
     CLIENT_IGNORE_SIGPIPE
     CLIENT_TRANSACTIONS
     CLIENT_RESERVED
     CLIENT_RESERVED2					// 1 << 15
     CLIENT_MULTI_STATEMENTS
     CLIENT_MULTI_RESULTS
     CLIENT_PS_MULTI_RESULTS
     CLIENT_PLUGIN_AUTH
     CLIENT_CONNECT_ATTRS				// 1 << 20
     CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA
     CLIENT_CAN_HANDLE_EXPIRED_PASSWORDS
     CLIENT_SESSION_TRACK
     CLIENT_DEPRECATE_EOF
     CLIENT_SSL_VERIFY_SERVER_CERT 	    int = 1 << 30
     CLIENT_OPTIONAL_RESULTSET_METADATA     int = 1 << 25
     CLIENT_REMEMBER_OPTIONS	              int = 1 << 31
)

/* ---- STATUS FLAGS -----------------------------------------------------------
* Status flags are a bit-field that gives information about the server state.
*
* These are set as the coordinator runs.
*   https://dev.mysql.com/doc/internals/en/status-flags.html
*/

const (
	SERVER_STATUS_IN_TRANS		= 	0x0001	// a transaction is active
	SERVER_STATUS_AUTOCOMMIT		= 	0x0002	// auto-commit is enabled
	SERVER_MORE_RESULTS_EXISTS 	= 	0x0008
	SERVER_STATUS_NO_GOOD_INDEX_USED = 0x0010
	SERVER_STATUS_NO_INDEX_USED 	=	0x0020
	SERVER_STATUS_CURSOR_EXISTS 	=	0x0040 // Used by Binary Protocol Resultset to signal that COM_STMT_FETCH must be used to fetch the row-data.
	SERVER_STATUS_LAST_ROW_SENT 	=	0x0080
	SERVER_STATUS_DB_DROPPED		= 	0x0100
	SERVER_STATUS_NO_BACKSLASH_ESCAPES = 0x0200
	SERVER_STATUS_METADATA_CHANGED =	0x0400
	SERVER_QUERY_WAS_SLOW 		=	0x0800
	SERVER_PS_OUT_PARAMS 		=	0x1000
	SERVER_STATUS_IN_TRANS_READONLY =	0x2000   //	in a read-only transaction
	SERVER_SESSION_STATE_CHANGED	= 	0x4000	 //connection state information has changed
)
