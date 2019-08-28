# Removing the JDBC-HERA Driver

## Implications

## How to run MySQL queries against a driverless Hera


## Major modifications to Hera source code

1. **HANDSHAKE PACKET AND HANDSHAKE RESPONSE**

In connectionhandler.go, two functions were added. These are `sendHandshake()` and `readHandshakeResponse()`.

- `sendHandshake` sends a HandshakeV10 through the client’s connection.
- `readHandshakeResponse()` therefore reads a HandshakeResponseV41 from the client’s connection. readHandshakeResponse() also sends an OK Packet to the client to indicate that the client can enter the command phase.

Placing these two functions outside of `lib/coordinator.go` is due to this reason. Coordinator code should just deal with the command phase for MySQL. Authentication and connection should happen in the connection handler.


2. **REVISIONS TO NETSTRING ENCODING, MYSQLPACKET ADDITION.**

All packets have the general form:
```go
type Packet struct {
	Cmd           int 		// command byte or opcode
	Serialized    []byte		// the full packet including header
	Payload       []byte		// payload only
	Sqid          int 		// sequence id
	Length        int 		// length of the payload
}
```

This change only applies to packets communicated INTERNALLY in Hera through channels.

Original:
Netstring general format Serialized: 		LENGTH + PAYLOAD
MySQL packet general format Serialized: 	HEADER + PAYLOAD

Modified:

Netstring Serialized:		 INDICATOR + LENGTH + PAYLOAD
MySQL Serialized:			 INDICATOR + HEADER + PAYLOAD

where INDICATOR is the byte 0x00 for MySQL and 0x01 for Netstring.

For nested netstrings, the 0x01 byte is deep-nested as well. This means that each netstring, at every nesting depth, inside the nested netstring has an indicator byte. An example with netstring depth=2 with 3 strings would look like this.

	0x01 LENGTH NESTED( 0x01 LENGTH PAYLOAD, 0x01 LENGTH PAYLOAD, 0x01 LENGTH PAYLOAD )

All tests for netstring and MySQLPackets were modified to reflect this change. The modifications were made to the input test strings, and nowhere else. All of them pass.

A consequence is that we cannot know what the packet type is until after we’ve tried reading the first byte. This motivates a new error called `WRONG_PACKET` that implements the error interface. I initialized one single instance of it. This gets sent any time a MySQL packet is read using netstring functions, or vice versa.

Similarly, if the indicator byte is not present, or it is not 0x00 or 0x01, then we should raise an `UNKNOWN_PACKET` error. This is also defined in `utility/encoding`, under `WRONG_PACKET`.

An example of its use is this:

If the err returned from NewNetstring(…) is encoding.WRONGPACKET, then we should try to read the bytes again using NewMySQLPacket(…). See workerclient.go: function doREAD() for an example.


3. Adding a MySQL case for all worker request handling code.
4. Places with important TODOS
    * cmdprocessor.go.       all of processCmd for any command aside from `COM_QUERY`

    Currently supported commands:
     - [ ] COM_SLEEP
	- [x] COM_QUIT
	- [ ] COM_INIT_DB
	- [x] COM_QUERY
	- [ ] COM_FIELD_LIST
	- [ ] COM_CREATE_DB 		
	- [ ] COM_DROP_DB
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
	- [ ] COM_STMT_CLOSE 		
	- [ ] COM_STMT_RESET
	- [ ] COM_SET_OPTION
	- [ ] COM_STMT_FETCH
	- [ ] COM_RESET_CONNECTION
	- [ ] COM_DAEMON 			
