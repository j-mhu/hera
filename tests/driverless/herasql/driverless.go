package main

import (
	"flag"
	"github.com/paypal/hera/cal"
	lib "github.com/paypal/hera/lib"
	"github.com/paypal/hera/utility/logger"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)
import "fmt"

func main() {
	// os.Args = []string{"$GOPATH/src/github.com/paypal/hera/tests/driverless/herasql/"}
	os.Args = []string{" "}
	os.Args = append(os.Args, "--name", "hera-test")

	for _, v := range os.Args {
		fmt.Println(v)
	}

	signal.Ignore(syscall.SIGPIPE)
	namePtr := flag.String("name", "", "module name in v$session table")
	flag.Parse()

	/* Don't log.
	We haven't configured log level, so lots goes to stdout/err log. */
	if len(*namePtr) == 0 {
		if logger.GetLogger().V(logger.Alert) {
			logger.GetLogger().Log(logger.Alert, "missing --name parameter")
		}
		lib.FullShutdown()
	}

	rand.Seed(time.Now().Unix())

	err := lib.InitConfig()
	if err != nil {
		if logger.GetLogger().V(logger.Alert) {
			logger.GetLogger().Log(logger.Alert, "failed to initialize configuration:", err.Error())
		}
		lib.FullShutdown()
	}

	pidfile, err := os.Create(lib.GetConfig().MuxPidFile)
	if err != nil {
		if logger.GetLogger().V(logger.Alert) {
			logger.GetLogger().Log(logger.Alert, "Can't open", lib.GetConfig().MuxPidFile, err.Error())
		}
		lib.FullShutdown()
	} else {
		pidfile.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	}

	os.Setenv("MUX_START_TIME_SEC", fmt.Sprintf("%d", time.Now().Unix()))
	os.Setenv("MUX_START_TIME_USEC", "0")

	//
	// worker also initialize a calclent with the same poolname using threadid==0 in
	// its bootstrap label message. if we let worker fire off its msg first, all proxy
	// messages will end up in the same swimminglane since that is what id(0) does.
	// so, let's send the bootstrap label message from proxy first using threadid==1.
	// that way, calmsgs with different threadids can end up in different swimminglanes,
	//
	caltxn := cal.NewCalTransaction(cal.TransTypeAPI, "mux-go", cal.TransOK, "", cal.DefaultTGName)
	caltxn.SetCorrelationID("abc")
	calclient := cal.GetCalClientInstance()
	if calclient != nil {
		release := calclient.GetReleaseBuildNum()
		if release != "" {
			evt := cal.NewCalEvent("VERSION", release, "0", "")
			evt.Completed()
		}
	}
	caltxn.Completed()

	//
	// create singleton broker and start worker/pools
	//
	if (lib.GetWorkerBrokerInstance() == nil) || (lib.GetWorkerBrokerInstance().RestartWorkerPool(*namePtr) != nil) {
		if logger.GetLogger().V(logger.Alert) {
			logger.GetLogger().Log(logger.Alert, "failed to start hera worker")
		}
		lib.FullShutdown()
	}

	caltxn = cal.NewCalTransaction(cal.TransTypeAPI, "mux-go-start", cal.TransOK, "", cal.DefaultTGName)
	caltxn.SetCorrelationID("runtxn")
	caltxn.Completed()

	lib.GetStateLog().SetStartTime(time.Now())

	go func() {
		sleep := time.Duration(lib.GetConfig().ConfigReloadTimeMs)
		for {
			time.Sleep(time.Millisecond * sleep)
			lib.CheckOpsConfigChange()
		}
	}()

	lib.CheckEnableProfiling()
	lib.GoStats()

	pool, err := lib.GetWorkerBrokerInstance().GetWorkerPool(lib.HeraWorkerType(0), 0, 0)
	// lib.HeraWorkerType(0) is wTypeRW
	if err != nil {
		if logger.GetLogger().V(logger.Alert) {
			logger.GetLogger().Log(logger.Alert, "failed to get pool WTYPE_RW, 0, 0:", err)
		}
		lib.FullShutdown()
	}

	for {
		if pool.GetHealthyWorkersCount() > 0 {
			break
		} else {
			if lib.GetConfig().EnableTAF {
				fallbackPool, err := lib.GetWorkerBrokerInstance().GetWorkerPool(lib.HeraWorkerType(2), 0, 0)
				// lib.HeraWorkerType(2) is wtypeStdBy
				if (err == nil) && (fallbackPool.GetHealthyWorkersCount() > 0) {
					break
				}
			}
		}
		time.Sleep(time.Millisecond * 100)
	}

	fmt.Println("Creating new tcplistener")
	lsn := lib.NewTCPListener(fmt.Sprintf("0.0.0.0:%d", 3333))

	if lib.GetConfig().EnableSharding {
		err = lib.InitShardingCfg()
		if err != nil {
			if logger.GetLogger().V(logger.Alert) {
				logger.GetLogger().Log(logger.Alert, "failed to initialize sharding config:", err)
			}
			lib.FullShutdown()
		}
	}

	lib.InitRacMaint(*namePtr)

	fmt.Println("Creating new mux server")
	srv := lib.NewServer(lsn, lib.HandleConnection)

	fmt.Println("Running server")
	srv.Run()
}
