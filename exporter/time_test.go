package exporter

import (
	"log"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)


func TestTime(t *testing.T) {
	 options := []redis.DialOption{
		redis.DialConnectTimeout(2 * time.Second),
		redis.DialReadTimeout(2 * time.Second),
		redis.DialWriteTimeout(2 * time.Second),
		redis.DialPassword("wb6Cluster"),
	} 

	p := make([]PartitionInfo, 0)
	p = append(p, PartitionInfo{
		Name: "p1",
		Num: "1",
		Nodes: []NodeInfo{
			NodeInfo{Host: "169.254.149.66:30001"},
			NodeInfo{Host: "169.254.149.66:30002"},
			NodeInfo{Host: "169.254.149.66:30003"},
		},
	})

	cluster := ClusterInfo{
		Name: "ispd",
		Partitions: p,
	}

	allClustersInfo.Store("c1", &cluster)
	
	ms, ss, _ := getConns(p, options)
	log.Printf("mCon %d, sCon %d\n", len(ms), len(ss))

	statSystemCapacity(0.1)
}