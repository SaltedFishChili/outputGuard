package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"outputGuard/global"
	. "outputGuard/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type HttpServer struct {
	WssServer *WssServer
	Ss        *ServerService
}

func (hs *HttpServer) handleWebSocket(ctx *gin.Context) {
	hostname := ctx.Query("hostname")
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		Logger.Error(fmt.Sprintf("Upgrade to WebSocket failed: %s", err.Error()))
		return
	}

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 1024),
		hostname: hostname,
		server:   hs.WssServer,
	}
	hs.WssServer.register <- client

	go client.WritePump()
	go client.ReadPump()
}

func (hs *HttpServer) Apis(ctx *gin.Context) {

	add := ctx.Query("add")
	del := ctx.Query("del")

	var messageStruct global.Messages
	if add != "" {
		result, err := hs.Ss.ServerAction(add)
		if err != nil {
			Logger.Error(fmt.Sprintf("add:解析%s失败:%s", add, err.Error()))
			ctx.JSON(http.StatusBadRequest, gin.H{
				"info":   err.Error(),
				"status": "failed",
			})
			return
		}
		addSem := make(chan struct{}, len(result.IP))
		addWg := sync.WaitGroup{}

		for _, ip := range result.IP {
			isNoDelStr := ctx.Query("nonDeletable")
			if isNoDelStr == "" {
				isNoDelStr = "false"
			}
			isNoDel, err := strconv.ParseBool(isNoDelStr)
			if err != nil {
				Logger.Error(fmt.Sprintf("nonDeletable:解析%s失败:%s", isNoDelStr, err.Error()))
				isNoDel = false
			}
			addSem <- struct{}{}
			addWg.Add(1)
			go func(ip string, messageStruct global.Messages, ctx *gin.Context) {

				defer func() {
					<-addSem
					addWg.Done()
				}()
				isLocal, err := isPrivateIP(ip)
				if err != nil {
					Logger.Error(fmt.Sprintf("isPrivateIP:解析%s失败:%s", ip, err.Error()))
				}
				go func(ip, types, Name string, isNoDel, isLocal bool) {

					if isLocal { // 内网ip强制设置为不可删除
						isNoDel = isLocal
					}
					if err := hs.WssServer.Orms.Add(types, ip, Name, time.Now().Local(), isNoDel, isLocal); err != nil {
						Logger.Error(fmt.Sprintf("添加%s 失败: %s", result.Name, err.Error()))
					}
				}(ip, result.Type, result.Name, isNoDel, isLocal)
				messageStruct.Action = "add"
				messageStruct.IP = ip
				messageStruct.IsLocalNet = isLocal

				messageJson, err := json.Marshal(messageStruct)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{
						"info":   err.Error(),
						"status": "failed",
					})
					Logger.Error(fmt.Sprintf("Error marshaling message: %s", err.Error()))
					return
				}
				Logger.Info(fmt.Sprint("即将发布的add任务:", string(messageJson)))
				hs.WssServer.broadcast <- []byte(messageJson)

			}(ip, messageStruct, ctx)
		}
		addWg.Wait()
		ctx.JSON(http.StatusOK, gin.H{
			"info":   result.Name,
			"status": "success",
		})
	}
	if del != "" {
		result, err := hs.Ss.ServerAction(del)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"info":   err.Error(),
				"status": "failed",
			})
			return
		}
		delSem := make(chan struct{}, len(result.IP))
		delWg := sync.WaitGroup{}
		for _, ip := range result.IP {
			noDel, err := hs.WssServer.Orms.QueryNoDel(ip)
			if err != nil {
				Logger.Error(fmt.Sprintf("查询%s 是否可删除失败: %s", result.Name, err.Error()))
			}
			if noDel {
				ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"info":   fmt.Sprintf("%s 不可删除", ip),
					"status": "failed",
				})
				return
			}
			delSem <- struct{}{}
			delWg.Add(1)
			go func(ip string, messageStruct global.Messages, ctx *gin.Context) {

				defer func() {
					<-delSem
					delWg.Done()
				}()

				go func(ip string) {
					if err := hs.WssServer.Orms.Del(ip); err != nil {
						Logger.Error(fmt.Sprintf("删除%s 失败: %s", result.Name, err.Error()))
					}
				}(ip)
				isLocal, err := isPrivateIP(ip)
				if err != nil {
					Logger.Error(fmt.Sprintf("isPrivateIP:解析%s失败:%s", ip, err.Error()))
				}
				messageStruct.Action = "del"
				messageStruct.IP = ip
				messageStruct.IsLocalNet = isLocal
				messageJson, err := json.Marshal(messageStruct)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{
						"info":   err.Error(),
						"status": "failed",
					})
					Logger.Error(fmt.Sprintf("Error marshaling message: %s", err.Error()))
					return
				}
				Logger.Info(fmt.Sprint("即将发布的del任务:", string(messageJson)))
				hs.WssServer.broadcast <- []byte(messageJson)

			}(ip, messageStruct, ctx)
		}
		delWg.Wait()
		ctx.JSON(http.StatusOK, gin.H{
			"info":   del,
			"status": "success",
		})
	}
}

func (hs *HttpServer) ShowAll(c *gin.Context) {
	allRecords, err := hs.WssServer.Orms.QueryAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch records from the database",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"Records": allRecords,
	})

}

func (hs *HttpServer) RunServerService() {
	r := gin.Default()

	// 获取当前工作目录
	workDir, err := os.Getwd()

	if err != nil {
		Logger.Panic(fmt.Sprintf("Failed to get working directory: %s", err.Error()))
	}

	// 构建静态文件和目录的绝对路径
	staticDir := filepath.Join(workDir, "static")
	indexFile := filepath.Join(staticDir, "index.html")
	r.StaticFile("/", indexFile)
	r.Static("/static", staticDir)

	wssGroup := r.Group("/ws")
	wssGroup.GET("", hs.handleWebSocket)

	go hs.WssServer.Run()

	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/show-all", hs.ShowAll)
	r.GET("/api", hs.Apis)

	if err := r.Run(":8080"); err != nil {
		Logger.Panic(fmt.Sprintf("HTTP server failed: %s", err.Error()))
	}
}
