package webank

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	adminUrl           = *flag.String("admin-url", getEnv("ADMIN_URL", "http://169.254.149.66:8080"), "WE-REDIS ADMIN URL OF WEBANK")
	assembleInfoPath   = *flag.String("assmeble-info-path", getEnv("ASSMEBLE_INFO_PATH", "/weredis/clusterinfo/v1/getAssembleInfo"), "assemble info path")
	CurrentClusterName = *flag.String("cluster-name", getEnv("CLUSTER_NAME", ""), "exporter cluster name")

	clusterInfo *ClusterInfo
	mu          sync.RWMutex
)

func getEnv(key string, defaultVal string) string {
	if envVal, ok := os.LookupEnv(key); ok {
		return envVal
	}
	return defaultVal
}

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 定时刷新当前维护的集群信息
			if CurrentClusterName == "" {
				log.Println("ticker refresh stop, beacuse current cluster name is nil")
				continue
			}
			updateCurrentClusterInfo(CurrentClusterName)
		}
	}()
}

// 获取当前exporter监测的集群信息
// 若传入集群名和当前监测的集群不同，则会自动切换
func GetCurrentClusterInfo(clusterName string) (*ClusterInfo, error) {
	if clusterName != CurrentClusterName {
		if err := updateCurrentClusterInfo(clusterName); err != nil {
			return nil, err
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	return clusterInfo, nil
}

func updateCurrentClusterInfo(clusterName string) error {
	CurrentClusterName = clusterName
	info, err := getAssembleInfo(clusterName)
	if err != nil {
		log.Printf("update cluster info err: %s\n", err)
		return errors.New("update cluster infor err:" + err.Error())
	}
	mu.Lock()
	defer mu.Unlock()
	clusterInfo = info
	return nil
}

func getAssembleInfo(clusterName string) (*ClusterInfo, error) {
	req, err := http.NewRequest("GET", adminUrl+assembleInfoPath, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("clusterName", clusterName)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("admin request fail")
	}

	result := &assembleResponse{}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, err
	} else if result.Code != "0" {
		return nil, errors.New("fail")
	}

	return &result.ResultData, nil
}

type assembleResponse struct {
	Code       string      `json:"code"`
	Msg        string      `json:"msg"`
	ResultData ClusterInfo `json:"resultData"`
	Page       *string     `json:"page"`
	Others     interface{} `json:"others"`
}

// RPD|RPD_GENERAL_REDIS_NODESET_1_CACHE|1|169.254.149.66:30001,169.254.149.66:30002,169.254.149.66:30003
func (c *ClusterInfo) UnmarshalJSON(data []byte) error {
	content := strings.Trim(string(data), "\"")
	// 处理每个分区的数据
	for _, s := range strings.Split(content, ";") {
		// 取num和host, 包含全部主从节点
		split := strings.Split(s, "|")
		if len(split) != 4 {
			continue
		}

		num := split[2]

		hosts := strings.Split(split[3], ",")
		nodes := make([]NodeInfo, len(hosts))
		for i, h := range hosts {
			nodes[i] = NodeInfo{
				Partition: num,
				Instance:  h,
			}
		}

		c.Name = CurrentClusterName
		c.Nodes = nodes
	}
	return nil
}
