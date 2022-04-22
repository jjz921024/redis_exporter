package webank

// ClusterInfo 表示一个weredis集群，维护该集群下所有redis实例
type ClusterInfo struct {
	Name string
	Nodes []NodeInfo
}

// NodeInfo 表示一个redis实例, 包含对于ip和port, 还有其归属分区
type NodeInfo struct {
	Partition string
	Host	  string
}

