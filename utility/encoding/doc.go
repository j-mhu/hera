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

type Packet struct {
     Cmd            int
     Serialized     []byte
     Payload        []byte
     Length         int
     Sequence_id    int
     IsMySQL          bool
}

const (
	// CodeSubCommand is a special command used to define that the payload contains multiple netstrings
	CodeSubCommand = '0'
     MAX_PACKET_SIZE     int = (1 << 24) - 1
)

// IsComposite returns if the netstring is compisite, embedding multiple netstrings in it
func (ns *Packet) IsComposite() bool {
	if ns.IsMySQL {
          return ns.Length == MAX_PACKET_SIZE
     }
     return ns.Cmd == (CodeSubCommand - '0')
}

type Packager interface {

     // Creates a new packet reader for the packager
     NewPacketReader(_reader io.Reader)

     // NewNetstring creates a Netstring from the reader, reading exactly
     // as many bytes as necessary.
     NewPacket(_reader io.Reader) (*Packet, error)

     // NewNetstringFrom creates a Netstring from command and payload
     NewPacketFrom(_cmd int, _payload []byte) *Packet

     // // NewNetstringEmbedded embeds a set of Netstrings into a netstring. May be put somewhere else...

     // MultiplePackets reads all of the incoming packets. It stores
     // all packets in nss. A pointer to the 'current'
     ReadMultiplePackets(_ns *Packet) ([]*Packet, error)

     // ReadNext returns the next packet from the stream.
     ReadNext() (ns *Packet, err error)
}
