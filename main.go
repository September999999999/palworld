package main

import (
	"github.com/gorcon/rcon"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	logger            = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	serverStartCmd    = "/home/steam/.steam/steam/steamapps/common/PalServer/PalServer.sh -rcon"
	serverProcessName = "Pal"
	updateCheckCmd    = "steamcmd +login anonymous +app_info_update 1 +app_info_print 2394010 +quit"
	updateCmd         = "steamcmd +login anonymous +app_update 2394010 validate +quit"
	lastChangeFile    = "last_change_info.txt"
)

func init() {
	os.Setenv("TZ", "Asia/Shanghai")
	time.LoadLocation("Asia/Shanghai")
}
func getUserNumber() (userNumber int) {
	conn, err := rcon.Dial("192.168.31.12:25575", "Qwer.123456")
	if err != nil {
		logger.Println(err)
		return 0
	}
	defer conn.Close()

	response, err := conn.Execute("ShowPlayers")
	if err != nil {
		logger.Println(err)
		return 0
	}
	records := strings.Split(response, "\n")

	// 数据条数即为records切片的长度
	dataCount := len(records)
	return dataCount - 2
}

func main() {
	const checkInterval = 10 * time.Minute
	for {
		userNumber := getUserNumber()
		if userNumber > 0 {
			logger.Printf("玩家在线 %d, 跳过更新...", userNumber)
		} else {
			logger.Println("开始检查游戏服务器是否有更新...")
			if checkForUpdate() {
				logger.Println("检测到更新，开始执行更新过程...")
				executeUpdate()
			} else {
				logger.Println("未检测到更新，10分钟后再次检查...")
				ensureServerRunning()
			}
		}

		time.Sleep(checkInterval)
	}
}

func executeCommand(cmdStr string) (output string, err error) {
	cmd := exec.Command("bash", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Printf("执行命令时发生错误: %v", err)
		return string(out), err
	}
	return string(out), nil
}

func ensureServerRunning() {
	output, err := executeCommand("netstat -tuln | grep :25575 | awk '{print $4}'")
	if err != nil || output == "" {
		logger.Println("游戏服务器未在运行，正在启动...")
		_, startErr := executeCommand(serverStartCmd)
		if startErr != nil {
			logger.Printf("启动游戏服务器时发生错误: %v", startErr)
		} else {
			logger.Println("游戏服务器启动成功。")
		}
	} else {
		logger.Printf("游戏服务器端口: %s\n", output)
	}
}

func checkForUpdate() bool {
	logger.Println("正在执行更新检查...")
	output, _ := executeCommand(updateCheckCmd)
	re := regexp.MustCompile(`last change :\s*(.+)`)
	matches := re.FindSubmatch([]byte(output))
	if len(matches) < 2 {
		logger.Println("未能在输出中找到最后更改的时间戳。")
		return false
	}
	lastChange := string(matches[1])
	currentInfo, err := ioutil.ReadFile(lastChangeFile)
	if err != nil || string(currentInfo) != lastChange {
		logger.Println("发现新的更新，准备开始更新流程...")
		writeLastChangeInfo(lastChange)
		return true
	}
	logger.Println("当前服务器已是最新状态，无需更新。")
	return false
}

func writeLastChangeInfo(info string) {
	err := ioutil.WriteFile(lastChangeFile, []byte(info), 0644)
	if err != nil {
		logger.Printf("写入最后更改信息到文件时发生错误: %v", err)
	} else {
		logger.Println("已成功更新标识文件。")
	}
}

func executeUpdate() {
	logger.Println("正在停止游戏服务器...")
	output, _ := executeCommand("pgrep " + serverProcessName) // 使用 pgrep 查找进程ID
	pids := strings.Split(output, "\n")
	if len(pids) == 0 || (len(pids) == 1 && pids[0] == "") {
		logger.Println("未找到游戏服务器进程，无需杀死。")
	} else {
		for _, pid := range pids {
			if pid != "" {
				_, killErr := executeCommand("sudo kill -9 " + pid) // 使用sudo提高权限
				if killErr != nil {
					logger.Printf("尝试杀死进程 %s 时发生错误: %v", pid, killErr)
				} else {
					logger.Printf("进程 %s 已被成功杀死。", pid)
				}
			}
		}
	}
	time.Sleep(5 * time.Second) // 等待几秒以确保进程已被杀死
	logger.Println("正在应用游戏服务器更新...")
	_, updateErr := executeCommand(updateCmd)
	if updateErr != nil {
		logger.Printf("更新游戏服务器时发生错误: %v", updateErr)
	} else {
		logger.Println("游戏服务器更新成功。")
	}
	logger.Println("正在重启游戏服务器...")
	_, restartErr := executeCommand(serverStartCmd)
	if restartErr != nil {
		logger.Printf("重启游戏服务器时发生错误: %v", restartErr)
	} else {
		logger.Println("游戏服务器重启成功。")
	}
}
