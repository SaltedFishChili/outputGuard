package service

import (
	"fmt"
	"net"
	. "outputGuard/logger"

	"github.com/vishvananda/netlink"
)

type HostRouter struct {
	DefaultLinkIdex int
	GatewayAddr     string
	ServerSv        *ServerService
}

func NewHostRouter() *HostRouter {
	var defaultLinkIndex int
	routes, err := netlink.RouteList(nil, netlink.NewRule().Family)
	if err != nil {
		Logger.Panic(fmt.Sprintf("获取默认网卡失败: %s", err.Error()))
	}
	for _, route := range routes {
		// 默认路由的Dst字段是nil，Gw不是nil
		if route.Dst == nil && route.Gw != nil {
			defaultLinkIndex = route.LinkIndex
			break
		}
	}
	if defaultLinkIndex == 0 {
		Logger.Panic("默认网卡未找到")
	}
	return &HostRouter{
		DefaultLinkIdex: defaultLinkIndex,
		ServerSv:        &ServerService{},
	}

}

// AddCustomRoute 添加自定义路由
func (hr *HostRouter) AddCustomRoute(destination string, cidr int) error {
	destIP := net.ParseIP(destination)
	gwIP := net.ParseIP(hr.GatewayAddr)
	if gwIP == nil || destIP == nil {
		return fmt.Errorf("invalid IP address")
	}
	isLocal, err := hr.ServerSv.isPrivateIP(destination)
	if err != nil {
		return fmt.Errorf("校验目标 IP 是否是内网 IP 失败: %s", err.Error())
	}
	// 检查目标 IP 是否是内网 IP
	if isLocal {
		Logger.Info(fmt.Sprintf("目标 IP 是内网 IP: %s", destination))
		return nil
	}

	// 如果目标 IP 不是内网 IP，执行带网关的路由添加
	route := netlink.Route{
		Dst:       &net.IPNet{IP: destIP, Mask: net.CIDRMask(cidr, 32)},
		Gw:        gwIP,
		LinkIndex: hr.DefaultLinkIdex,
	}

	if hr.RouteExists(destination) {
		if err := hr.DeleteCustomRoute(destination, cidr); err != nil {
			return fmt.Errorf("添加前删除已存在的路由失败: %s", err.Error())
		}
	}

	if err := netlink.RouteAdd(&route); err != nil {
		return err
	}
	return nil
}

// DeleteCustomRoute 删除自定义路由
func (hr *HostRouter) DeleteCustomRoute(destination string, cidr int) error {
	isLocal, err := hr.ServerSv.isPrivateIP(destination)
	if err != nil {
		return fmt.Errorf("校验目标 IP 是否是内网 IP 失败: %s", err.Error())
	}
	// 检查目标 IP 是否是内网 IP
	if isLocal {
		Logger.Info(fmt.Sprintf("目标 IP 是内网 IP: %s", destination))
		return nil
	}
	destIP := net.ParseIP(destination)
	if destIP == nil {
		return fmt.Errorf("invalid IP address")
	}

	route := netlink.Route{
		Dst:       &net.IPNet{IP: destIP, Mask: net.CIDRMask(cidr, 32)},
		LinkIndex: hr.DefaultLinkIdex,
	}
	if hr.RouteExists(destination) {
		if err := netlink.RouteDel(&route); err != nil {
			return err
		}

	}
	return nil
}

// RouteExists 检查路由是否存在
func (hr *HostRouter) RouteExists(destination string) bool {
	destIP := net.ParseIP(destination)
	if destIP == nil {
		return false
	}
	routes, err := netlink.RouteList(nil, netlink.NewRule().Family)
	if err != nil {
		Logger.Error(fmt.Sprintf("获取路由列表失败: %s", err.Error()))
		return false
	}

	for _, route := range routes {
		if route.Dst != nil && route.Dst.IP.Equal(destIP) && route.LinkIndex == hr.DefaultLinkIdex {
			return true
		}
	}
	return false
}
