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

## Major modifications to Hera source code

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
After a mysqlpacket/netstring enters Hera through the client's conn,
it is wrapped in this encoding and packaged into a Packet. All
Packets are expected to have an indicator byte.

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

If the err returned from NewNetstring(…) is encoding.WRONGPACKET, then we should try to read the bytes again using NewMySQLPacket(…).
NewNetstring and NewMySQLPacket were modified to read from a buffer, so that we
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
          - For example, COM_QUIT is unnecessary
     since workers return to the pool after the dispatched request is finished.
          - Similarly, COM_CLOSE, should not shut down a worker's connection
     to a database, it should just stop the request and sent the worker by to
     the idle connection pool.
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

## Moving forward for a more intelligent Hera. ##
Recommendations and suggestions from Hera/OCC team and myself:

- Hera should be able to differentiate MySQL clients by packet, port, or some other method.
- Keep netstring encoding as original and create a separate port to receive MySQL connections.
- Open port directly to database similar to how go-sql-driver does, and process raw packets directly.
     - This minimizes the overhead of rewriting response packets to the client, and all the packet data received from the database is exposed to the Hera server instead of through the limited window of the Go SQL driver.
