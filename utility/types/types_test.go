package main

import (
	"reflect"
//	"fmt"
	"github.com/paypal/hera/utility/encoding/netstring"
	"github.com/paypal/hera/utility/encoding/mysqlpackets"
	"github.com/paypal/hera/utility/encoding"
	"testing"
)

func TestTypes(t *testing.T) {
	sql := &mysqlpackets.MySQLPacket{}
	ns := &netstring.Netstring{}
	ps := &encoding.Packet{}

	t.Log("Type of ps: ", reflect.TypeOf(ps))

	t.Log("Type of ns: ", reflect.TypeOf(ns), "Type of sql: ", reflect.TypeOf(sql))

	switch interface{}(sql).(type) {
	case (*mysqlpackets.MySQLPacket):
		t.Log("MySQLPacket")
		t.Fail()

	case (*netstring.Netstring):
		t.Log("netstring")
		t.Fail()
	}
	t.Fail()

}


/*
* In conclusion, you can import encoding, encoding/netstring, and encoding/mysqlpackets 
* and it doesn't give a cyclic error. So we're good.
*/
