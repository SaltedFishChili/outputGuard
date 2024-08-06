package control

import (
	"outputGuard/service"
)

func NewControlServer() *Server {
	return &Server{}
}

type Server struct {
	ss service.ServerService
}

func (cs *Server) RunServer() {
	wssServer := service.NewServer()
	httpServer := &service.HttpServer{
		WssServer: wssServer,
	}
	//解析已添加的域名
	//当发现新的A记录时自动添加白名单
	go cs.ss.LookupDomainIP(wssServer)

	httpServer.RunServerService()
}
