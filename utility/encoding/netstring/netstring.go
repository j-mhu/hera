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

// Package netstring contains encoding and decoding functions in netstring format
package netstring

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/paypal/hera/utility/encoding"
	"github.com/paypal/hera/utility/logger"
	"io"
)

const (
	colon byte = ':'
	comma byte = ','
	space byte = ' '
	// CodeSubCommand is a special command used to define that the payload contains multiple netstrings
	CodeSubCommand = '0'
)

// NewInitNetstring creates a Netstring from the reader, reading exactly as many bytes as necessary. Assumes
// that this is the initial request received from the client, so it doesn't initially have the MySQL vs netstring
// encoding indicator byte.
func NewInitNetstring(_reader io.Reader) (*encoding.Packet, error) {
	ns := &encoding.Packet{}

	var buff bytes.Buffer
	var tmp = make([]byte, 1)
	var digit int
	var err error
	// read length
	length := 0

	if err == io.EOF {
		return nil, err
	}

	for {
		_, err = _reader.Read(tmp)
		b := tmp[0]
		if err != nil {
			return nil, err
		}
		buff.WriteByte(b)
		if b == colon {
			break
		} else {
			digit = int(b - '0')
			if (digit < 0) || (digit > 9) {
				return nil, errors.New("Expected digit reading length")
			}
			length = length*10 + digit
		}
	}

	//read the rest
	totalLen := length + buff.Len() + 1 /*comma*/
	ns.Serialized = make([]byte, totalLen + 1) // + 1 is for indicator byte
	ns.Serialized[0] = 1 // indicates netstring
	copy(ns.Serialized[1:], buff.Bytes())
	bytesRead := buff.Len() + 1
	var n int
	for bytesRead < totalLen {
		n, err = _reader.Read(ns.Serialized[bytesRead:])
		if err != nil {
			return nil, err
		}
		bytesRead += n
	}
	// read command
	next := buff.Len() + 1
	for next < (totalLen) {
		if ns.Serialized[next] == space {
			next++
			break
		}
		digit = int(ns.Serialized[next] - '0')
		if (digit < 0) || (digit > 9) {
			return nil, errors.New("Expected digit reading command")
		}
		ns.Cmd = ns.Cmd*10 + digit
		next++
	}
	ns.IsMySQL = false
	ns.Payload = ns.Serialized[next : totalLen]
	return ns, nil
}

// NewNetstring creates a Netstring from the reader, reading exactly as many bytes as necessary
func NewNetstring(reader io.Reader) (*encoding.Packet, error) {
	logger.GetLogger().Log(logger.Info, "Inside Netstring")
	ns := &encoding.Packet{}

	_reader := bufio.NewReader(reader)

	var buff bytes.Buffer
	// var tp = make([]byte, 1)
	var tmp = make([]byte, 1)
	var digit int
	var err error


	// read length
	length := 0
	// Read in type byte
	ttp, err := _reader.ReadByte()
	if err == io.EOF {
		return nil, err
	}

	if int(ttp) != 1 {
		if int(ttp) == 0 {
			return nil, encoding.WRONGPACKET
		}
		return nil, encoding.UNKNOWNPACKET
	}


	for {
		_, err = _reader.Read(tmp)
		b := tmp[0]
		if err != nil {
			return nil, err
		}
		buff.WriteByte(b)
		if b == colon {
			break
		} else {
			digit = int(b - '0')
			if (digit < 0) || (digit > 9) {
				return nil, errors.New("Expected digit reading length")
			}
			length = length*10 + digit
		}
	}

	//read the rest
	totalLen := length + buff.Len() + 1 /*comma*/
	ns.Serialized = make([]byte, totalLen + 1) // + 1 is for indicator byte
	ns.Serialized[0] = 1 // indicates netstring
	copy(ns.Serialized[1:], buff.Bytes())
	bytesRead := buff.Len() + 1
	var n int
	for bytesRead < totalLen {
		n, err = _reader.Read(ns.Serialized[bytesRead:])
		if err != nil {
			return nil, err
		}
		bytesRead += n
	}
	// read command
	next := buff.Len() + 1
	for next < (totalLen) {
		if ns.Serialized[next] == space {
			next++
			break
		}
		digit = int(ns.Serialized[next] - '0')
		if (digit < 0) || (digit > 9) {
			return nil, errors.New("Expected digit reading command")
		}
		ns.Cmd = ns.Cmd*10 + digit
		next++
	}
	ns.IsMySQL = false
	ns.Payload = ns.Serialized[next : totalLen]
	logger.GetLogger().Log(logger.Info, "Finished Netstring")
	return ns, nil
}

// NewNetstringFrom creates a Netstring from command and Payload
func NewNetstringFrom(_cmd int, _payload []byte) *encoding.Packet {
	// TODO: optimize
	payloadLen := len(_payload)
	cmdStr := fmt.Sprintf("%d", _cmd)
	var str string
	if payloadLen == 0 {
		str = fmt.Sprintf("%d:%s,", len(cmdStr), cmdStr)
	} else {
		str = fmt.Sprintf("%d:%s %s,", payloadLen+len(cmdStr)+1 /*the space*/, cmdStr, string(_payload))
	}
	ns := new(encoding.Packet)
	ns.Cmd = _cmd
	ns.IsMySQL = false
	byteStr := []byte(str)
	ns.Serialized = append([]byte{1}, byteStr...)
	if payloadLen > 0 {
		totalLen := len(ns.Serialized[1:])
		ns.Payload = ns.Serialized[totalLen-payloadLen : totalLen]
	}

	return ns
}

// NewNetstringEmbedded embedds a set of Netstrings into a netstring
func NewNetstringEmbedded(_netstrings []*encoding.Packet) *encoding.Packet {
	// TODO: optimize
	payloadLen := 0
	for _, i := range _netstrings {
		payloadLen += len(i.Serialized)
	}
	lenStr := fmt.Sprintf("%d:", payloadLen+2 /*len("0 ")*/)
	totalLen := len(lenStr) + payloadLen + 2 /*len("0 ")*/ + 1 /*ending comma*/
	ns := new(encoding.Packet)
	ns.Serialized = make([]byte, totalLen + 1) // plus indicator byte for nested netstring
	ns.Serialized[0] = 1
	copy(ns.Serialized[1:], []byte(lenStr))
	next := len(lenStr) + 1
	copy(ns.Serialized[next:], []byte{CodeSubCommand, space})
	next += 2
	for _, i := range _netstrings {
		copy(ns.Serialized[next:], i.Serialized)
		next += len(i.Serialized)
	}
	ns.Serialized[next] = byte(comma)
	ns.Payload = ns.Serialized[totalLen-payloadLen: totalLen]
	ns.IsMySQL = false
	return ns
}

// SubNetstrings parses the embedded Netstrings
func SubNetstrings(_ns *encoding.Packet) ([]*encoding.Packet, error) {
	//  TODO: optimize for zero-copy
	var nss []*encoding.Packet
	reader := bytes.NewReader(_ns.Payload)
	// fmt.Println("SubNetstrings: ", _ns.Payload)
	var ns *encoding.Packet
	var err error
	for {
		ns, err = NewNetstring(reader)
		if err == io.EOF {
			break
		}
		// fmt.Println(ns.Serialized)
		if err != nil {
			return nil, err
		}
		nss = append(nss, ns)
	}
	return nss, nil
}

// Reader decodes netstrings from a buffer
type Reader struct {
	reader io.Reader
	ns     *encoding.Packet
	nss    []*encoding.Packet
	next   int
}

// NewNetstringReader creates a Reader, that maintains the state for embedded Netstrings
func NewNetstringReader(_reader io.Reader) *Reader {
	nsr := new(Reader)
	nsr.reader = _reader
	return nsr
}

// ReadNext returns the next Netstring from the stream. Note: in case of embedded netstrings,
// the Reader will buffer some Netstrings
func (reader *Reader) ReadNext() (ns *encoding.Packet, err error) {
	logger.GetLogger().Log(logger.Info, "Inside netstring's ReadNext")
	for {
		if reader.ns != nil {
			ns = reader.ns
			reader.ns = nil
			return
		}
		if reader.next < len(reader.nss) {
			ns = reader.nss[reader.next]
			reader.next++
			return
		}
		reader.ns, err = NewNetstring(reader.reader)
		if err != nil {
			return nil, err
		}
		if reader.ns.Cmd == (CodeSubCommand - '0') {
			reader.nss, err = SubNetstrings(reader.ns)
			if err != nil {
				return nil, err
			}

			reader.ns = nil
			reader.next = 0
		}
	}
}