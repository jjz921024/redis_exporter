package webank

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	adminUrl           = flag.String("admin-url", getEnv("ADMIN_URL", "http://10.107.120.69:19999"), "WE-REDIS ADMIN URL OF WEBANK")
	assembleInfoPath   = flag.String("assmeble-info-path", getEnv("ASSMEBLE_INFO_PATH", "/weredis/clusterinfo/v1/getAssembleInfo"), "assemble info path")
	reportTopoPath     = flag.String("report-topo-path", getEnv("REPORT_TOPO_PATH", "/weredis/cluster/node/status/v1/"), "report cluster topology path")
	enableReport	   = flag.Bool("enable-report-admin", getEnvBool("EANBLE_REPORT_ADMIN", true), "enable report admin")
	CurrentClusterName = flag.String("cluster-name", getEnv("CLUSTER_NAME", ""), "exporter cluster name")

	clusterInfo *ClusterInfo
	mu          sync.RWMutex

	AdmCh = make(chan interface{})
	ClusterTopology map[string]string = make(map[string]string)

	client *http.Client
)

func getEnv(key string, defaultVal string) string {
	if envVal, ok := os.LookupEnv(key); ok {
		return envVal
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if envVal, ok := os.LookupEnv(key); ok {
		envBool, err := strconv.ParseBool(envVal)
		if err == nil {
			return envBool
		}
	}
	return defaultVal
}

func init() {
	tr := &http.Transport{
		MaxIdleConns: 10,
	}
	client = &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			// 定时刷新当前维护的集群信息
			if *CurrentClusterName == "" {
				log.Println("ticker refresh stop, beacuse current cluster name is nil")
				continue
			}
			updateCurrentClusterInfo(*CurrentClusterName)
		}
	}()

	if *enableReport {
		log.Println("enable report cluster topology info to admin")
		go func() {
			for range AdmCh {
				reportClusterTopology()
			}
		}()		
	}

}

// 获取当前exporter监测的集群信息
// 若传入集群名和当前监测的集群不同，则会自动切换
func GetCurrentClusterInfo(clusterName string) (*ClusterInfo, error) {
	if clusterName != *CurrentClusterName {
		log.Printf("detect cluster has changed, from:%s to %s\n", *CurrentClusterName, clusterName)
		if err := updateCurrentClusterInfo(clusterName); err != nil {
			return nil, err
		}
	}
	if clusterInfo == nil {
		log.Println("cluster info is nil")
		return nil, errors.New("cluster info is nil")
	}
	mu.RLock()
	defer mu.RUnlock()
	return clusterInfo, nil
}

func updateCurrentClusterInfo(clusterName string) error {
	info, err := getAssembleInfo(clusterName)
	if err != nil {
		log.Printf("update cluster info err: %s\n", err)
		return errors.New("update cluster infor err:" + err.Error())
	}
	mu.Lock()
	defer func() {
		*CurrentClusterName = clusterName
		mu.Unlock()
	}()
	clusterInfo = info
	return nil
}

func getAssembleInfo(clusterName string) (*ClusterInfo, error) {
	req, err := http.NewRequest("GET", *adminUrl+*assembleInfoPath, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("clusterName", clusterName)
	q.Set("componentRole", "exporter")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("get assemble info err:%s\n", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("stat request fail, code:" + strconv.Itoa(resp.StatusCode))
	}

	result := &assembleResponse{}
	err = json.Unmarshal(body, result)
	if err != nil {
		return nil, err
	} else if result.Code != "0" {
		return nil, errors.New("fail: " + result.Msg)
	}

	return &result.ResultData, nil
}

type response struct {
	Code       string      `json:"code"`
	Msg        string      `json:"msg"`
	Page       *string     `json:"page"`
	Others     interface{} `json:"others"`
}

type assembleResponse struct {
	response
	ResultData ClusterInfo `json:"resultData"`
}

type reportResponse struct {
	response
	ResultData interface{} `json:"resultData"`
}

// RPD|RPD_GENERAL_REDIS_NODESET_1_CACHE|1|169.254.149.66:30001,169.254.149.66:30002,169.254.149.66:30003
func (c *ClusterInfo) UnmarshalJSON(data []byte) error {
	content := strings.Trim(string(data), "\"")
	c.Name = *CurrentClusterName
	// 处理每个分区的数据
	for _, s := range strings.Split(content, ";") {
		// 取num和host, 包含全部主从节点
		split := strings.Split(s, "|")
		if len(split) != 4 {
			continue
		}

		p := PartitionInfo{
			Name: split[1],
			Num:  split[2],
		}

		hosts := strings.Split(split[3], ",")
		nodes := make([]NodeInfo, len(hosts))
		for i, h := range hosts {
			nodes[i] = NodeInfo{
				PartitionNum: p.Num,
				PartitionName: p.Name,
				Host:         h,
			}
		}
		p.Nodes = nodes
		c.Partitions = append(c.Partitions, p)
	}
	return nil
}

// 上报集群拓扑信息到admin
func reportClusterTopology() {
	// partition <--> nodes
	topo := make(map[string]interface{}, len(ClusterTopology))
	topo["clusterName"] = *CurrentClusterName

	for name, nodes := range ClusterTopology {
		arr := strings.Split(nodes, "\n")
		n := []ClusterTopo{}

		for _, line := range arr {
			element := strings.Split(line, " ")
			if len(element) < 8 {
				//log.Printf("cluster info invalid:%s\n", line)
				continue
			}

			id := element[0]

			host := strings.Split(element[1], "@")[0]
			s := strings.Split(host, ":")
			if len(s) != 2 {
				log.Printf("invalid host:%s\n", s)
				continue
			}
			ip := s[0]
			port, err := strconv.Atoi(s[1])
			if err != nil {
				log.Printf("parse port:%s err:%s\n", s[1], err.Error())
				continue
			}

			role := element[2]
			if !strings.Contains(role, "master") {
				role = element[3]
			} else {
				role = "master"
			}

			status := "success"
			if strings.Contains(element[2], "fail") || element[7] != "connected" {
				status = "fail"
			}

			n = append(n, ClusterTopo{
				Id: id,
				Ip: ip,
				Port: port,
				Role:   role,
				Status: status,
			})
		}
		topo[name] = n
	}

	body, err := json.Marshal(topo)
	if err != nil {
		log.Printf("json encode err:%s", err.Error())
		return
	}

	//log.Printf("%s", string(b))
	req, err := http.NewRequest("POST", *adminUrl+*reportTopoPath, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("json encode err:%s", err.Error())
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("report cluster topo err:%s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("report topo err:%s\n", err.Error())
		return
	} else if resp.StatusCode != 200 {
		log.Printf("report topo reponse fail, code:%d\n", resp.StatusCode)
		return
	}

	result := &reportResponse{}
	err = json.Unmarshal(respBody, result)
	if err != nil {
		log.Printf("report topo response invalid err:%s, resp:%s\n", err.Error(), string(respBody))
		return
	} else if result.Code != "0" {
		log.Printf("report topo response code:%s\n", result.Code)
		return
	}

}