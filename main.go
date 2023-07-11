/*
 * @Author: xx
 * @Date: 2023-03-17 16:25:35
 * @LastEditTime: 2023-04-20 16:05:43
 * @Description:
 */
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oschwald/geoip2-golang"
)

// GameConfig 游戏配置结构体
type GameConfig struct {
	Url             string `json:"url"`
	AFKey           string `json:"AFKey"`
	AdjustToken     string `json:"AdjustToken"`
	Orientation     string `json:"Orientation"`
	JSInterfaceName string `json:"JSInterfaceName"`
	IsOpen          bool   `json:"isOpen"`
}

type Config map[string]GameConfig

var configMap Config

var ipCityDB *geoip2.Reader

func main() {
	// 读取配置文件
	readConfig()

	// 读取ip库
	readIpConfig()

	// 创建gin实例
	router := gin.Default()

	// 设置路由
	router.GET("/get", getHandler)
	router.GET("/set", setHandler)
	router.GET("/showlog", showlogHandler)
	router.GET("/reloadCfg", reloadCfg)

	// 启动http服务
	router.Run("0.0.0.0:8089")
}

// 读取配置文件
func readConfig() {
	filePtr, err := os.Open("./config.json")
	if err != nil {
		fmt.Print(err)
		return
	}
	defer filePtr.Close()

	decoder := json.NewDecoder(filePtr)
	err = decoder.Decode(&configMap)
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Print("config loaded")
}

func readIpConfig() {
	var err error

	ipCityDB, err = geoip2.Open("./GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
}

// 重载配置
func reloadCfg(c *gin.Context) {
	readConfig()
	c.JSON(http.StatusOK, configMap)
}

// get请求处理函数
func getHandler(c *gin.Context) {
	// 获取参数
	gameID := c.Query("gameId")

	// 获取对应的配置
	gameConfig, ok := configMap[gameID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("gameId '%s' not found", gameID),
		})
		return
	}

	if !gameConfig.IsOpen {
		// 如果没有打开开关，在审核状态 记录访问日志
		logRequest(c.Request, gameID)
	}

	// 返回配置
	c.JSON(http.StatusOK, gameConfig)
}

// 返回日志
func showlogHandler(c *gin.Context) {
	fileBody, err := ioutil.ReadFile("log.txt")
	if err != nil {
		log.Fatal(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error reading log file",
		})
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", fileBody)
}

// set请求处理函数
func setHandler(c *gin.Context) {
	// 获取参数
	gameID := c.Query("gameId")
	open := c.Query("open")

	// 更新配置
	gameConfig, ok := configMap[gameID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("gameId '%s' not found", gameID),
		})
		return
	}

	gameConfig.IsOpen = (open == "true")
	configMap[gameID] = gameConfig

	// 保存配置文件
	saveConfig()

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("gameId '%s' isOpen set to '%s'", gameID, open),
	})
}

// 记录访问日志
func logRequest(req *http.Request, gameID string) {
	remoteAddr, err := getRemoteIP(req)
	if err != nil {
		fmt.Print(err)
		return
	}

	country, city := getIPCode(remoteAddr)
	if err != nil {
		fmt.Print(err)
		return
	}

	logText := fmt.Sprintf("[%s] %s %s %s %s %s\n", time.Now().Format("2006-01-02 15:04:05"), remoteAddr, country, city, gameID, req.URL.Path)
	fmt.Print(logText)

	// 打开或创建 log.txt 文件，以追加写入的方式打开
	file, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer file.Close()

	// 使用 file.Write() 函数进行写入
	_, err = file.Write([]byte(logText))
	if err != nil {
		fmt.Printf("Failed to write log file: %v\n", err)
	}
}

// 获取访问ip
func getRemoteIP(req *http.Request) (string, error) {
	remoteAddr := req.RemoteAddr
	if ip, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = ip
	} else {
		return "", err
	}
	ip := net.ParseIP(remoteAddr)
	if ip == nil {
		return "", fmt.Errorf("invalid remote ip: %s", remoteAddr)
	}
	return ip.String(), nil
}

// 获取城市代码
func getIPCode(ip string) (string, string) {
	if ip == "127.0.0.1" {
		return "本地", "localhost"
	}
	ipParse := net.ParseIP(ip)
	record, err := ipCityDB.City(ipParse)
	if err != nil {
		log.Fatal(err)
	}
	return record.Country.Names["zh-CN"], record.City.Names["en"]
}

// 保存配置文件
func saveConfig() {

	bytes, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return
	}

	err = ioutil.WriteFile("config.json", bytes, 0644)
	if err != nil {
		return
	}
	fmt.Print("config saved")

}
