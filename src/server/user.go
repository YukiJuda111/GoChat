package main

import (
	"net"
	"strings"
)

type User struct {
	Name string
	Addr string
	Ch   chan string
	conn net.Conn // socket

	server *Server
}

// 创建用户api
func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()

	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		Ch:     make(chan string),
		conn:   conn,
		server: server,
	}

	// 启动user的goroutine
	go user.ListenMessage()

	return user
}

func (user *User) Online() {
	// 将用户加入到onlineMap中
	user.server.mapLock.Lock()
	user.server.OnlineMap[user.Name] = user
	user.server.mapLock.Unlock()

	// 广播当前用户上线
	user.server.BroadCast(user, "已上线")
}

func (user *User) Offline() {
	// 将用户从onlineMap中删除
	user.server.mapLock.Lock()
	delete(user.server.OnlineMap, user.Name)
	user.server.mapLock.Unlock()

	// 广播当前用户下线
	user.server.BroadCast(user, "已下线")
}

// 给当前user发送消息
func (user *User) SendMsg(msg string) {
	user.conn.Write([]byte(msg))
}

func (user *User) HandleMessage(msg string) {
	if msg == "who" {
		user.server.mapLock.Lock()
		for _, u := range user.server.OnlineMap {
			onlineMsg := "[" + u.Addr + "]" + u.Name + ":在线...\n"
			user.SendMsg(onlineMsg)
		}
		user.server.mapLock.Unlock()
	} else if len(msg) > 7 && msg[:7] == "rename|" {
		// 消息格式: rename|newName
		newName := strings.Split(msg, "|")[1]
		// 判断name是否存在
		_, exist := user.server.OnlineMap[newName]
		if exist {
			user.SendMsg("当前用户名被使用\n")
		} else {
			user.server.mapLock.Lock()
			delete(user.server.OnlineMap, user.Name)
			user.server.OnlineMap[newName] = user
			user.server.mapLock.Unlock()

			user.Name = newName
			user.SendMsg("您已经更新用户名:" + user.Name + "\n")
		}
	} else if len(msg) > 4 && msg[:3] == "to|" {
		// 消息格式: to|userName|message
		remoteName := strings.Split(msg, "|")[1]
		if remoteName == "" {
			user.SendMsg("消息格式不正确，请使用\" to|userName|message\"格式。\n")
			return
		}

		remoteUser, ok := user.server.OnlineMap[remoteName]
		if !ok {
			user.SendMsg("当前用户不存在\n")
			return
		}
		content := strings.Split(msg, "|")[2]
		if content == "" {
			user.SendMsg("无效消息，请重发\n")
			return
		}
		remoteUser.SendMsg(user.Name + ": " + content + "\n")
		user.SendMsg("发送成功\n")
	} else {
		user.server.BroadCast(user, msg)
	}
}

// 监听当前user channel，一旦有消息，就发送给对端客户端
func (user *User) ListenMessage() {
	for {
		msg := <-user.Ch
		user.conn.Write([]byte(msg + "\n"))
	}
}
