package main

import (
     _ "github.com/go-sql-driver/mysql"
     "os"
)
import "testing"
import "database/sql"

/* This sends very simple queries to Hera server that don't return
* any result sets.
*/
func TestExec(t *testing.T) {

     os.Symlink(os.Getenv("GOPATH")+"/bin/mysqlworker", "mysqlworker")

     t.Log("Start TestExec+++++++++++++")
     DSN := "tcp(0.0.0.0:3333)/"
     // Open database connection
     t.Log("Opening up database connection")
     db, err := sql.Open("mysql", DSN)
     if err != nil {
          t.Log(err.Error())
     }

     defer db.Close()

     res, err := db.Exec("DROP TABLE IF EXISTS test;");
     if err != nil {
          t.Log(err.Error())
     }
     t.Log("Successful execution!")

     liid, err := res.LastInsertId()
     if err != nil {
          t.Log(err.Error())
     }
     t.Log("Liid = ", liid)

     ra, err := res.RowsAffected()
     if err != nil {
          t.Log(err.Error())
     }
     t.Log("Rows affected = ", ra)
     t.Log("End TestExec+++++++++++++++")
}
