package main

/*
 * 加所有的公网ip段
 * 操作node路由
 * 指向iptables 代理
 */
import (
	"fmt"
	"outputGuard/control"
	. "outputGuard/logger"
)

func main() {
	client := control.NewControlRouter()
	if err := client.BuildRouter(); err != nil {
		Logger.Panic(fmt.Sprintf("路由添加失败:%s", err.Error()))
	}
}
