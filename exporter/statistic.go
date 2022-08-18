package exporter

import (
	"fmt"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/mna/redisc"
	log "github.com/sirupsen/logrus"
)

type SysCapUsed struct {
	systemId string
	used     int
}

var (
	StatResult    = make(map[string][]SysCapUsed)
	lockKeyPrefix = "__exporter_subsystem_capacity_stat:"
)

// 遍历内存中维护的所有集群, 统计每个集群内各子系统的使用容量:
// 1. 对集群的最小分区加锁
// 2. 获取全部分区的所有master节点，dbsize累加，求采样总数
// 3. 获取所有slave的连接，randkey + object计算大小
// 4. 缓存统计结果并写入文件
func statSystemCapacity(rate float64) {
	options := []redis.DialOption{
		redis.DialConnectTimeout(2 * time.Second),
		redis.DialReadTimeout(2 * time.Second),
		redis.DialWriteTimeout(2 * time.Second),
		// TODO:
		redis.DialPassword("wb6Cluster"),
	}

	allClustersInfo.Range(func(key, value interface{}) bool {
		clusterInfo := value.(*ClusterInfo)
		if len(clusterInfo.Partitions) == 0 {
			log.Errorf("can not found partition for cluster:%s", clusterInfo.Name)
			return true
		}

		// 对集群最小分区加锁, 抢到锁的exporter才能执行统计任务
		partition := minPartition(clusterInfo.Partitions)
		c, err := connectToRedisCluster(partition, options)
		if err != nil {
			log.Errorf("can not connect partition:%s, err:%s", partition.Name, err)
			return true
		}
		defer c.Close()
		
		// lockKey上拼接日期, 每个集群每天只需执行一次统计, 锁自动过期 1 day
		lockKey := lockKeyPrefix + time.Now().Format("2006-01-02")
		if _, err := redis.String(c.Do("set", lockKey, "nx", "ex", "86400")); err != nil {
			log.Errorf("lock cluster:%s fail, err:%s", clusterInfo.Name, err)
			return true
		}

		mConns, sConns, err := getConns(clusterInfo.Partitions, options)
		if err != nil {
			log.Errorf("get cluster:%s conn fail, err:%s", clusterInfo.Name, err)
			return true
		}

		var total int
		for _, c := range mConns {
			size, err := redis.Int(c.Do("dbsize"))
			if err != nil {
				log.Errorf("exec dbsize fail,c:%v err:%s", c, err)
			}
			total += size
		}
		log.Printf("cluster:%s total dbsize:%d\n", clusterInfo.Name, total)

		for _, c := range sConns {
			log.Printf("con %s\n", c)
		}
		
		return true
	})
}

// 返回集群内编号最小的分区
func minPartition(partitions []PartitionInfo) PartitionInfo {
	min := partitions[0]
	for i := 1; i < len(partitions); i++ {
		p := partitions[i]
		if min.Num > p.Num {
			min = p
		}
	}
	return min
}


func connectToRedisCluster(partition PartitionInfo, options []redis.DialOption) (redis.Conn, error) {
	nodes := partition.Nodes
	if len(nodes) == 0 {
		return nil, fmt.Errorf("can not found node for partition:%s", partition.Name)
	}

	uri := nodes[0].Host
	if frags := strings.Split(uri, ":"); len(frags) != 2 {
		uri = uri + ":6379"
	}

	cluster := redisc.Cluster{
		StartupNodes: []string{uri},
		DialOptions:  options,
	}
	if err := cluster.Refresh(); err != nil {
		log.Errorf("Cluster refresh failed: %v", err)
	}

	conn, err := cluster.Dial()
	if err != nil {
		log.Errorf("Dial failed: %v", err)
	}

	c, err := redisc.RetryConn(conn, 10, 100*time.Millisecond)
	if err != nil {
		log.Errorf("RetryConn failed: %v", err)
	}

	return c, err
}


// 分类返回所有集群内所有主从节点的连接
func getConns(partitions []PartitionInfo, options []redis.DialOption) ([]redis.Conn, []redis.Conn, error) {	
	mConns := make([]redis.Conn, 0)
	sConns := make([]redis.Conn, 0)

	for _, p := range partitions {
		nodes := p.Nodes
		for _, n := range nodes {
			addr := n.Host
			if frags := strings.Split(addr, ":"); len(frags) != 2 {
				addr = addr + ":6379"
			}

			c, err := redis.Dial("tcp", addr, options...)
			if err != nil {
				log.Errorf("Dial redis:%s failed, err: %s", addr, err)
			}
			
			reply, err := redis.String(c.Do("info", "replication"))
			if err != nil {
				log.Printf("exec info cmd for node: %s, err: %s", addr, err)
			}
			
			if strings.Contains(reply, "role:master") {
				mConns = append(mConns, c)
			} else if strings.Contains(reply, "role:slave") {
				sConns = append(sConns, c)
			} else {
				log.Errorf("unknow role for node: %s", addr)
			}
		}
	}

	return mConns, sConns, nil
}