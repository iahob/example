package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

var (
	serverName = flag.String("name", "B", "")
)

func main() {
	flag.Parse()

	// Etcd 服务器地址
	endpoints := []string{"127.0.0.1:2379"}
	clientConfig := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 2 * time.Second,
	}
	cli, err := clientv3.New(clientConfig)
	if err != nil {
		panic(err)
	}

	s1, err := concurrency.NewSession(cli)
	if err != nil {
		panic(err)
	}
	fmt.Println("session lessId is ", s1.Lease())

	e1 := concurrency.NewElection(s1, "my-election")
	go func() {
		// 参与选举，如果选举成功，会定时续期
		if err := e1.Campaign(context.Background(), *serverName); err != nil {
			fmt.Println(err)
		}
		fmt.Println("Campaign return")

	}()

	masterName := ""
	go func() {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		timer := time.NewTicker(time.Second)
		for range timer.C {
			timer.Reset(time.Second)
			select {
			case resp := <-e1.Observe(ctx):
				if len(resp.Kvs) > 0 {
					// 查看当前谁是 master
					masterName = string(resp.Kvs[0].Value)
					fmt.Println("get master with:", masterName)
				}
			}
		}
	}()

	go func() {
		timer := time.NewTicker(5 * time.Second)
		for range timer.C {
			// 判断自己是 master 还是 slave
			if masterName == *serverName {
				fmt.Println("oh, i'm master")
			} else {
				fmt.Println("slave!")
			}
		}
	}()

	c := make(chan os.Signal, 1)
	// 接收 Ctrl C 中断
	signal.Notify(c, os.Interrupt, os.Kill)

	s := <-c
	fmt.Println("Got signal:", s)
	err = e1.Resign(context.TODO())
	if err != nil {
		return
	}
}
