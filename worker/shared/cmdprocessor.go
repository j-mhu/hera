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

package shared

// TODO: MySQL packet processing in worker for all commands.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/paypal/hera/utility/encoding"
	"github.com/paypal/hera/utility/encoding/mysqlpackets"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/paypal/hera/cal"
	"github.com/paypal/hera/common"
	"github.com/paypal/hera/utility"
	"github.com/paypal/hera/utility/encoding/netstring"
	"github.com/paypal/hera/utility/logger"

	"database/sql"
)

// CmdProcessorAdapter is interface for differentiating the specific database implementations.
// For example there is an adapter for MySQL, another for Oracle
type CmdProcessorAdapter interface {
	GetColTypeMap() map[string]int
	Heartbeat(*sql.DB) bool
	InitDB() (*sql.DB, error)
	/* ProcessError's workerScope["child_shutdown_flag"] = "1 or anything" can help terminate after the request */
	ProcessError(errToProcess error, workerScope *WorkerScopeType, queryScope *QueryScopeType)
	// ProcessResult is used for date related types to translate between the database format to the mux format
	ProcessResult(colType string, res string) string
	UseBindNames() bool
}

// bindType defines types of bind variables
type bindType int

// constants for BindType
const (
	btUnknown bindType = iota
	btIn
	btOut
)

// BindValue is a placeholder for a bind value, with index tracking its position in the query.
type BindValue struct {
	index int
	name  string
	value interface{}
	//
	// whether client has passed in a value.
	//
	valid bool
	//
	// input or output.
	//
	btype bindType
	// the data type
	dataType common.DataType
}

// CmdProcessor holds the data needed to process the client commmands
type CmdProcessor struct {
	ctx context.Context
	// adapter for various databases
	adapter CmdProcessorAdapter
	//
	// socket to mux
	//
	SocketOut *os.File
	//
	// db instance.
	//
	db *sql.DB
	//
	// open txn if having dml.
	//
	tx *sql.Tx
	//
	// prepared statement yet to be executed.
	//
	stmt *sql.Stmt
	//
	// tells if the current SQL is a query which returns result set (i.e. SELECT)
	//
	hasResult bool
	// tells if the current connection is in transaction. it becomes true if a DML ran successfull
	inTrans bool
	// tells if the current connection has an open cursor
	inCursor bool
	//
	// all bindvar for the query after parsing.
	// using map with name key instead of array with position index for faster matching
	// when processing CmdBindName/Value since some queres can set hundreds of bindvar.
	//
	bindVars map[string]*BindValue
	// placeholders for bindouts
	bindOuts    []string
	numBindOuts int
	//
	// matching bindname to location in query for faster lookup at CmdExec.
	//
	bindPos []string
	//
	// indexed by stmt_id, stores all prepared statements sent by MySQL client
	//

	stmts map[int]*sql.Stmt 				// each stmt is given a stmtid to identify it by. this map contains the mappings
	currsid int // current available stmt.id

	stmtParams map[*sql.Stmt]int			// each stmt has a numParams required to execute or query the db. this map records the number for each stmt

	numColumns int				// number of columns specified in query
	packager *mysqlpackets.Packager // in charge of writing packets
	//
	// hera protocol let client sends bindname in one ns command and bindvalue for the
	// bindname in the very next ns command. this parameter is used to track which
	// name is for the current value.
	//
	currentBindName string
	//
	// result set for read query.
	//
	rows *sql.Rows
	//
	// result for dml query.
	//
	result sql.Result
	//
	//
	//
	sqlParser     common.SQLParser
	regexBindName *regexp.Regexp
	//
	// cal txn for the current session.
	//
	calSessionTxn cal.Transaction
	// cal txn for a SQL
	calExecTxn cal.Transaction
	// last error
	lastErr error
	// the FNV hash of the SQL, for logging
	sqlHash uint32
	// the name of the cal TXN
	calSessionTxnName string
	heartbeat         bool
	// counter for requests, acting like ID
	rqId uint16
	// used in eor() to send the right code
	moreIncomingRequests func() bool
	queryScope           QueryScopeType
	WorkerScope          WorkerScopeType
}

type QueryScopeType struct {
	NsCmd   string
	SqlHash string
}
type WorkerScopeType struct {
	Child_shutdown_flag bool
}

// NewCmdProcessor creates the processor using th egiven adapter
func NewCmdProcessor(adapter CmdProcessorAdapter, sockMux *os.File) *CmdProcessor {
	cs := os.Getenv("CAL_CLIENT_SESSION")
	if cs == "" {
		cs = "CLIENT_SESSION"
	}
	stmts := make(map[int]*sql.Stmt)

	return &CmdProcessor{adapter: adapter, SocketOut: sockMux, calSessionTxnName: cs, stmts:stmts, heartbeat: true}
}

// TODO: Needs MySQL integration
// ProcessCmd implements the client commands like prepare, bind, execute, etc
func (cp *CmdProcessor) ProcessCmd(ns *encoding.Packet) error {
	if ns == nil {
		return errors.New("empty netstring passed to processcommand")
	}
	if logger.GetLogger().V(logger.Debug) {
		logger.GetLogger().Log(logger.Debug, "process command", DebugString(ns.Serialized))
	}
	var err error

	cp.queryScope.NsCmd = fmt.Sprintf("%d", ns.Cmd)
	if ns.IsMySQL {
			logger.GetLogger().Log(logger.Info, "IsMySQL=", ns.IsMySQL, ", received packet with command:", common.SQLcmds[ns.Cmd])
			// otherloop:
			switch ns.Cmd {
			case common.COM_QUERY:
				logger.GetLogger().Log(logger.Info, "common.COM_QUERY")
				/* Right now this is only good for simple queries that don't ask for anything aside from an OK packet.
				* Reason being that COM_QUERY_RESPONSE packets fall into several categories. OK, ERR, or a series of packets
				* that include ColumnDefinition packets. We currently have no good way of honest replication of
				* ColumnDefinition.
				 */
				if logger.GetLogger().V(logger.Debug) {
					logger.GetLogger().Log(logger.Debug, "Executing ", cp.inTrans)
				}

				// Get the query from the payload
				sqlQuery := cp.preprocess(ns)

				// If the sqlQuery contains a select, use Query -- otherwise use Exec
				if cp.hasResult {
					cp.rows, err = cp.db.Query(sqlQuery)
				} else {
					cp.result, err = cp.db.Exec(sqlQuery)
					logger.GetLogger().Log(logger.Debug, "cp.result", cp.result != nil)
				}

				if err != nil {
					cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
					cp.calExecErr("RC", err.Error())
					if logger.GetLogger().V(logger.Warning) {
						logger.GetLogger().Log(logger.Warning, "Execute error:", err.Error())
					}
					if cp.inTrans {
						cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
					} else {
						cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
					}
					cp.lastErr = err
					err = nil
					break
				}

				if cp.tx != nil {
					cp.inTrans = true
				}

				if cp.result != nil {
					logger.GetLogger().Log(logger.Debug, "cp.result != nil case")
					var rowcnt int64
					rowcnt, err = cp.result.RowsAffected()
					logger.GetLogger().Log(logger.Debug, "Got RowsAffected")
					if err != nil {
						if logger.GetLogger().V(logger.Debug) {
							logger.GetLogger().Log(logger.Debug, "RowsAffected():", err.Error())
						}
						cp.calExecErr("RowsAffected", err.Error())
						break
					}
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "exe row", rowcnt)
					}

					// Get the last insert id from the sql.Result
					var liid int64
					liid, err = cp.result.LastInsertId()
					logger.GetLogger().Log(logger.Debug, "Got LastInsertID")
					if err != nil {
						if logger.GetLogger().V(logger.Debug) {
							logger.GetLogger().Log(logger.Debug, "LastInsertId():", err.Error())
						}
						cp.calExecErr("LastInsertId", err.Error())
						break
					}
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "exe LastInsertId", rowcnt)
					}
					logger.GetLogger().Log(logger.Debug, "Making new SQL packet, prev sqid", ns.Sqid)
					// Set an OK packet reporting the number of rows affected and last insert id. I don't know what to put for the message though...
					np := mysqlpackets.NewMySQLPacketFrom(ns.Sqid + 1, mysqlpackets.OKPacket(int(rowcnt), int(liid), uint32(mysqlpackets.CLIENT_PROTOCOL_41),"This packet has to be over 7 bytes."))
					logger.GetLogger().Log(logger.Debug, "Wrote with serialized, sqid", np.Serialized, np.Sqid)
					// Send OK packet.
					err = cp.eor(common.EORFree, np)


				}
			case common.COM_STMT_PREPARE:
				// TODO: The server always sends back a COM_STMT_PREPARE_RESPONSE to a prepared stmt command.
				// This requires Protocol::ColumnDefinition packets, which as of right now have not been implemented.
				//

				// WORK IN PROGRESS.
				cp.queryScope = QueryScopeType{}
				cp.lastErr = nil
				cp.sqlHash = 0
				cp.heartbeat = false // for hb

				sqlQuery := cp.preprocess(ns)

				if logger.GetLogger().V(logger.Verbose) {
					logger.GetLogger().Log(logger.Verbose, "Preparing:", sqlQuery)
				}

				//
				// start a new transaction for the first dml request.
				//
				var startTrans bool
				cp.hasResult, startTrans = cp.sqlParser.Parse(sqlQuery)
				if cp.calSessionTxn == nil {
					cp.calSessionTxn = cal.NewCalTransaction(cal.TransTypeAPI, cp.calSessionTxnName, cal.TransOK, "", cal.DefaultTGName)
				}
				cp.sqlHash = utility.GetSQLHash(string(ns.Payload))
				cp.queryScope.SqlHash = fmt.Sprintf("%d", cp.sqlHash)
				cp.calExecTxn = cal.NewCalTransaction(cal.TransTypeExec, fmt.Sprintf("%d", cp.sqlHash), cal.TransOK, "", cal.DefaultTGName)
				if (cp.tx == nil) && (startTrans) {
					cp.tx, err = cp.db.Begin()
				}

				if cp.tx != nil {
					cp.stmt, err = cp.tx.Prepare(sqlQuery)
				} else {
					cp.stmt, err = cp.db.Prepare(sqlQuery)
				}
				cp.stmts[cp.currsid] = cp.stmt
				cp.stmtParams[cp.stmt] = len(cp.bindVars)


				if err != nil {
					cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
					cp.calExecErr("Prepare", err.Error())
					cp.lastErr = err
					err = nil
				}

				// This is the part we need to send out a COM_STMT_PREPARE_OK packet. Which requires
				// column definition packets, which... means we need to figure out how to reconstruct
				// this guy.

				// Write the COM_STMT_PREPARE_OK prologue packets.
				prepareOK := mysqlpackets.NewMySQLPacketFrom(ns.Sqid + 1, mysqlpackets.StmtPrepareOK(cp.currsid, cp.numColumns, len(cp.bindVars)))
				// write prepareOK to conn
				cp.eor(common.EORFree, prepareOK)

				// Write column definitions to conn for each parameter and each column.
				// BIG PROBLEM: ColumnTypes can only be obtained from the go-sql-driver AFTER executing the query.
				for i := 0; i < len(cp.bindVars); i++ {
					// TODO: Send column definition for each parameter.
					// mysqlpackets.ColumnDefinition(...) in utility/encoding/mysqlpackets
					// cp.eor(...)
				}

				if len(cp.bindVars) > 0 {
					// TODO: Send EOF packet.
					// mysqlpackets.EOFPacket(warnings, status_flags int, capabilities uint32)
				}

				for i := 0; i < cp.numColumns; i++ {
					// TODO: Send column definition for each column.
					// mysqlpackets.ColumnDefinition(...) in utility/encoding/mysqlpackets
					// cp.eor(...)
				}

				if cp.numColumns > 0 {
					// TODO: Send EOF packet.
					// mysqlpackets.EOFPacket(warnings, status_flags int, capabilities uint32)
				}

				cp.rows = nil
				cp.result = nil
				cp.bindOuts = cp.bindOuts[:0]
				cp.numBindOuts = 0
				cp.currsid++

			case common.COM_STMT_EXECUTE:
				// First read in the stmt-id and obtain it from the map of stmt-id to stmts.
				pos := 1 // start at 1 to skip the command byte
				stmtid := mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT4, &pos)
				cp.stmt = cp.stmts[stmtid]

				// get numParams from stmtParams
				numParams := cp.stmtParams[cp.stmt]
				nullBitmap := []byte{}
				paramTypes := []byte{}
				values := []byte{}
				var newParams bool
				if numParams > 0 {
					// get null_bitmap from com stmt execute packet
					nullBitmap = mysqlpackets.ReadString(ns.Payload, mysqlpackets.VARSTR, &pos, (numParams + 7) / 8)
					// also get the new_params_bind_flag which is 1 fixed len integer
					if mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT1, &pos) == 1 {
						newParams = true
					}
				}
				if newParams {
					// get parameter types
					paramTypes = mysqlpackets.ReadString(ns.Payload, mysqlpackets.VARSTR, &pos, numParams * 2)
					// also get value of each parameter
					values = mysqlpackets.ReadString(ns.Payload, mysqlpackets.EOFSTR, &pos, 0)
				}

				// Then use either Query or Exec to obtain results and/or rows.
				if cp.stmt != nil {

					if !newParams {
						//
						// @TODO: do we keep a flag for curent statement.
						//
						if cp.hasResult {
							cp.rows, err = cp.stmt.Query()
						} else {
							cp.result, err = cp.stmt.Exec()
						}
					} else {
						// Get the new bound parameters and send them in as arguments.
						if cp.hasResult {
							cp.rows, err = cp.stmt.Query(values)
						} else {
							cp.result, err = cp.stmt.Exec(values)
						}
					}
					if err != nil {
						cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
						cp.calExecErr("RC", err.Error())
						if logger.GetLogger().V(logger.Warning) {
							logger.GetLogger().Log(logger.Warning, "Execute error:", err.Error())
						}
						if cp.inTrans {
							cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
						} else {
							cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
						}
						cp.lastErr = err
						err = nil
						break
					}
					if cp.tx != nil {
						cp.inTrans = true
					}

					cp.calExecTxn.Completed()
					cp.calExecTxn = nil

				}

				// Then use rows.Scan to obtain the column values for a returned result row.


				// Package into COM_STMT_EXECUTE response with resultsets.

				// Send to conn

			case common.COM_STMT_FETCH:
				// Fetches from an existing resultset.... dude
				pos := 1 // Start past the command byte
				stmtid := mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT4, &pos)
				numRows := mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT4, &pos)

				// Fetch from existing resultset keyed in to an already executed statement

			case common.COM_CREATE_DB, common.COM_DROP_DB, common.COM_INIT_DB:
				pos := 1
				schema_name := mysqlpackets.ReadString(ns.Payload, mysqlpackets.EOFSTR, &pos, 0)
				// Send this directly to the db as a query.
				var query string
				if ns.Cmd == common.COM_CREATE_DB {
					query = fmt.Sprintf("CREATE DATABASE %s;", schema_name)
				} else if ns.Cmd == common.COM_DROP_DB {
					query = fmt.Sprintf("DROP DATABASE IF EXISTS %s;", schema_name)
				} else {
					query = fmt.Sprintf("USE %s;", schema_name)
				}
				cp.result, err = cp.db.Exec(query)
				if err != nil {
					logger.GetLogger().Log(logger.Debug, common.SQLcmds[ns.Cmd], "failure to act on DB: ", err.Error())
					// Construct ERRPACKET.
					np := mysqlpackets.NewMySQLPacketFrom(ns.Sqid + 1, mysqlpackets.ERRPacket(0/* */, "0"/* */ ))
					logger.GetLogger().Log(logger.Debug, "Wrote with serialized, sqid", np.Serialized, np.Sqid)
					// Send ERR packet.
					err = cp.eor(common.EORFree, np)
				}
				if cp.result != nil {
					logger.GetLogger().Log(logger.Debug, "cp.result != nil case")
					var rowcnt int64
					rowcnt, err = cp.result.RowsAffected()
					logger.GetLogger().Log(logger.Debug, "Got RowsAffected")
					if err != nil {
						if logger.GetLogger().V(logger.Debug) {
							logger.GetLogger().Log(logger.Debug, "RowsAffected():", err.Error())
						}
						cp.calExecErr("RowsAffected", err.Error())
						break
					}
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "exe row", rowcnt)
					}

					// Get the last insert id from the sql.Result
					var liid int64
					liid, err = cp.result.LastInsertId()
					logger.GetLogger().Log(logger.Debug, "Got LastInsertID")
					if err != nil {
						if logger.GetLogger().V(logger.Debug) {
							logger.GetLogger().Log(logger.Debug, "LastInsertId():", err.Error())
						}
						cp.calExecErr("LastInsertId", err.Error())
						break
					}
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "exe LastInsertId", rowcnt)
					}
					logger.GetLogger().Log(logger.Debug, "Making new SQL packet, prev sqid", ns.Sqid)
					// Set an OK packet reporting the number of rows affected and last insert id. I don't know what to put for the message though...
					np := mysqlpackets.NewMySQLPacketFrom(ns.Sqid + 1, mysqlpackets.OKPacket(int(rowcnt), int(liid), uint32(mysqlpackets.CLIENT_PROTOCOL_41),"This packet has to be over 7 bytes."))
					logger.GetLogger().Log(logger.Debug, "Wrote with serialized, sqid", np.Serialized, np.Sqid)
					// Send OK packet.
					err = cp.eor(common.EORFree, np)
				}

			case common.COM_STMT_CLOSE:
				// Read in the stmtid from the pakcet
				pos := 1
				stmtid := mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT4, &pos)
				// Close the statement
				err := cp.stmts[stmtid].Close()
				if err != nil {
					// Other cal logging and eor stuff
					logger.GetLogger().Log(logger.Warning, "Tried to close statement but got", err.Error())
				}
				// Also remove the current stmtid - sttmt mapping from the stmts map
				delete(cp.stmts, stmtid)

				// No response is sent back to the client.

			case common.COM_STMT_SEND_LONG_DATA:
				// pos := 1
				// stmtid := mysqlpackets.ReadFixedLenInt(ns.Payload, mysqlpackets.INT4, &pos)
			}
	} else {
outloop:
	switch ns.Cmd {
	case common.CmdClientCalCorrelationID:
		logger.GetLogger().Log(logger.Verbose, "Got to CmdClientCalCorrelationID")
		//
		// @TODO parse out correlationid.
		//
		if cp.calSessionTxn != nil {
			cp.calSessionTxn.SetCorrelationID("@todo")
		}
	case common.CmdPrepare, common.CmdPrepareV2, common.CmdPrepareSpecial:
		cp.queryScope = QueryScopeType{}
		cp.lastErr = nil
		cp.sqlHash = 0
		cp.heartbeat = false // for hb
		//
		// need to turn "select * from table where ca=:a and cb=:b"
		// to "select * from table where ca=? and cb=?"
		// while keeping an ordered list of (":a"=>"val_:a", ":b"=>"val_:b") to run
		// stmt.Exec("val_:a", "val_:b"). val_:a and val_:b are extracted using
		// BindName and BindValue
		//
		sqlQuery := cp.preprocess(ns)
		if logger.GetLogger().V(logger.Verbose) {
			logger.GetLogger().Log(logger.Verbose, "Preparing:", sqlQuery)
		}
		//
		// start a new transaction for the first dml request.
		//
		var startTrans bool
		cp.hasResult, startTrans = cp.sqlParser.Parse(sqlQuery)
		if cp.calSessionTxn == nil {
			cp.calSessionTxn = cal.NewCalTransaction(cal.TransTypeAPI, cp.calSessionTxnName, cal.TransOK, "", cal.DefaultTGName)
		}
		cp.sqlHash = utility.GetSQLHash(string(ns.Payload))
		cp.queryScope.SqlHash = fmt.Sprintf("%d", cp.sqlHash)
		cp.calExecTxn = cal.NewCalTransaction(cal.TransTypeExec, fmt.Sprintf("%d", cp.sqlHash), cal.TransOK, "", cal.DefaultTGName)
		if (cp.tx == nil) && (startTrans) {
			cp.tx, err = cp.db.Begin()
		}
		if cp.tx != nil {
			cp.stmt, err = cp.tx.Prepare(sqlQuery)
		} else {
			cp.stmt, err = cp.db.Prepare(sqlQuery)
		}
		if err != nil {
			cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
			cp.calExecErr("Prepare", err.Error())
			cp.lastErr = err
			err = nil
		}
		cp.rows = nil
		cp.result = nil
		cp.bindOuts = cp.bindOuts[:0]
		cp.numBindOuts = 0
	case common.CmdBindName, common.CmdBindOutName:
		if cp.stmt != nil {
			cp.currentBindName = string(ns.Payload)
			if strings.HasPrefix(string(ns.Payload), ":") {
				cp.currentBindName = string(ns.Payload)
			} else {
				var buffer bytes.Buffer
				buffer.WriteString(":")
				buffer.Write(ns.Payload)
				cp.currentBindName = buffer.String()
			}
			if cp.bindVars[cp.currentBindName] == nil {
				//
				// @TODO a bindname not in the query.
				//
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "nonexisting bindname", cp.currentBindName)
				}
				err = fmt.Errorf("bindname not found in query: %s", cp.currentBindName)
				cp.calExecErr("Bind error", cp.currentBindName)
				break
			}
			if ns.Cmd == common.CmdBindName {
				cp.bindVars[cp.currentBindName].btype = btIn
			} else {
				cp.bindVars[cp.currentBindName].btype = btOut
				cp.bindVars[cp.currentBindName].valid = true
				cp.numBindOuts++
			}
			cp.bindVars[cp.currentBindName].dataType = common.DataTypeString
		}
	case common.CmdBindType:
		if cp.stmt != nil {
			var btype int
			btype, err = strconv.Atoi(string(ns.Payload))
			if err != nil {
				cp.calExecErr("BindTypeConv", err.Error())
				break
			}
			cp.bindVars[cp.currentBindName].dataType = common.DataType(btype)
		}
	case common.CmdBindValue:
		if cp.stmt != nil {
			//
			// double check to make sure.
			//
			if cp.bindVars[cp.currentBindName] == nil {
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "nonexisting bindname", cp.currentBindName)
				}
				err = fmt.Errorf("bindname not found in query: %s", cp.currentBindName)
				cp.calExecErr("BindValNF", cp.currentBindName)
				break
			} else {
				if len(ns.Payload) == 0 {
					cp.bindVars[cp.currentBindName].value = sql.NullString{}
					if logger.GetLogger().V(logger.Verbose) {
						logger.GetLogger().Log(logger.Verbose, "BindValue:", cp.currentBindName, ":", cp.bindVars[cp.currentBindName].dataType, ":<nil>")
					}
				} else {
					switch cp.bindVars[cp.currentBindName].dataType {
					case common.DataTypeTimestamp:
						var day, month, year, hour, min, sec, ms int
						fmt.Sscanf(string(ns.Payload), "%d-%d-%d %d:%d:%d.%d", &day, &month, &year, &hour, &min, &sec, &ms)
						cp.bindVars[cp.currentBindName].value = time.Date(year, time.Month(month), day, hour, min, sec, ms*1000000, time.UTC)
					case common.DataTypeTimestampTZ:
						var day, month, year, hour, min, sec, ms, tzh, tzm int
						fmt.Sscanf(string(ns.Payload), "%d-%d-%d %d:%d:%d.%d %d:%d", &day, &month, &year, &hour, &min, &sec, &ms, &tzh, &tzm)
						// Note: the Go Oracle driver ignores th elocation, always uses time.Local
						cp.bindVars[cp.currentBindName].value = time.Date(year, time.Month(month), day, hour, min, sec, ms*1000000, time.FixedZone("Custom", tzh*3600))
					case common.DataTypeRaw, common.DataTypeBlob:
						cp.bindVars[cp.currentBindName].value = ns.Payload
					default:
						cp.bindVars[cp.currentBindName].value = sql.NullString{String: string(ns.Payload), Valid: true}
					}
					if logger.GetLogger().V(logger.Verbose) {
						logger.GetLogger().Log(logger.Verbose, "BindValue:", cp.currentBindName, ":", cp.bindVars[cp.currentBindName].dataType, ":", cp.bindVars[cp.currentBindName].value)
					}
				}
				cp.bindVars[cp.currentBindName].valid = true
			}
		}
	case common.CmdBindNum:
		if cp.stmt != nil {
			err = fmt.Errorf("Batch not supported")
			cp.calExecErr("Batch", err.Error())
			break
		}
	case common.CmdExecute:
		if cp.stmt != nil {
			//
			// step through bindvar at each location to build bindinput.
			//
			bindinput := make([]interface{}, 0)
			if cap(cp.bindOuts) >= cp.numBindOuts {
				cp.bindOuts = cp.bindOuts[:cp.numBindOuts]
				// clear old values just in case
				for i := range cp.bindOuts {
					cp.bindOuts[i] = ""
				}
			} else {
				cp.bindOuts = make([]string, cp.numBindOuts)
			}
			curbindout := 0
			for i := 0; i < len(cp.bindPos); i++ {
				key := cp.bindPos[i]
				val := cp.bindVars[key]
				if val.btype == btIn {
					if !val.valid {
						err = fmt.Errorf("bindname undefined: %s", key)
						break outloop
					}
					if cp.adapter.UseBindNames() {
						bindinput = append(bindinput, sql.Named(key[1:], val.value))
					} else {
						bindinput = append(bindinput, val.value)
					}
				} else if val.btype == btOut {
					if cp.adapter.UseBindNames() {
						value := sql.Named(key[1:], sql.Out{Dest: &(cp.bindOuts[curbindout])})
						bindinput = append(bindinput, value)
						if logger.GetLogger().V(logger.Debug) {
							logger.GetLogger().Log(logger.Debug, "bindout", val.index, value, curbindout)
						}
						curbindout++
					} else {
						err = errors.New("outbind not supported")
						break outloop
					}
				}
			}
			if logger.GetLogger().V(logger.Debug) {
				logger.GetLogger().Log(logger.Debug, "Executing ", cp.inTrans)
				logger.GetLogger().Log(logger.Debug, "BINDS", bindinput)
			}
			if len(bindinput) == 0 {
				//
				// @TODO: do we keep a flag for curent statement.
				//
				if cp.hasResult {
					cp.rows, err = cp.stmt.Query()
				} else {
					cp.result, err = cp.stmt.Exec()
				}
			} else {
				if cp.hasResult {
					cp.rows, err = cp.stmt.Query(bindinput...)
				} else {
					cp.result, err = cp.stmt.Exec(bindinput...)
				}
			}
			if err != nil {
				cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
				cp.calExecErr("RC", err.Error())
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "Execute error:", err.Error())
				}
				if cp.inTrans {
					cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
				} else {
					cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
				}
				cp.lastErr = err
				err = nil
				break
			}
			if cp.tx != nil {
				cp.inTrans = true
			}
			cp.calExecTxn.Completed()
			cp.calExecTxn = nil
			if cp.result != nil {
				var rowcnt int64
				rowcnt, err = cp.result.RowsAffected()
				if err != nil {
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "RowsAffected():", err.Error())
					}
					cp.calExecErr("RowsAffected", err.Error())
					break
				}
				if logger.GetLogger().V(logger.Debug) {
					logger.GetLogger().Log(logger.Debug, "exe row", rowcnt)
				}
				sz := 2
				if len(cp.bindOuts) > 0 {
					sz++
					sz += len(cp.bindOuts)
				}
				if logger.GetLogger().V(logger.Verbose) {
					logger.GetLogger().Log(logger.Verbose, "BINDOUTS", len(cp.bindOuts), cp.bindOuts)
				}

				nss := make([]*encoding.Packet, sz)
				nss[0] = netstring.NewNetstringFrom(common.RcValue, []byte("0"))
				nss[1] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.FormatInt(rowcnt, 10)))
				if sz > 2 {
					if len(cp.bindOuts) > 0 {
						nss[2] = netstring.NewNetstringFrom(common.RcValue, []byte("1"))
						for i := 0; i < len(cp.bindOuts); i++ {
							nss[i+3] = netstring.NewNetstringFrom(common.RcValue, []byte(cp.bindOuts[i]))
						}
					}
				}
				resns := netstring.NewNetstringEmbedded(nss)
				err = cp.eor(common.EORInTransaction, resns)
			}
			if cp.rows != nil {
				var cols []string
				cols, err = cp.rows.Columns()
				if err != nil {
					if logger.GetLogger().V(logger.Warning) {
						logger.GetLogger().Log(logger.Warning, "rows.Columns()", err.Error())
					}
					cp.calExecErr("Columns", err.Error())
					break
				}
				if logger.GetLogger().V(logger.Debug) {
					logger.GetLogger().Log(logger.Debug, "exe col", cols, len(cols))
				}
				// TODO: what is there are rows?
				sz := 2
				if len(cp.bindOuts) > 0 {
					sz++
				}

				nss := make([]*encoding.Packet, sz)
				nss[0] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.Itoa(len(cols))))
				nss[1] = netstring.NewNetstringFrom(common.RcValue, []byte("0"))
				if sz > 2 {
					nss[2] = netstring.NewNetstringFrom(common.RcValue, []byte("0"))
				}
				resns := netstring.NewNetstringEmbedded(nss)
				if cp.hasResult {
					/*
						TODO: this is the proper implementation, need to fix mux, meanwhile just done use EOR_IN_CURSOR_...
						if cp.inTrans {
							cp.eor(EOR_IN_CURSOR_IN_TRANSACTION, resns)
						} else {
							cp.eor(EOR_IN_CURSOR_NOT_IN_TRANSACTION, resns)
						}
					*/
					WriteAll(cp.SocketOut, resns)
				} else {
					if cp.inTrans {
						cp.eor(common.EORInTransaction, resns)
					} else {
						cp.eor(common.EORFree, resns)
					}
				}
			}
		} else {
			if cp.inTrans {
				cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(cp.lastErr.Error())))
			} else {
				cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcSQLError, []byte(cp.lastErr.Error())))
			}
		}
	case common.CmdFetch:
		// TODO fecth chunk size
		if cp.rows != nil {
			calt := cal.NewCalTransaction(cal.TransTypeFetch, fmt.Sprintf("%d", cp.sqlHash), cal.TransOK, "", cal.DefaultTGName)
			var cts []*sql.ColumnType
			cts, err = cp.rows.ColumnTypes()
			if err != nil {
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "rows.Columns()", err.Error())
				}
				calt.AddDataStr("RC", err.Error())
				calt.SetStatus(cal.TransError)
				calt.Completed()
				break
			}
			var nss []*encoding.Packet
			cols, _ := cp.rows.Columns()
			readCols := make([]interface{}, len(cols))
			writeCols := make([]sql.NullString, len(cols))
			for i := range writeCols {
				readCols[i] = &writeCols[i]
			}
			for cp.rows.Next() {
				err = cp.rows.Scan(readCols...)
				if err != nil {
					cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
					if logger.GetLogger().V(logger.Warning) {
						logger.GetLogger().Log(logger.Warning, "fetch:", err.Error())
					}
					calt.AddDataStr("RC", err.Error())
					calt.SetStatus(cal.TransError)
					calt.Completed()
					break
				}
				for i := range writeCols {
					var outstr string
					if writeCols[i].Valid {
						outstr = cp.adapter.ProcessResult(cts[i].DatabaseTypeName(), writeCols[i].String)
					}
					if logger.GetLogger().V(logger.Debug) {
						logger.GetLogger().Log(logger.Debug, "query result", outstr)
					}
					nss = append(nss, netstring.NewNetstringFrom(common.RcValue, []byte(outstr)))
				}
			}
			if len(nss) > 0 {
				resns := netstring.NewNetstringEmbedded(nss)
				err = WriteAll(cp.SocketOut, resns)
				if err != nil {
					if logger.GetLogger().V(logger.Warning) {
						logger.GetLogger().Log(logger.Warning, "Error writing to mux", err.Error())
					}
					calt.AddDataStr("RC", "Comm error")
					calt.SetStatus(cal.TransError)
					calt.Completed()
					break
				}
			}
			calt.Completed()
			if cp.inTrans {
				cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcNoMoreData, nil))
			} else {
				cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcNoMoreData, nil))
			}
			cp.rows = nil
		} else {
			// send back to client only if last result was ok
			var nsr *encoding.Packet
			if cp.lastErr == nil {
				nsr = netstring.NewNetstringFrom(common.RcError, []byte("fetch requested but no statement exists"))
			}
			if cp.inTrans {
				cp.eor(common.EORInTransaction, nsr)
			} else {
				cp.eor(common.EORFree, nsr)
			}
		}
	case common.CmdColsInfo:
		if cp.rows == nil {
			if logger.GetLogger().V(logger.Warning) {
				logger.GetLogger().Log(logger.Warning, "CmdColsInfo with no cursor, possible after a failed query?")
			}
			// no error returned, this happens if the query fails so the client doesn't expect response
			break
		}
		var cts []*sql.ColumnType
		cts, err = cp.rows.ColumnTypes()
		if err != nil {
			if logger.GetLogger().V(logger.Warning) {
				logger.GetLogger().Log(logger.Warning, "rows.Columns()", err.Error())
			}
			break
		}
		if cts == nil {
			ns := netstring.NewNetstringFrom(common.RcValue, []byte("0"))
			err = WriteAll(cp.SocketOut, ns)
		} else {
			nss := make([]*encoding.Packet, len(cts)*5+1)
			nss[0] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.Itoa(len(cts))))
			var cnt = 1
			var width, prec, scale int64
			var ok = true
			for _, ct := range cts {
				nss[cnt] = netstring.NewNetstringFrom(common.RcValue, []byte(ct.Name()))
				cnt++
				typename := ct.DatabaseTypeName()
				if len(typename) == 0 {
					typename = "UNDEFINED"
				}
				nss[cnt] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.Itoa(cp.adapter.GetColTypeMap()[strings.ToUpper(typename)])))
				cnt++
				width, ok = ct.Length()
				if !ok {
					width = 0
				}
				nss[cnt] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.FormatInt(width, 10)))
				cnt++
				prec, scale, ok = ct.DecimalSize()
				if !ok {
					prec = 0
					scale = 0
				}
				if logger.GetLogger().V(logger.Debug) {
					logger.GetLogger().Log(logger.Debug, "colinfo", cnt, ct.Name(), typename, width, prec, scale)
				}
				//
				// java int is 32bit, HeraClientImpl.java has
				// meta.setPrecision(Integer.parseInt(new String(obj.getData())))
				// that would not take value like 9223372036854775807.
				//
				if prec > 2147483647 {
					prec = 2147483647
				}
				if scale > 2147483647 {
					scale = 2147483647
				}
				nss[cnt] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.FormatInt(prec, 10)))
				cnt++
				nss[cnt] = netstring.NewNetstringFrom(common.RcValue, []byte(strconv.FormatInt(scale, 10)))
				cnt++
			}
			resns := netstring.NewNetstringEmbedded(nss)
			err = WriteAll(cp.SocketOut, resns)
		}
	case common.CmdCommit:
		if logger.GetLogger().V(logger.Debug) {
			logger.GetLogger().Log(logger.Debug, "Commit")
		}
		if cp.tx != nil {
			calevt := cal.NewCalEvent("COMMIT", "Local", cal.TransOK, "")
			err = cp.tx.Commit()
			if err != nil {
				cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "Commit error:", err.Error())
				}
				calevt.AddDataStr("RC", err.Error())
				calevt.SetStatus(cal.TransError)
			} else {
				cp.tx = nil
			}
			calevt.Completed()
		} else {
			if logger.GetLogger().V(logger.Warning) {
				logger.GetLogger().Log(logger.Warning, "Commit issued without a transaction")
			}
		}
		if err == nil {
			cp.inTrans = false
			cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcOK, nil))
		} else {
			cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
			err = nil
		}
	case common.CmdRollback:
		if cp.tx != nil {
			calevt := cal.NewCalEvent("ROLLBACK", "Local", cal.TransOK, "")
			err = cp.tx.Rollback()
			if err != nil {
				cp.adapter.ProcessError(err, &cp.WorkerScope, &cp.queryScope)
				if logger.GetLogger().V(logger.Warning) {
					logger.GetLogger().Log(logger.Warning, "Rollback error:", err.Error())
				}
				calevt.AddDataStr("RC", err.Error())
				calevt.SetStatus(cal.TransError)
			} else {
				cp.tx = nil
			}
			calevt.Completed()
		} else {
			if logger.GetLogger().V(logger.Warning) {
				logger.GetLogger().Log(logger.Warning, "Rollback issued without a transaction")
			}
		}
		if err == nil {
			cp.inTrans = false
			cp.eor(common.EORFree, netstring.NewNetstringFrom(common.RcOK, nil))
		} else {
			cp.eor(common.EORInTransaction, netstring.NewNetstringFrom(common.RcSQLError, []byte(err.Error())))
			err = nil
		}
	}
	}

	logger.GetLogger().Log(logger.Verbose, "Finished outerloop")
	return err
}

func (cp *CmdProcessor) SendDbHeartbeat() bool {
	var masterIsUp bool
	masterIsUp = cp.adapter.Heartbeat(cp.db)
	return masterIsUp
}

// InitDB performs various initializations at start time
func (cp *CmdProcessor) InitDB() error {
	if logger.GetLogger().V(logger.Info) {
		logger.GetLogger().Log(logger.Info, "setup db connection.")
	}
	var err error
	cp.db, err = cp.adapter.InitDB()
	if err != nil {
		if logger.GetLogger().V(logger.Warning) {
			logger.GetLogger().Log(logger.Warning, "driver error", err.Error())
		}
		return err
	}
	cp.ctx = context.Background()
	cp.db.SetMaxIdleConns(1)
	cp.db.SetMaxOpenConns(1)

	//
	cp.sqlParser, err = common.NewRegexSQLParser()
	if err != nil {
		if logger.GetLogger().V(logger.Warning) {
			logger.GetLogger().Log(logger.Warning, "bindname regex complie:", err.Error())
		}
		return err
	}
	// MySQL can have ` as the first character in the table name as well as the column_name
	cp.regexBindName, err = regexp.Compile(":([`]?[a-zA-Z])\\w*[`]?")
	if err != nil {
		if logger.GetLogger().V(logger.Warning) {
			logger.GetLogger().Log(logger.Warning, "bindname regex complie:", err.Error())
		}
		return err
	}

	return nil
}

// TODO: Needs MySQL integration
func (cp *CmdProcessor) eor(code int, ns *encoding.Packet) error {
	if (code == common.EORFree) && cp.moreIncomingRequests() {
		code = common.EORMoreIncomingRequests
	}
	if (code == common.EORFree) && (cp.calSessionTxn != nil) {
		cp.calSessionTxn.Completed()
		cp.calSessionTxn = nil
	}
	var payload []byte
	if ns != nil {
		payload = make([]byte, len(ns.Serialized)+1 /*code*/ +2 /*rqId*/)
		payload[0] = byte('0' + code)
		payload[1] = byte(cp.rqId >> 8)
		payload[2] = byte(cp.rqId & 0xFF)
		copy(payload[3:], ns.Serialized)
	} else {
		payload = []byte{byte('0' + code), byte(cp.rqId >> 8), byte(cp.rqId & 0xFF)}
	}
	cp.heartbeat = true
	return WriteAll(cp.SocketOut, netstring.NewNetstringFrom(common.CmdEOR, payload))
}

func (cp *CmdProcessor) calExecErr(field string, err string) {
	cp.calExecTxn.AddDataStr(field, err)
	cp.calExecTxn.SetStatus(cal.TransError)
	cp.calExecTxn.Completed()
	cp.calExecTxn = nil
}

/**
 * extract bindnames and save them in bindVars with their position index.
 * replace bindnames in query with "?"
 */
func (cp *CmdProcessor) preprocess(packet *encoding.Packet) string {
	//
	// @TODO strip comment sections which could have ":".
	//

	var query string

	if !packet.IsMySQL {
		query = string(packet.Payload)
		//
		// SELECT account_number,flags,return_url,time_created,identity_token FROM wseller
		// WHERE account_number=:account_number
		// and flags=:flags and return_url=:return_url,
		//
		binds := cp.regexBindName.FindAllString(query, -1)
		//
		// just create a new map for each query. the old map if any will be gc out later.
		//
		cp.bindVars = make(map[string]*BindValue)
		cp.bindPos = make([]string, len(binds))
		for i, val := range binds {
			cp.bindVars[val] = &(BindValue{index: i, name: val, valid: false, btype: btUnknown})
			cp.bindPos[i] = val
		}
		if !(cp.adapter.UseBindNames()) {
			query = cp.regexBindName.ReplaceAllString(query, "?")
		}
		return query
	} else {
		logger.GetLogger().Log(logger.Debug, "Out here in the preprocessing")
		if len(packet.Payload) == 0 {
			return ""
		}

		query = string(packet.Payload[1:])
		logger.GetLogger().Log(logger.Debug, "Acquired the query: ", query)
		//
		// SELECT account_number,flags,return_url,time_created,identity_token FROM wseller
		// WHERE account_number=:account_number
		// and flags=:flags and return_url=:return_url,
		//
		binds := cp.regexBindName.FindAllString(query, -1)
		logger.GetLogger().Log(logger.Debug, "Did some binding")
		//
		// just create a new map for each query. the old map if any will be gc out later.
		//
		cp.bindVars = make(map[string]*BindValue)
		cp.bindPos = make([]string, len(binds))
		for i, val := range binds {
			cp.bindVars[val] = &(BindValue{index: i, name: val, valid: false, btype: btUnknown})
			cp.bindPos[i] = val
		}

		// Get the number of columns in the query
		logger.GetLogger().Log(logger.Debug, "pls")
		splits := strings.Split(strings.ToLower(query), " as ")
		logger.GetLogger().Log(logger.Debug, "really?", splits)
		if len(splits) > 1 {
			cp.bindOuts = strings.SplitN(splits[1], ",", -1)
			cp.numColumns = len(cp.bindOuts)
			cp.numBindOuts = cp.numColumns
		}
		// cp.stmts[cp.currsid] = query
		logger.GetLogger().Log(logger.Debug, "WHICH PART FAILED")
		return query
	}
}

func (cp *CmdProcessor) isIdle() bool {
	return !(cp.inCursor) && !(cp.inTrans)
}
