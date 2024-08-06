package service

import (
	"encoding/json"
	"fmt"
	"outputGuard/global"
	. "outputGuard/logger"
	"outputGuard/model/orm"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	hostname string
	server   *WssServer
}

type WssServer struct {
	Orms       *orm.ORM
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mutex      sync.Mutex
}

func NewServer() *WssServer {
	return &WssServer{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client, 10000),
		unregister: make(chan *Client, 10000),
		broadcast:  make(chan []byte, 10000),
	}
}

func (s *WssServer) Run() {
	go func() {
		for {
			select {
			case client := <-s.register:
				s.registerClient(client)
			}
		}
	}()

	go func() {
		for {
			select {
			case client := <-s.unregister:
				s.unregisterClient(client)
			}
		}
	}()
	for {
		select {
		case message := <-s.broadcast:
			s.broadcastMessage(message)
		}
	}
}

func (s *WssServer) registerClient(client *Client) {
	s.mutex.Lock()
	s.clients[client] = true
	go s.sendMessageToFirstRegisterClient(client)
	Logger.Info(fmt.Sprintf("客户端:%s注册成功,当前客户端数:%d", client.hostname, len(s.clients)))
	s.mutex.Unlock()
}

func (s *WssServer) unregisterClient(client *Client) {
	s.mutex.Lock()
	if _, ok := s.clients[client]; ok {
		delete(s.clients, client)
		close(client.send)
		Logger.Info(fmt.Sprintf("客户端:%s注销成功,当前客户端数:%d", client.hostname, len(s.clients)))

	}
	s.mutex.Unlock()
}

func (s *WssServer) broadcastMessage(message []byte) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for client := range s.clients {

		go func(c *Client) {
			select {
			case c.send <- message:
				return
			default:

				Logger.Info(fmt.Sprintf("发送消息给客户端:%s失败", c.hostname))
				//失败重试
				s.retryFailedMessage(c, message)
			}
		}(client)
	}
}

func (s *WssServer) sendMessageToFirstRegisterClient(client *Client) {
	ips, err := s.Orms.QueryAll()
	if err != nil {
		Logger.Error(fmt.Sprintf("查询IP失败: %s", err.Error()))
	}
	for _, ip := range ips {
		messageStruct := global.Messages{
			IP:         ip.IP,
			Action:     "add",
			IsLocalNet: ip.IsLocalNet,
		}
		messageJson, err := json.Marshal(messageStruct)
		if err != nil {
			Logger.Error(fmt.Sprintf("Error marshaling message: %s", err.Error()))
			continue
		}
		message := []byte(messageJson)

		go func(client *Client, message []byte) {
			s.mutex.Lock()
			defer s.mutex.Unlock()
			select {
			case client.send <- message:
				return
			default:
				s.retryFailedMessage(client, message)
			}

		}(client, message)

	}
}

func (s *WssServer) retryFailedMessage(c *Client, message []byte) {
	Logger.Info(fmt.Sprintf("重试发送消息给客户端:%s", c.hostname))
	time.Sleep(500 * time.Millisecond)
	maxRetries := 5 // 最大重试次数
	retryCount := 0

	for retryCount < maxRetries {
		select {
		case c.send <- message:
			Logger.Info(fmt.Sprintf("重试消息发送给客户端:%s 成功", c.hostname))
			return
		default:
			time.Sleep(500 * time.Millisecond)
			retryCount++
		}
	}

	Logger.Error(fmt.Sprintf("消息发送给客户端:%s 失败，已达到最大%d次数", c.hostname, maxRetries))
}

func (c *Client) WritePump() {
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.server.unregister <- c
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			c.server.unregister <- c
			return
		}
	}
}
