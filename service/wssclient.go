package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"outputGuard/global"
	. "outputGuard/logger"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn          *websocket.Conn
	done          chan struct{}
	WssServerAddr string
}

func NewWebSocketClient() *WebSocketClient {
	return &WebSocketClient{
		conn: nil,
		done: make(chan struct{}),
	}
}

func (wc *WebSocketClient) Connect() error {
	hostname, _ := os.Hostname()
	u := url.URL{Scheme: "ws", Host: wc.WssServerAddr, Path: "/ws", RawQuery: "hostname=" + hostname}
	Logger.Info(fmt.Sprintf("开始连接wss server %s\n", u.String()))

	for {
		select {
		case <-wc.done:
			Logger.Error("连接wss server失败,尝试重新连接")
			return nil
		default:
			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err == nil {
				wc.conn = conn
				Logger.Info("连接wss server成功")
				return nil
			}

			Logger.Error(fmt.Sprintf("连接wss server失败: %s, 等待5秒重试", err.Error()))
			time.Sleep(5 * time.Second)
		}
	}
}

func (wc *WebSocketClient) StartReceiver() {
	go func() {
		for {
			select {
			case <-wc.done:
				return
			default:
				_, message, err := wc.conn.ReadMessage()
				if err != nil {

					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						Logger.Error(fmt.Sprintf("连接关闭：%s", err.Error()))
					} else {
						Logger.Error(fmt.Sprintf("读取数据失败：%s", err.Error()))
						wc.Connect()
					}

				}

				var mgs global.Messages
				err = json.Unmarshal(message, &mgs)
				if mgs.IP == "" {
					Logger.Info(fmt.Sprintf("client接收ip为空的数据: %s", string(message)))
					continue
				}
				Logger.Info(fmt.Sprintf("client接收到数据: %s", string(message)))
				if err != nil {
					Logger.Error(fmt.Sprintf("解析数据失败：%s,原始字符串：%s", err.Error(), string(message)))
					continue
				}
				global.ClientCacher.IpChan <- mgs
			}
		}
	}()
}

func (wc *WebSocketClient) StartSender() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wc.done:
			return
		case t := <-ticker.C:
			err := wc.conn.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				Logger.Error(fmt.Sprintf("发送数据失败:%s", err.Error()))
			}
		}
	}
}

func (wc *WebSocketClient) Close() {
	close(wc.done)
	if wc.conn != nil {
		wc.conn.Close()
	}
}
