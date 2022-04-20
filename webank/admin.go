package webank

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// TODO: 配置文件指定
	adminUrl         = "http://127.0.0.1:8080"
	assembleInfoPath = "/weredis/clusterinfo/v1/getAssembleInfo"
	CurrentClusterName = "ddddd"

	clusterInfo *ClusterInfo
	mu          sync.RWMutex
)

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			// 定时刷新当前维护的集群信息
			updateCurrentClusterInfo(CurrentClusterName)
		}

		/* for {
			select {
			case <-ticker.C:		
			}
		} */
	}()
}

// 获取当前exporter监测的集群信息
// 若传入集群名和当前监测的集群不同，则会自动切换
func GetCurrentClusterInfo(clusterName string) ClusterInfo {
	if clusterName != CurrentClusterName {
		updateCurrentClusterInfo(clusterName)
	}
	mu.RLock()
	defer mu.RUnlock()
	return *clusterInfo
}

func updateCurrentClusterInfo(clusterName string) {
	if clusterName == "" {
		log.Println("cluster name is mempty")
		return
	}
	CurrentClusterName = clusterName
	info, err := getAssembleInfo(clusterName)
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	mu.Lock()
	defer mu.Unlock()
	clusterInfo = info
}

func getAssembleInfo(clusterName string) (*ClusterInfo, error) {
	// TODO: http是不是长连接
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
