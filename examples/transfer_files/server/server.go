package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"ants"
)

func main() {
	http.HandleFunc("/ants",Ants)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080",nil))
}

func Ants(w http.ResponseWriter,r *http.Request) {
	err:=ants.DefaultUpgrader.Upgrade(w, r, func(conn *ants.Conn) {
		filepath:="../../../statics/websocket_frame.jpg"
		fd,err:=os.Open(filepath)
		if err!=nil{
			log.Println(err)
		}
		err=conn.SendFile(fd)
		if err!=nil{
			log.Println("send file err:",err)
		}
	})
	if err != nil {
		fmt.Printf("upgrade error, err=%v", err)
		return
	}

	fmt.Printf("conn upgrade done")
}

