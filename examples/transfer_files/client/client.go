package main

import (
	"fmt"

	"ants"
)


func main() {
	conn,_, err := ants.DefaultDialer.Dial("ws://localhost:8080/ants")
	if err != nil {
		panic(err)
	}
	filepath:="../transfer_websocket_frame.jpg"

	err=conn.AcceptFile(filepath)
	if err != nil {
		fmt.Println("accept file err:",err)
	}
	fmt.Println("接收文件成功")
}
