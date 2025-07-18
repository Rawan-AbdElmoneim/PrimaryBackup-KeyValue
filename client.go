package kvservice

import (
	"fmt"
	"net/rpc"
	"sysmonitor"
	"time"
)

type KVClient struct {
	monitorClnt *sysmonitor.Client
	view        sysmonitor.View
	id          string
	//
	  seqNo     int64
}

func MakeKVClient(monitorServer string) *KVClient {
	client := new(KVClient)
	client.monitorClnt = sysmonitor.MakeClient("", monitorServer)
	client.view = sysmonitor.View{}
	//
	client.id = fmt.Sprintf("%d", nrand())
	return client
}

func call(srv string, rpcname string, args interface{}, reply interface{}) bool {
	c, errx := rpc.Dial("unix", srv)
	if errx != nil {
		return false
	}
	defer c.Close()

	err := c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func (client *KVClient) updateView() {
	view, _ := client.monitorClnt.Get()
	client.view = view
}

func (client *KVClient) Get(key string) string {
	args := &GetArgs{
		Key:   key,
		//
		CID:   client.id,
		SeqNo: client.seqNo,
	}
	client.seqNo++

	timeout := time.After(30 * time.Second)

	ticker := time.NewTicker(sysmonitor.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return ""
		case <-ticker.C:
			client.updateView()

			if client.view.Primary =="" {

				continue
			}
			var reply GetReply
			valid := call(client.view.Primary,"KVServer.Get",args,&reply)
			if valid && reply.Err == OK {

				return reply.Value
			}
		}
	}
}

func (client *KVClient) PutAux(key string, value string, dohash bool) string {
	args := &PutArgs{
		Key:    key,
		Value:  value,
		DoHash: dohash,
		//
		CID:    client.id,
		SeqNo:  client.seqNo,
	}
	client.seqNo++

	timeout :=time.After(30 * time.Second)
	ticker :=time.NewTicker(sysmonitor.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return ""
		case <-ticker.C:
			client.updateView()
			if client.view.Primary == "" {

				continue
			}

			var reply PutReply
			valid := call(client.view.Primary, "KVServer.Put", args, &reply)
			if valid {
				if reply.Err == OK {

					return reply.PreviousValue

				} else if reply.Err == ErrWrongServer {
					continue
				}
			}
		}
	}
}

func (client *KVClient) Put(key string, value string) {
	client.PutAux(key, value, false)
}

func (client *KVClient) PutHash(key string, value string) string {
	return client.PutAux(key, value, true)
}