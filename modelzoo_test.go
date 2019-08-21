package modelzoo

import (
	"fmt"
	"testing"
	"time"
)

var path = ""
var endpoint = []string{"127.0.0.1"}
var nodename = "AB"
var tcppoint = "127.0.0.1:18526"
var tcppoint2 = "127.0.0.1:18525"

func TestZkConn(t *testing.T) {
	z, err := ZkConn(path, endpoint, nodename)
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	z, err = z.Zkget()
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	err = z.aliveNodeServices()
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	err = z.AliveNodeAddOrUpdate(tcppoint)
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	watchAlive(z.careNode)
	z.conn.Close()
	fmt.Println("Please creat new222")
	ch := make(chan bool, 1)
	go func() {
		time.Sleep(time.Second * 20)
		ch <- true
	}()
	for {
		if msg := <-ch; msg {
			break
		}
	}
	z2, err := ZkConn(path, endpoint, nodename)
	defer z2.conn.Close()
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	z2, err = z2.Zkget()
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	err = z2.aliveNodeServices()
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	err = z2.AliveNodeAddOrUpdate(tcppoint2)
	if err != nil {
		t.Errorf("fail : %s", err)
	}
	watchAlive(z2.careNode)
	t.Logf("success!")
	fmt.Println("Please creat new")
	go func() {
		time.Sleep(time.Second * 50)
		ch <- true
	}()
	for {
		if msg := <-ch; msg {
			break
		}
	}
	fmt.Println("Please close the net")
	go func() {
		time.Sleep(time.Second * 30)
		ch <- true
	}()
	for {
		if msg := <-ch; msg {
			break
		}
	}
	//z.watchAlive(z.careNode)
}
