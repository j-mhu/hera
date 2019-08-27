#!/bin/bash

## This script runs the unit test / basic_coordinator on the dummy server.
# Dummy server is at localhost:3333 via tcp.

clear 

# Set environment variables
export DB_USER=x
export DB_PASSWORD=x
export DB_DATASOURCE=x
export username=realU
export password=realU-pwd
export TWO_TASK='tcp(localhost:3306)/'
export TWO_TASK_READ='tcp(localhost:3306)/'

# Run the test specified by variable n
$GOROOT/bin/go install github.com/paypal/hera/{mux,worker/mysqlworker}

cd $GOPATH/src/github.com/paypal/hera/tests/driverless/herasql
go run driverless.go
