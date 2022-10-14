package exporter

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/mna/redisc"
	log "github.com/sirupsen/logrus"
)

const lockKeyPrefix = "__exporter_subsystem_capacity_stat:"

// 保存一个集群内所有子系统统计指标
// key: systemId
var StatResult map[string]*SysUsage

// 各项使用率统计
type SysUsage struct {
	capacity int // 内存容量

	bigKeyNum    int      // bigKey数量
	bigKeySample []string // bigKey样本

	ttlKeyNum    int
	ttlKeySample []string
}

// 遍历内存中维护的所有集群, 统计每个集群内各子系统的使用容量:
// 1. 对集群的最小分区加锁, 若加锁成功则表示该exporter对这个集群执行采样任务
// 2. 创建集群内所有master节点对应的其中一个slave节点的连接
// 3. 对每个节点dbsize累加，得到key总数, 再计算需采样的key总数
// 4. 使用randkey命令, 在所有slave节点中随机循环采样key
// 5. 汇总最后结果, 上报admin
func sampleStatClusterUsage() {
	options := []redis.DialOption{
		redis.DialConnectTimeout(2 * time.Second),
		redis.DialReadTimeout(2 * time.Second),
		redis.DialWriteTimeout(2 * time.Second),
		redis.DialPassword(RedisPassword),
	}

	// 只对exporter内已知的集群尝试进行采样扫描
	allClustersInfo.Range(func(_, info interface{}) bool {
		clusterInfo := info.(*ClusterInfo)
		if len(clusterInfo.Partitions) == 0 {
			log.Errorf("can not found partition for cluster:%s\n", clusterInfo.Name)
			return true
		}

		// 对集群最小分区加锁, 抢到锁的exporter才能执行统计任务
		partition := minPartition(clusterInfo.Partitions)
		clusterConn, err := connectToRedisCluster(partition, options)
		if err != nil {
			log.Errorf("can not connect partition:%s, err:%s\n", partition.Name, err)
			return true
		}
		defer clusterConn.Close()

		// lockKey上拼接日期, 每个集群每天只需执行一次统计, 锁自动过期 1 day (86400)
		lockKey := lockKeyPrefix + time.Now().Format("2006-01-02")
		if _, err := redis.String(clusterConn.Do("set", lockKey, "weredis-exporter-" + host, "nx", "ex", "86400")); err != nil {
			log.Errorf("lock cluster:%s fail, err:%s", clusterInfo.Name, err)
			return true
		}

		log.Infof("lock success, start scan cluster:%s\n", clusterInfo.Name)

		conns, err := createSlaveConnections(clusterInfo, options)
		if err != nil {
			log.Errorf("get cluster:%s conn fail, err:%s\n", clusterInfo.Name, err)
			return true
		}
		defer func() {
			log.Infof("close cluster:%s connection\n", clusterInfo.Name)
			for _, c := range conns {
				c.conn.Close()
			}
		}()

		connsNum := len(conns)
		log.Infof("connect cluster:%s slave num:%d\n", clusterInfo.Name, connsNum)

		// 集群内所有key的总数
		var totalKeyNum int
		for _, c := range conns {
			size, err := redis.Int(c.conn.Do("dbsize"))
			if err != nil {
				log.Errorf("exec dbsize fail,c:%v err:%s\n", c, err)
			} else {
				totalKeyNum += size
			}
		}
		sampleKeyNum := int(float64(totalKeyNum) * sampleRate)
		log.Infof("cluster:%s total key num:%d, sample num:%d\n", clusterInfo.Name, totalKeyNum, sampleKeyNum)

		StatResult = make(map[string]*SysUsage)

		// 扫描主逻辑
		// 1. 随机获取一个连接, 用其对应游标进行scan
		// 2. 判断bigKey, ttlKey
		// 3. 获取对应value大小, 并归属某个子系统
		for i := 0; i < sampleKeyNum; {
			c := conns[rand.Intn(connsNum)]
			res, err := redis.Values(c.conn.Do("scan", c.cursor, "count", scanCount))
			if err != nil {
				log.Errorf("exec scan fail, node:%s err:%s\n", c.addr, err)
				c.cursor = rand.Intn(10000)
				continue
			}

			if len(res) != 2 {
				log.Errorf("scan result format is illegal, %+v\n", res)
				continue
			}

			nextCur, err := strconv.Atoi(string(res[0].([]uint8)))
			if err != nil {
				log.Errorf("parse next curosr err: %s\n", err)
				continue
			}

			// 记录下一次该连接的scan游标, 更具有随机性
			c.cursor = nextCur + rand.Intn(10)

			// 遍历scan到的所有key
			for _, key := range res[1].([]interface{}) {
				k := string(key.([]uint8))
				var systemId string
				if len(k) > 4 {
					systemId = k[:4]
				} else {
					systemId = "unknown"
				}

				sysUsage := StatResult[systemId]
				if sysUsage == nil {
					sysUsage = &SysUsage{}
				}

				// 统计使用容量
				usage, err := redis.Int(c.conn.Do("memory", "usage", k, "sample", 0))
				if err != nil {
					log.Errorf("exec [memory usage] on key:[%s], err:%s\n", k, err)
					continue
				}

				// 判断是否设置ttl
				ttl, err := redis.Int(c.conn.Do("ttl", k))
				if err != nil {
					log.Errorf("exec [ttl] on key:[%s], err:%s\n", k, err)
					continue
				}

				// 判断是否属于bigKey
				isBigKey, err := isBigKey(k, usage, c.conn)
				if err != nil {
					log.Errorf("exec [Big Key] on key:[%s], err:%s\n", k, err)
					continue
				}

				// 记录结果
				// TODO: 限制个数
				sysUsage.capacity += usage
				if ttl == -1 || ttl > *ttlThreshold {
					sysUsage.ttlKeyNum += 1
					sysUsage.ttlKeySample = append(sysUsage.ttlKeySample, k)
				}
				if isBigKey {
					sysUsage.bigKeyNum += 1
					sysUsage.bigKeySample = append(sysUsage.bigKeySample, k)
				}

				// 记录扫描到key的个数
				i += 1
			}
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

type slaveConn struct {
	conn       redis.Conn
	cursor     int    // 随机游标
	addr       string // 连接addr
	masterAddr string // 对应master的addr
}

// 获取集群内各master对应其中一个slave节点的连接
func createSlaveConnections(clusterInfo *ClusterInfo, options []redis.DialOption) ([]slaveConn, error) {
	conns := make([]slaveConn, 0, len(clusterInfo.Partitions)*3)

	// 保存已经连上其slave节点的master
	connectedNode := make([]string, 0, len(clusterInfo.Partitions))

	for _, p := range clusterInfo.Partitions {
		for _, n := range p.Nodes {
			addr := n.Host
			if frags := strings.Split(addr, ":"); len(frags) != 2 {
				addr = addr + ":6379"
			}

			c, err := redis.Dial("tcp", addr, options...)
			if err != nil {
				log.Errorf("Dial redis:%s failed, err: %s", addr, err)
				continue
			}

			reply, err := redis.String(c.Do("info", "replication"))
			if err != nil {
				log.Printf("exec info cmd for node: %s, err: %s", addr, err)
				continue
			}

			var masterHost, masterPort string
			var isSlave bool

			lines := strings.Split(reply, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) <= 2 || strings.HasPrefix(line, "# ") || !strings.Contains(line, ":") {
					continue
				}

				split := strings.SplitN(line, ":", 2)
				fieldKey := split[0]
				fieldValue := split[1]

				if fieldKey == "role" {
					if fieldValue == "slave" {
						isSlave = true
					} else {
						break
					}
				}

				if fieldKey == "master_host" {
					masterHost = fieldValue
				}

				if fieldKey == "master_port" {
					masterPort = fieldValue
				}
			}

			masterAddr := masterHost + ":" + masterPort
			if isSlave && !Contains(connectedNode, masterAddr) {
				conns = append(conns, slaveConn{
					conn:       c,
					cursor:     rand.Intn(10000),
					addr:       addr,
					masterAddr: masterAddr,
				})
				connectedNode = append(connectedNode, masterAddr)
			} else {
				c.Close()
			}

		}
	}
	return conns, nil
}

// 判断是否属于BigKey
func isBigKey(key string, usage int, c redis.Conn) (bool, error) {
	if usage > *memoryThreshold {
		return true, nil
	}

	s, err := redis.String(c.Do("type", key))
	if err != nil {
		log.Errorf("exec [type] on key:%s, err%s\n", key, err)
		return false, err
	}

	var itemNum int
	switch s {
		case "string", "exstrtype": itemNum, err = 0, nil
		case "list": itemNum, err = redis.Int(c.Do("llen", key))
		case "set": itemNum, err = redis.Int(c.Do("scard", key))
		case "hash": itemNum, err = redis.Int(c.Do("hlen", key))
		case "zset": itemNum, err = redis.Int(c.Do("zcard", key)) 
		case "tairhash-": itemNum, err = redis.Int(c.Do("exhlen", key, "noExp"))
		default: itemNum, err = 0, errors.New("unknown type:" + s)
	}

	if err != nil {
		log.Errorf("fetch item num on key:%s err:%s\n", key, err)
		return false, err;
	}

	return itemNum > *itemsThreshold, nil
}
