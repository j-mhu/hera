# Removing the JDBC-HERA Driver

It would be of interest to make Hera compatible with MySQL protocol.
Hera is already built on netstring encoding, and uses netstring exclusively
to send messages across channels and conns internally. Any client that connects
to Hera must connect through the JDBC-HERA driver in order to use Hera, which can
be inconvenient.

Contents include:
1. how to run MySQL queries against a driverless Hera,
2. how to modify Hera source code to expand capabilities for MySQL clients.

## How to run MySQL queries against a driverless Hera
This folder contains a test file `main_test.go` which uses [go-sql-driver](https://godoc.org/github.com/go-sql-driver/mysql)
to run SQL queries against the mock server in [`tests/mocksqlsrv`](https://github.com/paypal/hera/tree/master/tests/mocksqlsrv).
You can modify `main_test.go` to run whatever queries you want.

You will need to have the mock server running before running the test.
Instructions to start up the server are in the README for that folder.

From this directory,

```
chmod +x herasql/setup.sh
./herasql/setup.sh
```

In a separate terminal, same directory, run `go test`. Bask in the success of
the `PASS`.

> `setup.sh` sets environmental variables necessary to run Hera. It also installs
the binaries for mysqlworker and mux, and then runs the `driverless.go` file,
which sets up a Hera server without the JDBC-driver in front of it. You can modify
`setup.sh` do any pre-server startup tasks, or to change the port number.

> Please make sure that the `TWO_TASK` and `TWO_TASK_READ` variables have the same port
number set as the mock server, if you're running on localhost.

> Otherwise, if you want to change the connected database to a real MySQL server, change the `TWO_TASK` and `TWO_TASK_READ` variables to the appropriate DSN.

## Major modifications to Hera source code

Wherever I have modified code, I have left explanations and comments to explain
why MySQL protocol requires specific logic, different handling, etc.
The files I've touched include

- [codes.go](https://github.com/j-mhu/hera/tree/master/common/codes.go)
- [config.go](https://github.com/j-mhu/hera/tree/master/config/config.go)
- [connectionhandler.go](https://github.com/j-mhu/hera/tree/master/lib/connectionhandler.go)
- [coordinator.go](https://github.com/j-mhu/hera/tree/master/lib/coordinator.go)
- [netstring.go](https://github.com/j-mhu/hera/tree/master/utility/encoding/netstring.go)
- [util.go](https://github.com/j-mhu/hera/tree/master/lib/util.go)
- [workerclient.go](https://github.com/j-mhu/hera/tree/master/lib/workerclient.go)
- [worker/cmdprocessor.go](https://github.com/j-mhu/hera/tree/master/worker/shared/cmdprocessor.go)
- [worker/common.go](https://github.com/j-mhu/hera/tree/master/worker/shared/common.go)
- [worker/workerservice.go](https://github.com/j-mhu/hera/tree/master/worker/shared/workerservice.go)


Some of these files were just modified to add debugging statements to the logger.
The most important file is `cmdprocessor.go`.

The client driver located in `hera/client/gosqldriver` looks like it was modified in
commits, but I reverted changes from a previous design. In other words, it's the same as the original.

#### 1. **Handshake** ####

In `lib/connectionhandler.go`, two functions were added. These are
`sendHandshake()` and `readHandshakeResponse()`.

- `sendHandshake` sends a HandshakeV10 through the client’s connection.
- `readHandshakeResponse()` therefore reads a HandshakeResponseV41 from the
client’s connection.
     - `readHandshakeResponse()` also sends an OK Packet to the client to
     indicate that the client can enter the command phase.

This is why these two functions are outside of `lib/coordinator.go`.
Coordinator code should just deal with the command phase for MySQL,
and authentication and connection should happen in the connection handler.

#### 2. **Changes to netstring encoding and adding mysqlpacket encoding.** ####

All packets now have the general form:
```go
type Packet struct {
	Cmd           int 		// command byte or opcode
	Serialized    []byte		// the full packet including header
	Payload       []byte		// payload only
	Sqid          int 		// sequence id
	Length        int 		// length of the payload
}
```

An incoming netstring or mysqlpacket will be packaged into a packet, which
is passed around in channels and conns in Hera. Both of them are similar because
they prepend information about the payload to the actual payload.

A change was made to netstring encoding. Here are the original encodings:

```
Netstring general format Serialized: 		LENGTH:PAYLOAD
MySQL packet general format Serialized: 	HEADER PAYLOAD
```

Now, for Hera internal-specific encoding, they are modified to look like this:

ENCODING | INDICATOR | PREPENDED | PAYLOAD
--- | --- | --- | ---
netstring | **0x01** | LENGTH | ...
mysqlpacket | **0x00** | HEADER | ...

where INDICATOR is a byte that differentiates between netstring and mysqlpacket.
After a mysqlpacket or netstring enters Hera through the client's conn,
it is wrapped in this encoding and packaged into a Packet. All `Packet`s are expected to have an indicator byte.

For nested netstrings, the 0x01 byte is deep-nested as well. This means that each netstring, at every nesting depth, inside the nested netstring has an indicator byte. An example with netstring depth 2 and 3 strings would look like this.
```
0x01 LENGTH:NESTED( 0x01 LENGTH:PAYLOAD, 0x01 LENGTH:PAYLOAD, 0x01 LENGTH:PAYLOAD )
```
All tests for netstring and MySQLPackets were modified to reflect this change. The modifications were made to the input test strings, and nowhere else.
The only functions that had to be changed were netstring and mysqlpacket functions.

A consequence is that we cannot know what the packet type is until after we’ve tried reading the first byte. This motivates a new error called `WRONG_PACKET` that implements the error interface. I initialized one single instance of it. This gets sent any time a MySQL packet is read using netstring functions, or vice versa.

Similarly, if the indicator byte is not present, or it is not 0x00 or 0x01, then we should raise an `UNKNOWN_PACKET` error.
Both are defined in [`utility/encoding`](https://github.com/j-mhu/hera/tree/master/utility/encoding), under `WRONGPACKET` and `UNKNOWNPACKET`.

An example of its use is this:

If the err returned from `NewNetstring(…)` is `encoding.WRONGPACKET`, then we should try to read the bytes again using `NewMySQLPacket(…)`.
`NewNetstring` and `NewMySQLPacket` were modified to read from a buffer, so that we
do not consume input on a packet misread. See [workerclient.go: doRead()](https://github.com/j-mhu/hera/tree/master/lib/workerclient.go) for an example.


#### 3. Adding a MySQL case for all worker request handling code. ####

There are some differences in how SQL queries are processed and handled
between OCC wire protocol and general MySQL packet reading and handling.

As a result, there are a few places with very important TODOs.

* cmdprocessor.go
     - Provide support for unsupported command codes below.
     - There are specific fields that need to be added to the command processor struct to
     keep track of statements. In my code, currently a map with int keys and sql.Stmt values
     is used. This is for prepared statements.  
     - Not all of these commands are relevant or should be handled exactly
     as if Hera were a MySQL DBMS server.
          - For example, `COM_QUIT` is unnecessary
     since workers return to the pool after the dispatched request is finished.
          - Similarly, `COM_CLOSE` should not shut down a worker's connection
     to a database.
     - Fix segfaulting when client closes the connection...
     - Hera records the state of transactions and always updates the state variables for OCC commands. The same needs to be done for MySQL commands, but there may be subtle differences.
          - Code for `COM_QUERY` is complete and could be used as an example.
* mysqlpackets.go
     - Implement ReconstructColumnDefinition
     - Implement BinaryProtocolResultSet
     - Implement ResultsetRow

* connectionhandler.go, server.go, config.go
     - Set configuration to use MySQL vs OCC wire protocol. This currently needs to be done manually.

Currently supported commands:

- [ ] COM_SLEEP
- [x] COM_QUIT
- [x] COM_INIT_DB
- [x] COM_QUERY
- [ ] COM_FIELD_LIST
- [x] COM_CREATE_DB 		
- [x] COM_DROP_DB
- [ ] COM_REFRESH
- [ ] COM_SHUTDOWN
- [ ] COM_STATISTICS
- [ ] COM_PROCESS_INFO 		
- [ ] COM_CONNECT
- [ ] COM_PROCESS_KILL
- [ ] COM_DEBUG
- [ ] COM_PING
- [ ] COM_TIME 				
- [ ] COM_DELAYED_INSERT
- [ ] COM_CHANGE_USER
- [ ] COM_BINLOG_DUMP
- [ ] COM_TABLE_DUMP
- [ ] COM_CONNECT_OUT  		
- [ ] COM_REGISTER_SLAVE
- [ ] COM_STMT_PREPARE
- [ ] COM_STMT_EXECUTE
- [ ] COM_STMT_SEND_LONG_DATA
- [x] COM_STMT_CLOSE 		
- [ ] COM_STMT_RESET
- [ ] COM_SET_OPTION
- [ ] COM_STMT_FETCH
- [ ] COM_RESET_CONNECTION
- [ ] COM_DAEMON


#### 4. Works in progress. ####
Our biggest priority is supporting prepared statements, because the majority
of queries people are interested in will return result rows. For example,
SELECTs. COM_QUERY works well enough, but a lot of SQL APIs automatically
use COM_STMT_PREPARE under the hood for optimization and security.

- This is because prepared statements are parsed once and cached so that they
don't need to be hard parsed a second time by the database.
- Also, binding the variables into the query can foil SQL injections.

The steps that need to be taken are mostly in `cmdprocessor.go` and `mysqlpackets.go`.

The most relevant pages are on [`COM_STMT_PREPARE`](https://dev.mysql.com/doc/dev/mysql-server/8.0.12/page_protocol_com_stmt_prepare.html),
[`COM_STMT_EXECUTE`](https://dev.mysql.com/doc/dev/mysql-server/8.0.12/page_protocol_com_stmt_execute.html),
[`COM_STMT_PREPARE Response`](https://dev.mysql.com/doc/internals/en/com-stmt-prepare-response.html#packet-COM_STMT_PREPARE_OK),
[`Binary Protocol Result Set`](https://dev.mysql.com/doc/internals/en/binary-protocol-resultset.html),
and [`Protocol::ColumnDefinition`](https://dev.mysql.com/doc/internals/en/com-query-response.html#packet-Protocol::ColumnDefinition).

`mysqlpackets.go` has some rudimentary `ColumnDefinition` reconstruction code, and
most of `COM_STMT_PREPARE_OK`. `cmdprocessor.go` has most of the logic written
or commented for `COM_STMT_PREPARE`.

There was an attempt to write code for result sets and logic for `COM_STMT_EXECUTE`
in both `cmdprocessor.go` and `mysqlpackets.go`. The comments are there even if
there is currently no implementation.

Other commands are lower priority.

##### a. Issues with prepared statements. #####
- We must reconstruct column definition packets to send when the client
issues a COM_STMT_PREPARE command. We can obtain information about a schema's
column from ColumnTypes. However, because we use database/sql,
ColumnTypes are only accessible from sql.Rows. However, Prepare returns a sql.Stmt,
not sql.Rows. sql.Rows are only returned from Exec or Query.
     - In other words, the problem is that we need to execute the query
     when the client requests for it to be prepared.

## Moving forward for a more intelligent Hera. ##
Recommendations and suggestions from Hera/OCC team and myself:

- Hera should be able to differentiate MySQL clients by packet, port, or some other method.
- Keep netstring encoding as original and create a separate port to receive MySQL connections.
- Connect directly to MySQL database similar to how go-sql-driver does, and exchange raw packets directly.
     - This minimizes the overhead of rewriting response packets to the client, and all the packet data received from the database is exposed to the Hera server instead of through the limited window of the Go SQL driver.
