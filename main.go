package main

import (
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

func main() {
	for {
		logger.Println("开始检查游戏服务器是否有更新...")
		if checkForUpdate() {
			logger.Println("检测到更新，开始执行更新过程...")
			executeUpdate()
		} else {
			logger.Println("未检测到更新，10分钟后再次检查...")
			ensureServerRunning() // 不使用goroutine，直接在主线程中检查和启动服务器
		}
		time.Sleep(10 * time.Minute)
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
	output, _ := executeCommand("ps -aux | grep " + serverProcessName + " | awk '{print $2}'")
	pids := strings.Split(output, "\n")
	for _, pid := range pids {
		if pid != "" {
			_, killErr := executeCommand("kill -9 " + pid)
			if killErr != nil {
				logger.Printf("尝试杀死进程 %s 时发生错误: %v", pid, killErr)
			}
		}
	}
	time.Sleep(5 * time.Second) // 等待几秒以确保进程已被杀死
	logger.Println("正在应用游戏服务器更新...")
	_, updateErr := executeCommand(updateCmd)
	if updateErr != nil {
		logger.Printf("更新游戏服务器时发生错误: %v", updateErr)
	}
	logger.Println("正在重启游戏服务器...")
	_, restartErr := executeCommand(serverStartCmd)
	if restartErr != nil {
		logger.Printf("重启游戏服务器时发生错误: %v", restartErr)
	} else {
		logger.Println("游戏服务器更新并重启成功。")
	}
}
