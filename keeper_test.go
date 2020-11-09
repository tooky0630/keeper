package keeper

import (
    "fmt"
    "testing"
)

func TestContainer_Register(t *testing.T) {
    c := New()
    if err := c.Register(new(HelloSrv), Name("helloService")); err != nil {
       t.Fatal(err)
    }
    ctl := new(HelloCtl)
    if err := c.Register(ctl, Name("helloCtl")); err != nil {
        t.Fatal(err)
    }
    t.Log(ctl.Hello())
}

type HelloSrv struct {
    word string
}

func (srv *HelloSrv) Hello() string {
    fmt.Println("pass HelloSrv...")
    return "Hello World " + srv.word
}


type HelloCtl struct {
    helloSrv HelloSrv `name:"helloService"`
}

func (ctl *HelloCtl) Hello() string {
    fmt.Println("pass HelloCtl...")
    return ctl.helloSrv.Hello()
}
