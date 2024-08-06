package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"outputGuard/global"
	. "outputGuard/logger"
	"strings"
	"sync"
	"time"
)

type ServerService struct {
	Type         string
	Name         string
	IP           []string
	IsDoamin     bool
	DNSResolvers DNSResolver
	DNSAddr      string
	// Orms         *orm.ORM
}

type DNSResolver struct {
	Resolver *net.Resolver
}

func (ss *ServerService) ServerAction(ip string) (ServerService, error) {
	result, err := ss.GetIPv4Addresses(ip)
	if err != nil {
		return ServerService{}, err
	}
	return result, nil
}

func (ss *ServerService) GetIPv4Addresses(input string) (ServerService, error) {
	var ssr ServerService
	if strings.Contains(input, "/") {
		ssr.Type = "IP"
		ssr.IP = []string{input}
		ssr.IsDoamin = false
		ssr.Name = input
		return ssr, nil
	}

	ip := net.ParseIP(input)
	if ip != nil {

		if ip.To4() != nil {
			ssr.Type = "IP"
			ssr.IP = []string{ip.String()}
			ssr.IsDoamin = false
			ssr.Name = input
			return ssr, nil
		}
		return ssr, fmt.Errorf("invalid IPv4 address: %s", input)
	}

	ipv4Addresses, err := ss.getIPv4AddressesForDomain(input)
	if err != nil {
		return ssr, err
	}

	ssr.Type = "Domain"
	ssr.IP = ipv4Addresses
	ssr.IsDoamin = true
	ssr.Name = input
	return ssr, nil
}

/*
 * 优先使用指定的DNS
 * 其次使用系统的resolv
 */
func (ss *ServerService) BuildDNSResolver() {
	var resolver *net.Resolver

	if ss.DNSAddr != "" {
		// 使用指定的 DNS 地址解析
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Second * 5, // 设置一个超时时间
				}
				return d.DialContext(ctx, "udp", ss.DNSAddr)
			},
		}
	} else {
		// 使用系统的 resolv.conf 中的 DNS
		resolver = net.DefaultResolver
	}

	ss.DNSResolvers = DNSResolver{
		Resolver: resolver,
	}
}

func (ss *ServerService) getIPv4AddressesForDomain(domain string) ([]string, error) {
	ss.BuildDNSResolver()
	ipbj, err1 := ss.DNSResolvers.Resolver.LookupIP(context.Background(), "ip", domain)
	if err1 != nil {
		return nil, fmt.Errorf("dns解析失败: %v", err1)
	}

	ipv4AddressesMap := make(map[string]struct{})

	for _, ip := range ipbj {
		if ip.To4() != nil {
			ipv4AddressesMap[ip.String()] = struct{}{}
		}
	}

	ipv4Addresses := make([]string, 0, len(ipv4AddressesMap))
	for ip := range ipv4AddressesMap {
		ipv4Addresses = append(ipv4Addresses, ip)
	}

	if len(ipv4Addresses) == 0 {
		return nil, fmt.Errorf("no IPv4 addresses found for the domain")
	}

	return ipv4Addresses, nil
}

/*
 * 每十分钟解析已添加的域名
 * 如果存在新的A记录则自动添加白名单
 */
func (ss *ServerService) LookupDomainIP(wssServer *WssServer) {
	for {
		time.Sleep(1 * time.Minute)
		domian, err := wssServer.Orms.QueryUniqueDomainNames()
		Logger.Info(fmt.Sprintf("查询到的域名为:%s", domian))
		if err != nil {
			Logger.Error(fmt.Sprintf("查询域名失败: %s", err.Error()))
			return
		}
		sem := make(chan struct{}, len(domian))
		wg := sync.WaitGroup{}

		for _, domain := range domian {
			sem <- struct{}{}
			wg.Add(1)
			go func(domain string, wssServer *WssServer) {
				defer func() {
					<-sem
					wg.Done()
				}()
				result, err := ss.GetIPv4Addresses(domain)
				if err != nil {
					Logger.Error(fmt.Sprintf("查询域名 %s 失败: %s", domain, err.Error()))
					return
				}
				for _, ip := range result.IP {

					isExits, err := wssServer.Orms.Query(ip)
					if err != nil {
						Logger.Error(fmt.Sprintf("查询IP %s 是否存在失败!: %s", ip, err.Error()))
						continue
					}
					if isExits {
						Logger.Info(fmt.Sprintf("域名:%s解析到的ip:%s已存在,不再重复添加", domain, ip))
						continue
					}
					isLocal, err := isPrivateIP(ip)
					if err != nil {
						Logger.Error(fmt.Sprintf("isPrivateIP:解析%s失败:%s", ip, err.Error()))
					}
					var messageStruct global.Messages
					messageStruct.Action = "add"
					messageStruct.IP = ip
					messageStruct.IsLocalNet = isLocal
					messageJson, err := json.Marshal(messageStruct)
					if err != nil {
						Logger.Error(fmt.Sprintf("Error marshaling message: %s", err.Error()))
						continue
					}
					wssServer.broadcast <- []byte(messageJson)
					go func(ip, Type, Name string, isLocal bool) {
						if err := wssServer.Orms.Add(Type, ip, Name, time.Now().Local(), isLocal, isLocal); err != nil {
							Logger.Error(fmt.Sprintf("添加IP %s 失败: %s", ip, err.Error()))
						}
					}(ip, result.Type, result.Name, isLocal)
					Logger.Info(fmt.Sprintf("域名:%s解析到的ip:%s添加成功", domain, ip))
				}

			}(domain, wssServer)
		}
		wg.Wait()

	}
}

func isPrivateIP(ipAddr string) (bool, error) {
	if strings.Contains(ipAddr, "/") {
		ipAddr = strings.Split(ipAddr, "/")[0]
	}
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return false, fmt.Errorf("无效的IP地址")
	}

	// 私有IP地址范围
	privateIPBlocks := []*net.IPNet{
		//100.64.0.0/10
		&net.IPNet{IP: net.ParseIP("100.64.0.0"), Mask: net.CIDRMask(10, 32)},
		// 127.0.0.1 – 127.255.255.255
		&net.IPNet{IP: net.ParseIP("127.0.0.0"), Mask: net.CIDRMask(8, 32)},
		// 10.0.0.0 – 10.255.255.255
		&net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		// 172.16.0.0 – 172.31.255.255
		&net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		// 192.168.0.0 – 192.168.255.255
		&net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
		//169.254.0.0/16
		&net.IPNet{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)},
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true, nil
		}
	}

	return false, nil
}
