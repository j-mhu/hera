// This test checks whether or not netstring and mysqlpackets
// properly implements the Packager interface becuase of the iffyness
// that comes with the Reader interface.

package packagertest

import (
	"testing"
     "bytes"
	"github.com/paypal/hera/utility/encoding/netstring"
     "github.com/paypal/hera/utility/encoding/mysqlpackets"
     "github.com/paypal/hera/utility/encoding"
     "reflect"
)

func tmake(t *testing.T) ([]*encoding.Packet, []encoding.Packager){

     mspackager := mysqlpackets.MySQLPacket{}
     nspackager := netstring.Netstring{}

     packets := make([]*encoding.Packet, 2)
     packagers := make([]encoding.Packager, 2)

     // Simple MySQL query packet
     query := []byte{1, 00, 00, 00, 1}
     ms := mspackager.NewPacketFrom(0, query)
     t.Log("Created simple MySQL query packet:", query)

     // Simple netstring query packet
     ns := encoding.Packet{Cmd: 502, Payload: []byte("0"), Serialized: []byte("5:502 0,"), IsMySQL:false}
     t.Log("Created simple netstring query packet: 5:502 0")

     // Create list of packets
     packets[0] = ms
     packets[1] = &ns

     // Create list of packagers
     packagers[0] = &mspackager
     packagers[1] = &nspackager

     return packets, packagers
}

func TestNewPacket(t *testing.T) {
     // Test cases and test packagers
     tcs, tps := tmake(t)
     for i := 0; i < len(tcs); i++ {
          p := tps[i]
          pkt := tcs[i]
          t.Log("Creating new packet from ", pkt.Serialized)
          msg, err := p.NewPacket(bytes.NewReader(pkt.Serialized))

          if err != nil {
               t.Log("Error:", err)
          }

          // Test that the packet read is as expected!
		if msg.Length != pkt.Length {
			t.Log("Length expected", pkt.Length, "instead got", msg.Length)
		}
		if msg.Sequence_id != pkt.Sequence_id {
			t.Log("Sequence id expected", pkt.Sequence_id, "instead got", msg.Sequence_id)
		}
		if msg.Cmd != pkt.Cmd {
			t.Log("Command expected", pkt.Cmd, "instead got", msg.Cmd)
			t.Fail()
		}
		if !reflect.DeepEqual(msg.Serialized, pkt.Serialized) {
			t.Log("Payload expected", pkt.Serialized, "instead got", msg.Serialized)
			t.Fail()
		}
     }
}

func TestNewPacketFrom(t *testing.T) {
     // Test cases and test packagers
     tcs, tps := tmake(t)
     for i := 0; i < len(tcs); i++ {
          p := tps[i]
          pkt := tcs[i]
          t.Log("Creating new packet from ", pkt.Payload)
          var msg *encoding.Packet
          if (pkt.IsMySQL) {
               msg = p.NewPacketFrom(0, pkt.Payload)
          } else {
               msg = p.NewPacketFrom(pkt.Cmd, pkt.Payload)
          }

          // Test that the packet read is as expected!
		if msg.Length != pkt.Length {
			t.Log("Length expected", pkt.Length, "instead got", msg.Length)
		}
		if msg.Sequence_id != pkt.Sequence_id {
			t.Log("Sequence id expected", pkt.Sequence_id, "instead got", msg.Sequence_id)
		}
		if msg.Cmd != pkt.Cmd {
			t.Log("Command expected", pkt.Cmd, "instead got", msg.Cmd)
			t.Fail()
		}
		if !reflect.DeepEqual(msg.Serialized, pkt.Serialized) {
			t.Log("Payload expected", pkt.Serialized, "instead got", msg.Serialized)
			t.Fail()
		}
     }
}


func TestReadNext(t *testing.T) {
     // Test cases and test packagers
     tcs, tps := tmake(t)
     for i := 0; i < len(tcs); i++ {
          p := tps[i]
          pkt := tcs[i]
          t.Log("Creating new packet reader from ", pkt.Serialized)
          p.NewPacketReader(bytes.NewReader(pkt.Serialized))
          for {
               t.Log("Reading")
               msg, err := p.ReadNext()
               if err != nil {
                    break
               }

               // Test that the packet read is as expected!
     		if msg.Length != pkt.Length {
     			t.Log("Length expected", pkt.Length, "instead got", msg.Length)
     		}
     		if msg.Sequence_id != pkt.Sequence_id {
     			t.Log("Sequence id expected", pkt.Sequence_id, "instead got", msg.Sequence_id)
     		}
     		if msg.Cmd != pkt.Cmd {
     			t.Log("Command expected", pkt.Cmd, "instead got", msg.Cmd)
     			t.Fail()
     		}
     		if !reflect.DeepEqual(msg.Serialized, pkt.Serialized) {
     			t.Log("Payload expected", pkt.Serialized, "instead got", msg.Serialized)
     			t.Fail()
     		}
          }
     }
}
