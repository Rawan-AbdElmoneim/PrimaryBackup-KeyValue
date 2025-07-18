package kvservice

import (
	"crypto/rand"
	"hash/fnv"
	"math/big"
)

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongServer = "ErrWrongServer"
)

type Err string

type PutArgs struct {
	Key    string
	Value  string
	DoHash bool
	//
	 CID   string
	SeqNo  int64
}

type PutReply struct {
	Err           Err
	PreviousValue string
}

type GetArgs struct {
	Key   string
	//
	 CID   string
	SeqNo int64
}

type GetReply struct {
	Err   Err
	Value string
}

type StateSyncArgs struct {
	Data map[string]string
	
	Duplicates map[string]map[int64]string
}

type StateSyncReply struct {
	Err Err
}



type KVReplicaGetArgs struct {

	GetArgs
}
type KVReplicaPutArgs struct {
	PutArgs

}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}