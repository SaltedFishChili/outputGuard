package service

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type ClientService struct{}

/*
 * 检查 IP 转发参数是否开启
 * 用以给gateway使用
 * 该方法执行 sysctl 命令来查询内核参数 net.ipv4.ip_forward 的值，
 * 如果该参数未开启（即值不为 1），iptables无法实现转发
 */
func (cs *ClientService) CheckIPForwarding() error {
	cmd := exec.Command("sysctl", "net.ipv4.ip_forward")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("执行 sysctl 命令时发生错误: %v", err))
	}

	if strings.TrimSpace(out.String()) != "net.ipv4.ip_forward = 1" {
		return fmt.Errorf(fmt.Sprintf("内核参数 net.ipv4.ip_forward 未开启, 当前值为: %s", out.String()))
	}
	return nil
}
