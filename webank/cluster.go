package webank

import "math/rand"

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

type ClusterTopo struct {
	Id 			 string `json:"id"`
	Ip			 string `json:"ip"`
	Port		 int	`json:"port"`
	Role		 string	`json:"role"`
	Status		 string `json:"status"`
	SlaveOf      string `json:"slaveOf"`
}
