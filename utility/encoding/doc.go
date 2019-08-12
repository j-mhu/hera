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

// Package encoding provides the encoding functions such as netstring etc.,
package encoding

import "io"

/* Packaging interface for both netstrings or MySQL protocol packets.
* Note that Handshake packets may look slightly different / have a
* separate implementation.
*/
type Packet struct {
     Cmd  int            // Op code / command byte, etc.
     Serialized []byte   // Entire packet byte array including header, payload
     Payload    [] byte  // Content section (excludes header)
     Length int
	Sequence_id int
}

// Reader decodes netstrings from a buffer or stores information
// about the state of the sequence id when packets are exchanged between
// server and client.
type Reader struct {
     reader  io.Reader
     packet  *Packet
     packets []*Packet
     next    int
}

/* A Packet interface gives options for reading netstrings or MySQL packets.
* The functions listed below are common to both netstrings and MySQL packets.
* package netstring implements the Packaging interface. So does package
* mysqlpackets.
*/
type Packaging interface {

     // NewPacket creates a Packet from the reader, reading exactly as many
     // bytes as necessary specified in the MySQL packet header or netstring
     // length.
     NewPacket(_reader io.Reader) (*Packet, error)

     // NewPacketFrom creates a new Packet from command and payload. When
     // using mysqlpackets to implement the Packaging interface, can make
     // the cmd argument optional.
     NewPacketFrom(_cmd int, _payload []byte) *Packet

     // NewNetStringReader creates a Reader that maintains the state (e.g.)
     // counting the number of packets exchanged in sequence between client
     // and server or netstrings in a payload
     NewPacketReader(_reader io.Reader) *Reader

     ReadMultiplePackets(_p *Packet) ([]*Packet, error)

     // ReadNext returns the next packet from the string.
     ReadNext() (packet *Packet, err error)

     // IsComposite returns if the packet has multiple packets / embedded
     // netstrings.
     IsComposite() bool
}
