# 动态防火墙架构图

本项目是一个基于iptables的动态防火墙系统，主要用于处理动态变化的域名和IP地址，以满足不允许线上随意访问公网的需求，同时自动化管理外网访问规则，提高效率。

## 系统架构图

```mermaid
graph LR
    subgraph "Server"
        S1[Web Interface]
        S2[API Interface]
        S3[WSS Interface]
        S4[Domain Resolver]
    end

    subgraph "Gateway"
        G1[IPTables Manager]
        G2[Prometheus Exporter]
    end

    subgraph "Route"
        R1[Route Manager]
    end

    S1 --> S2
    S2 --> S3
    S2 --> S4

    S3 --> G1
    S4 --> G1

    G1 --> G2

    R1 --> G1

系统组件

1. Server

	•	Web Interface：提供一个网页界面，可以通过该界面添加或删除需要访问的域名和IP地址。
	•	API Interface：提供API接口，可以通过API方式添加或删除域名和IP地址。
	•	WSS Interface：提供WebSocket接口，允许Gateway注册并接受任务。
	•	Domain Resolver：每分钟自动解析添加的域名，如果A记录有变化则自动更新iptables规则。

2. Gateway

	•	IPTables Manager：运行在有完全互联网权限的机器上，接收Server发布的添加/删除任务并添加到iptables。
	•	Prometheus Exporter：统计每个IP的收发流量，并将数据暴露给Prometheus进行监控。

3. Route

	•	Route Manager：运行在任意需要访问外网的机器或K8s Pod中，将所有非内网网段的路由指向Gateway。