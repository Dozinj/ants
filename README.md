## 简介

ants 是一个仿写的轻轻量级`websocket`框架,这也是这个名字的由来。框架基本实现了websocket数据帧传输形式,其他的...

![流程图](https://thumbnail1.baidupcs.com/thumbnail/7058c6a87v680e370314b66eef46ce47?fid=1665266475-250528-345263831835967&rt=pr&sign=FDTAER-DCb740ccc5511e5e8fedcff06b081203-YAOgiEOKJy5QpGksQ574geVDLVo%3d&expires=8h&chkbd=0&chkv=0&dp-logid=8686842231510637079&dp-callid=0&time=1628906400&size=c1536_u864&quality=90&vuk=1665266475&ft=image&autopolicy=1)

### 简单使用

`client`

```go
conn,_, err := ants.DefaultDialer.Dial("ws://localhost:8080/ants")
	if err != nil {
		log.Println(err)
		return
	}
	go func() {
		for {
			if err = conn.WriteMessage(ants.TextMessage,[]byte("hello")); err != nil {
				fmt.Printf("send failed, err=%v\n", err)
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			if ce, ok := err.(*ants.CloseError); ok {
				fmt.Printf("close err=%d, %s\n", ce.Code, ce.Text)
				return
			}
			fmt.Printf("recv failed, err=%v\n", err)
			time.Sleep(1 * time.Second)
			return
		}
		fmt.Printf("messageType=%d, msg=%s\n", mt, msg)
	}
```

`server`
```go
func main() {
	http.HandleFunc("/ants",Ants)
	log.Fatal(http.ListenAndServe(":8080",nil))
}

func Ants(writer http.ResponseWriter, request *http.Request) {
	err := ants.DefaultUpgrader.Upgrade(writer, request, func(conn *ants.Conn) {
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				if closeErr, ok := err.(*ants.CloseError); ok {
					fmt.Printf("conn closed, because=%v\n", closeErr)
					break
				}
				fmt.Printf("read error, err=%v\n", err)
				break
			}

			fmt.Printf("recv: mt=%d, msg=%s\n", mt, message)

			err = conn.WriteMessage(mt, message)
			if err != nil {
				fmt.Printf("write error: err=%v\n", err)
				break
			}
		}
		fmt.Printf("conn finished")
	})

	if err != nil {
		fmt.Printf("upgrade error, err=%v\n", err)
		return
	}

	fmt.Printf("conn upgrade done\n")
}
```



### 关于websocket

WebSocket是一种全新的协议。它将TCP的Socket（套接字）应用在了web page上，从而使通信双方建立起一个保持在活动状态连接通道，并且属于**全双工**（双方同时进行双向通信）。WebSocket协议借用HTTP协议的`101 switch protocol`来达到协议转换的，从HTTP协议切换成WebSocket通信协议。另外WebSocket传输的数据都是以`Frame`（帧）的形式实现的。

[更多相关](https://www.huaweicloud.com/articles/4157e9b5a58ef15e29d71f76b08e1b92.html)







