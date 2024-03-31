package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	// 在线用户列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	// 消息广播的channel
	Message chan string
}

// 创建server接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}
	return server
}

// 监听Message广播消息channel的goroutine，一旦有消息就发送给全部的在线user
func (server *Server) ListenMessager() {
	for {
		msg := <-server.Message

		// 将msg发送给全部的在线用户
		server.mapLock.Lock()
		for _, cli := range server.OnlineMap {
			cli.Ch <- msg
		}
		server.mapLock.Unlock()
	}
}

// 广播
func (server *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	server.Message <- sendMsg
}

// 每次用户接入，都会启动一个handler的goroutine
func (server *Server) Handler(conn net.Conn) {
	user := NewUser(conn, server)
	user.Online()

	// 监听用户是否活跃的channel
	isLive := make(chan bool)

	// 接受客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf) // 阻塞
			if n == 0 {              // 用户调用Close()，返回0
				user.Offline()
				return
			}
			if err != nil && err != io.EOF {
				fmt.Println("Conn Read err: ", err)
			}
			msg := string(buf[:n-1]) // 去掉'\n'
			user.HandleMessage(msg)
			// 用户的任意消息，代表当前用户是一个活跃的
			isLive <- true
		}
	}()

	timer := time.NewTimer(time.Minute)
	for {
		select {
		case <-isLive:
			// 每次进isLive都重置定时器
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(time.Minute)
		case <-timer.C: // 定时器到时间后，会向C发送当前时间
			// 已经超时
			// 将当前的user强制关闭
			user.SendMsg("你已超时，已被踢出服务器")
			close(user.Ch)
			close(isLive)
			delete(server.OnlineMap, user.Name)
			conn.Close()
			return
		}
	}

}

// 启动服务器
func (server *Server) Start() {
	// socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.Ip, server.Port))
	if err != nil {
		fmt.Println("net.Listen err: ", err)
		return
	}
	// close listen socket
	defer listener.Close()

	// 启动监听Message的goroutine
	go server.ListenMessager()

	for {
		// accept
		conn, err := listener.Accept() // 阻塞
		if err != nil {
			fmt.Println("listener accept err: ", err)
			continue
		}
		// do handler
		go server.Handler(conn)
	}
	// close listen socket
}
