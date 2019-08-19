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

package lib

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/paypal/hera/client/gosqldriver"
	"github.com/paypal/hera/utility/encoding/netstring"
	"net"
	"strings"

	"github.com/paypal/hera/common"
	"github.com/paypal/hera/utility/logger"
)

/*
 * database/sql driver to be used internally by hera. That way components reading the configration from the database
 * (like sharding configuration for example) can be coded in standard GO SQL.
 * It is simply a wrapper over client/gosqldriver
 */
type heraLoopDriver struct {
}

// ConnHandlerFunc defines the signature of a fucntion that can be used as a callback by the loop driver
type ConnHandlerFunc func(net.Conn)

var connHandler ConnHandlerFunc

// RegisterLoopDriver installs the callback for the loop driver
func RegisterLoopDriver(f ConnHandlerFunc) {
	connHandler = f
	drvLoop := &heraLoopDriver{}
	sql.Register("heraloop", drvLoop)
}

/**
URL: <ShardID>:<PoolType>:<PoolID>
TODO: add another parameter for debugging/troubleshooting, IDing the client
*/
func (driver *heraLoopDriver) Open(url string) (driver.Conn, error) {
	cli, srv := net.Pipe()

	// Create packager for doing packets"
	nets := &netstring.Netstring{}
	go connHandler(srv)

	logger.GetLogger().Log(logger.Verbose, "We're out here in loopdriver 64")

	if logger.GetLogger().V(logger.Debug) {
		logger.GetLogger().Log(logger.Debug, "Hera loop driver driver, opening", url, ": ", cli)
	}
	if len(url) > 0 {
		// now set the shard ID
		fields := strings.Split(url, ":")
		if (len(fields) == 3) && (GetConfig().EnableSharding) {
			ns := nets.NewPacketFrom(common.CmdSetShardID, []byte(fields[0]))
			cli.Write(ns.Serialized)
			logger.GetLogger().Log(logger.Verbose, "HERA loop driver driver, fields", ns.Serialized)

			ns, err := nets.NewPacket(cli)
			if err != nil {
				return nil, fmt.Errorf("Failed to set shardID: %s", err.Error())
			}
			if ns.Cmd != common.RcOK {
				return nil, fmt.Errorf("HERA_SET_SHARD_ID response: %s", string(ns.Serialized))
			}
			if logger.GetLogger().V(logger.Debug) {
				logger.GetLogger().Log(logger.Debug, "HERA loop driver driver, opened to shard", fields[0])
			}
		}
	}
	return gosqldriver.NewHeraConnection(cli), nil
}
