
package kvservice

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"syscall"
	"sysmonitor"
	"time"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		n, err = fmt.Printf(format, a...)
	}
	return
}

type KVServer struct {
	l           net.Listener
	dead        bool
	unreliable  bool
	id          string
	monitorClnt *sysmonitor.Client
	view        sysmonitor.View
	done        sync.WaitGroup
	finish      chan interface{}
	///////
	data        map[string]string

	mu          sync.Mutex
	primaryRole   bool
	backupRole    bool

	 primaryAddr string
	
	
	duplicates map[string]map[int64]string
}



func (server *KVServer) BackupSync(args *StateSyncArgs, reply *StateSyncReply) error {
    server.mu.Lock()
    defer server.mu.Unlock()

    switch {
     
        
    case !server.backupRole:
        reply.Err =ErrWrongServer
    
    default:
     
        clonedData :=make(map[string]string)
        for key, val := range args.Data {
            clonedData[key] =val

        }

        
        clonedOps :=make(map[string]map[int64]string)
        for client, ops := range args.Duplicates {

            clientOps := make(map[int64]string, len(ops))

            for seq, val := range ops {
                clientOps[seq] =val
            }
            clonedOps[client] =clientOps
        }
        server.data =clonedData
        server.duplicates =clonedOps
		
        reply.Err = OK
    }

    return nil
}

func (server *KVServer) PrimaryToBackupGet(args *KVReplicaGetArgs, reply *GetReply) error {
	return server.Get(&args.GetArgs, reply)
}
func (server *KVServer) Get(args *GetArgs, reply *GetReply) error {
	server.mu.Lock()

	defer server.mu.Unlock()

	if !server.primaryRole && !server.backupRole {
		reply.Err =ErrWrongServer

		return nil
	}

	
	if server.backupRole {
		reply.Err =ErrWrongServer
		return nil
	}

	
	if server.primaryRole &&server.view.Backup !="" {
		fargs := &KVReplicaGetArgs{GetArgs: *args}
		var freply GetReply
		if call(server.view.Backup,"KVServer.PrimaryToBackupGet",fargs,&freply) {

			if freply.Err !=OK {
				
				newView, _ := server.monitorClnt.Get()
				if newView.Primary !=server.id {
					reply.Err = ErrWrongServer
					return nil
				}
			}
		}
	}

	value, exists :=server.data[args.Key]
	if exists {
		reply.Value =value
	} else {
		reply.Value =""

	}
	reply.Err =OK
	return nil
}



func (server *KVServer) Put(args *PutArgs, reply *PutReply) error {
    server.mu.Lock()
    defer server.mu.Unlock()

    if !server.primaryRole {
        reply.Err = ErrWrongServer
        return nil
    }
    if server.duplicates ==nil {

        server.duplicates =make(map[string]map[int64]string)
    }
    _, exists :=server.duplicates[args.CID]
    if !exists {

        server.duplicates[args.CID] = make(map[int64]string)
    }
	prevValue, exists :=server.duplicates[args.CID][args.SeqNo]
    if exists {
    if args.DoHash {
        reply.PreviousValue =prevValue
    }
    reply.Err =OK
    return nil
    }
    oldValue :=server.data[args.Key]

    var newValue string

    if args.DoHash {
        combined :=oldValue +args.Value
        newValue =fmt.Sprintf("%d", hash(combined))


        reply.PreviousValue =oldValue
        server.duplicates[args.CID][args.SeqNo] =oldValue
    } else {
        newValue = args.Value
        server.duplicates[args.CID][args.SeqNo] =""
    }
    server.data[args.Key] =newValue

    if server.primaryRole && server.view.Backup !="" {
        go func() {
            fargs := &KVReplicaPutArgs{
                PutArgs: PutArgs{
                    Key:    args.Key,
                    Value:    newValue,
                    DoHash: false,
					//
                    CID:     args.CID,
                    SeqNo:  args.SeqNo,
                },
            }
            var freply PutReply
            for i:=0; i < 3;i++ {
                if call(server.view.Backup,"KVServer.PrimaryToBackupPut",fargs,&freply) {
                    break
                }
                time.Sleep(sysmonitor.PingInterval)
            }
        }()
    }
    reply.Err = OK
    return nil
}

func (server *KVServer) PrimaryToBackupPut(args *KVReplicaPutArgs, reply *PutReply) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if !server.backupRole {
		reply.Err =ErrWrongServer
		return nil
	}

	if server.duplicates ==nil {
		server.duplicates =make(map[string]map[int64]string)
	}




	_, exists := server.duplicates[args.CID]
	if !exists {
		server.duplicates[args.CID] =make(map[int64]string)
	}
	_, exists = server.duplicates[args.CID][args.SeqNo]
	if exists {

		reply.Err = OK
		return nil
	}
	server.data[args.Key] = args.Value
	server.duplicates[args.CID][args.SeqNo] ="" 

	reply.Err =OK

	return nil
}

func (server *KVServer) tick() {
	view, err := server.monitorClnt.Ping(server.view.Viewnum)
	if err != nil {
		if server.primaryRole {
			server.mu.Lock()

			server.primaryRole =false
			server.mu.Unlock()
		}
		return
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	oldPrimary := server.primaryRole
	oldViewNum := server.view.Viewnum
	server.view = view
	server.primaryRole = (view.Primary ==server.id)

	server.backupRole = (view.Backup == server.id)

	
	if server.primaryRole && view.Backup !="" && (!oldPrimary ||view.Viewnum !=oldViewNum) {
		go func() {
			for i:=0; i< 3;i++ { 
				server.mu.Lock()

				dataCopy :=make(map[string]string)
				for key,val :=range server.data {

					dataCopy[key] = val
				}
				
				dupCopy :=make(map[string]map[int64]string)
				for cid, seqs :=range server.duplicates {

					dupCopy[cid] =make(map[int64]string)
					for seq, val := range seqs {

						dupCopy[cid][seq] = val
					}
				}
				server.mu.Unlock()
				args := &StateSyncArgs{Data: dataCopy, Duplicates: dupCopy}
				var reply StateSyncReply
				if call(view.Backup,"KVServer.BackupSync",args,&reply) && reply.Err == OK {
					server.mu.Lock()

					server.primaryAddr = view.Backup
					server.mu.Unlock()
					break
				}
				time.Sleep(sysmonitor.PingInterval)
			}
		}()
	}

	if server.primaryRole && view.Backup != "" {
		server.primaryAddr = view.Backup
	} else if server.backupRole && view.Primary != "" {

		server.primaryAddr = view.Primary
	} else {

		server.primaryAddr = ""
	}
}
func (server *KVServer) Kill() {
	server.dead = true
	server.l.Close()
}

func StartKVServer(monitorServer string, id string) *KVServer {
	server := new(KVServer)
	server.id = id
	server.monitorClnt = sysmonitor.MakeClient(id, monitorServer)
	server.view = sysmonitor.View{}
	server.finish = make(chan interface{})
	server.data = make(map[string]string)

	
	server.duplicates = make(map[string]map[int64]string)
	rpcs := rpc.NewServer()

	rpcs.Register(server)

	os.Remove(server.id)
	l, e := net.Listen("unix", server.id)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	server.l = l

	go func() {
		for server.dead == false {
			conn, err := server.l.Accept()
			if err == nil && server.dead == false {
				if server.unreliable && (rand.Int63()%1000) < 100 {
					conn.Close()
				} else if server.unreliable && (rand.Int63()%1000) < 200 {
					c1 := conn.(*net.UnixConn)
					f, _ := c1.File()
					err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
					if err != nil {
						fmt.Printf("shutdown: %v\n", err)
					}
					server.done.Add(1)
					go func() {
						rpcs.ServeConn(conn)
						server.done.Done()
					}()
				} else {
					server.done.Add(1)
					go func() {
						rpcs.ServeConn(conn)
						server.done.Done()
					}()
				}
			} else if err == nil {
				conn.Close()
			}
			if err != nil && server.dead == false {
				fmt.Printf("KVServer(%v) accept: %v\n", id, err.Error())
				server.Kill()
			}
		}
		DPrintf("%s: wait until all request are done\n", server.id)
		server.done.Wait()
		close(server.finish)
	}()

	server.done.Add(1)
	go func() {
		for server.dead == false {
			server.tick()
			time.Sleep(sysmonitor.PingInterval)
		}
		server.done.Done()
	}()

	return server
}