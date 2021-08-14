package main

import (
	"fmt"
	"log"
	"net/http"

	"ants"
)

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



