package exporter

import (
	"sync"

	"math/rand"
	"github.com/gomodule/redigo/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type ClusterExporter struct {
	*Exporter

	// 采集的集群名
	clusterName string

	// 维护集群内所有分片的拓扑信息
	clusterTopology sync.Map
}

func NewClusterExporter(e *Exporter, name string, opts Options) *ClusterExporter {
	ce := &ClusterExporter{
		Exporter: e,
		clusterName: name,
	}

	opts.Registry.MustRegister(ce)
	return ce
}

// 开始采集前的hook
func (e *ClusterExporter) Describe(ch chan<- *prometheus.Desc) {
}

// 采集结束后的hook
func (e *ClusterExporter) Collect(ch chan<- prometheus.Metric) {
	opt := e.options
	if err := e.scrapeRedisCluster(ch); err != nil {
		log.Warnf("scrape partition:%s for cluster nodes info error:%s\n", opt.Partition, err.Error())
	}
	// 触发上报信号, 传递集群拓扑信息
	admCh <- e
}

func (e *ClusterExporter) scrapeRedisCluster(ch chan<- prometheus.Metric) error {
	c, err := e.connectToRedis()
	if err != nil {
		log.Errorf("Couldn't connect to redis instance for scrape partition")
		log.Debugf("connectToRedis( %s ) err: %s", e.redisAddr, err)
		return err
	}
	defer c.Close()

	if nodes, err := redis.String(doRedisCmd(c, "CLUSTER", "NODES")); err == nil {
		e.clusterTopology.Store(e.options.PartitionName, nodes)
		e.extractClusterNodesMetrics(ch, nodes)
	}
	return nil
}


// ClusterInfo 表示一个weredis集群，维护该集群下所有redis实例
type ClusterInfo struct {
	Name       string
	Partitions []PartitionInfo
}

type PartitionInfo struct {
	Name  string
	Num   string
	Nodes []NodeInfo
}

// NodeInfo 表示一个redis实例, 包含对于ip和port, 还有其归属分区
type NodeInfo struct {
	PartitionNum string
	PartitionName string
	Host         string
}

// GetNodes 获取集群的所有node
func (c *ClusterInfo) GetNodes() []NodeInfo {
	n := []NodeInfo{}
	for _, p := range c.Partitions {
		n = append(n, p.Nodes...)
	}
	return n
}

// PickNodeForEachPartition 随机挑选每个分区中的一个节点
func (c *ClusterInfo) PickNodeForEachPartition() []NodeInfo {
	n := []NodeInfo{}
	for _, p := range c.Partitions {
		idx := rand.Intn(len(p.Nodes))
		n = append(n, p.Nodes[idx])
	}
	return n
}

// 节点的状态信息, 用于上报
type NodeTopo struct {
	Id 			 string `json:"id"`
	Ip			 string `json:"ip"`
	Port		 int	`json:"port"`
	Role		 string	`json:"role"`
	Status		 string `json:"status"`
	SlaveOf      string `json:"slaveOf"`
}