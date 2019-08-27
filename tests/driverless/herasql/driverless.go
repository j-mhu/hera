package main

import lib "github.com/paypal/hera/lib"
import "fmt"

func main() {
	fmt.Println("Initializing configuration")
	err := lib.InitConfig()
	if err != nil {
		fmt.Println(err.Error())
	}

	lib.GetConfig()
//	fmt.Println(config)
	
	fmt.Println("Creating new tcplistener")
	lsn := lib.NewTCPListener(fmt.Sprintf("0.0.0.0:%d", 3333))
	
	fmt.Println("Creating new mux server")
	srv := lib.NewServer(lsn, lib.HandleConnection)
	
	fmt.Println("Running server")
	srv.Run()
}
