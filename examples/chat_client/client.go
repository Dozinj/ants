package main

import (
	"fmt"
	"log"
	"time"

	"ants"
)

func main() {
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
				break
			}
			fmt.Printf("recv failed, err=%v\n", err)
			time.Sleep(1 * time.Second)
			break
		}
		fmt.Printf("messageType=%d, msg=%s\n", mt, msg)
	}
}




