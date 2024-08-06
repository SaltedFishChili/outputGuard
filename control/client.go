package control

import (
	"flag"
	"fmt"
	"outputGuard/global"
	. "outputGuard/logger"
	"outputGuard/pkg"
	"outputGuard/service"
)

func init() {
	global.ExporterDatas <- global.ExporterData{
		ChainName: "done",
	}
}

func NewControlClient() *Client {
	client := &Client{}

	ipt, err := service.NewIpts()
	if err != nil {
		Logger.Panic(fmt.Sprintf("初始化iptables失败:%s", err.Error()))
	}
	client.Ipt = ipt
	return client
}

type Client struct {
	Ipt service.IptableRules
	Css *service.ClientService
}

func (cc *Client) RecvierServerMessage() {

	client := service.NewWebSocketClient()
	defer client.Close()
	flag.StringVar(&client.WssServerAddr, "iptables-wss-server", "", "设置server地址")
	flag.Parse()
	client.Connect()
	client.StartReceiver()
	client.StartSender()
}

func (cc *Client) HandleIptablesMessage() {

	// 检查内核参数ip_forward是否开启
	if err := cc.Css.CheckIPForwarding(); err != nil {
		Logger.Panic(fmt.Sprintf("检查内核参数失败:%s", err.Error()))
	}

	//添加允许内网网段，避免机器访问内网失败
	if err := cc.Ipt.InitAddLocalNet(); err != nil {
		Logger.Panic(fmt.Sprintf("初始化允许内网网段失败:%s", err.Error()))
	}

	// 添加forward accept规则
	if err := cc.Ipt.CheckForwardAcceptRule(); err != nil {
		Logger.Panic(fmt.Sprintf("添加forward accept规则失败! %s", err.Error()))
	}

	// 添加drop all规则
	if err := cc.Ipt.AddDropAll(); err != nil {
		Logger.Panic(fmt.Sprintf("添加drop all失败! %s", err.Error()))
	}

	iptSem := make(chan struct{}, 10)
	for message := range global.ClientCacher.IpChan {
		iptSem <- struct{}{}
		go func(message global.Messages) {
			defer func() {
				<-iptSem
			}()

			switch message.Action {
			case "add":
				if err := cc.Ipt.AddAccept(message.IP); err != nil {
					global.ClientCacher.IpChan <- message
					Logger.Error(fmt.Sprintf("%s添加iptables规则失败:%s,写回通道继续重试", message.IP, err.Error()))
					return
				}
				if !message.IsLocalNet {

					if err := cc.Ipt.AddMasqueradeRule(message.IP); err != nil {
						global.ClientCacher.IpChan <- message
						Logger.Error(fmt.Sprintf("%s添加伪装规则失败:%s,写回通道继续重试", message.IP, err.Error()))
						return
					}
					if err := cc.Ipt.AddForwordRule(message.IP); err != nil {
						global.ClientCacher.IpChan <- message
						Logger.Error(fmt.Sprintf("%s添加转发规则失败:%s,写回通道继续重试", message.IP, err.Error()))
						return
					}

				}

			case "del":

				if err := cc.Ipt.DeleteAccept(message.IP); err != nil {
					global.ClientCacher.IpChan <- message
					Logger.Error(fmt.Sprintf("%s删除iptables规则失败:%s,写回通道继续重试", message.IP, err.Error()))
					return
				}
				if !message.IsLocalNet {
					if err := cc.Ipt.DeleteMasqueradeRule(message.IP); err != nil {
						global.ClientCacher.IpChan <- message
						Logger.Error(fmt.Sprintf("%s删除伪装规则失败:%s,写回通道继续重试", message.IP, err.Error()))
						return
					}
					if err := cc.Ipt.DeleteForwordRule(message.IP); err != nil {
						global.ClientCacher.IpChan <- message
						Logger.Error(fmt.Sprintf("%s删除转发规则失败:%s,写回通道继续重试", message.IP, err.Error()))
						return

					}
				}
			default:
				Logger.Info(fmt.Sprintf("%s的行为%s未知,不处理", message.IP, message.Action))
				return

			}
			Logger.Info(fmt.Sprintf("ip:%s %siptables成功!", message.IP, message.Action))
		}(message)
	}
}

func (cc *Client) Exporter() {

	go func() {
		for {

			<-global.RunSig
			if err := cc.Ipt.Count("filter", "FORWARD"); err != nil {
				Logger.Error(fmt.Sprintf("构建iptables exporter数据失败:%s", err.Error()))

			}
			global.ExporterDatas <- global.ExporterData{
				ChainName: "done",
			}
		}

	}()

	pkg.RunExporter()
}
