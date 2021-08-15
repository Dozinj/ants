package ants

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
)
type fields struct {
	conn           net.Conn
	bufR           *bufio.Reader
	bufW           *bufio.Writer
	isServer       bool
	State          string
	readBufferSize int
	pingTimes       int
}

func newField()fields{
	rw:=bytes.NewBuffer(nil)
	return fields{
		conn: nil,
		bufR: bufio.NewReaderSize(rw, defaultReadSize),
		bufW: bufio.NewWriter(rw),
		State: Connected,
		isServer: true,
		readBufferSize: defaultReadSize,
		pingTimes: 0,
	}
}

func TestConn_writeControlFrame(t *testing.T) {
	type args struct {
		code OpCode
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "test1",
			fields:  newField(),
			args:    args{code: opCodePing, data: []byte("Ping")},
			wantErr: false,
		},
		{
			name:    "test2",
			fields:  newField(),
			args:    args{code: opCodePong, data: []byte("Pong")},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			if err := c.writeControlFrame(tt.args.code, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("writeControlFrame() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConn_writeDataframe(t *testing.T) {
	type args struct {
		data []byte
		mt   MessageType
	}

	data1:=strings.Repeat("a",125)
	data2:=strings.Repeat("b",1024)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "less 126 bit",
			fields:  newField(),
			args:    args{mt: MessageType(opCodeText), data: []byte(data1)},
			wantErr: false,
		},
		{
			name: "more than 126 and less than 65536 bit",
			fields:  newField(),
			args:    args{mt: MessageType(opCodeText), data: []byte(data2)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			if err := c.writeDataframe(tt.args.data, tt.args.mt); (err != nil) != tt.wantErr {
				t.Errorf("writeDataframe() error = %v, wantErr %v", err, tt.wantErr)
			}

			//客户端接收
			c.isServer=false
			mt,data,err:=c.ReadMessage()
			if err!=nil{
				t.Error("ReadMessage()",err)
			}

			if !reflect.DeepEqual(mt,tt.args.mt){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", mt, tt.args.mt)
			}

			if !reflect.DeepEqual(data,tt.args.data){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", data, tt.args.data)
			}

			t.Logf("sendFrame_and_readFrames() = %v,%v, want %v,%vsuccess", mt,data, tt.args.mt,tt.args.data)
		})
	}
}



func TestConn_sendFrame_and_readFrames(t *testing.T) {
	type args struct {
		frame *Frame
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "test1",
			fields: newField(),
			args:   args{frame: &Frame{Fin: 1, Payload: []byte{1, 2}, PayloadLen: 126, PayloadExtendLen: 2}},
			wantErr: false,
		},
		{
			name:   "test2",
			fields: newField(),
			args:   args{frame: &Frame{Fin: 1, Payload: []byte{16}, PayloadLen: 16, PayloadExtendLen:0 }},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			frameSend:=*(tt.args.frame)
			if err := c.sendFrame(tt.args.frame); (err != nil) != tt.wantErr {
				t.Errorf("sendFrame() error = %v, wantErr %v", err, tt.wantErr)
			}

			//转化为客户端
			c.isServer=false
			frameRead,err:=c.readFrame()
			if err!=nil{
				t.Error("readFrame",err)
			}

			if !reflect.DeepEqual(*frameRead,frameSend){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", *frameRead, frameSend)
			}
			t.Logf("sendFrame_and_readFrames() = %v, want %v success", *frameRead, frameSend)
		})
	}
}

func TestConn_ServerWriteMessage_and_ClientReadMessage(t *testing.T) {
	type args struct {
		mt   MessageType
		data []byte
	}
	data1:=strings.Repeat("s",65535+10)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "TextMessage",
			fields:newField() ,
			args:args{mt: TextMessage,data: []byte("hello")},
			wantErr: false,
		},
		{
			name: "TextMessage2",
			fields:newField() ,
			args:args{mt: TextMessage,data: []byte{88}},
			wantErr: false,
		},
		{
			name: "BinaryMessage",
			fields:newField() ,
			args:args{mt: BinaryMessage,data: []byte("thank you")},
			wantErr: false,
		},
		{
			name: "TextMessage with fragmentDataFrames",
			fields:newField() ,
			args:args{mt: BinaryMessage,data: []byte(data1)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			mtSend:=tt.args.mt
			dataSend:=tt.args.data
			if err := c.WriteMessage(tt.args.mt, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			//客户端接收
			c.isServer=false
			mt,data,err:=c.ReadMessage()
			if err!=nil{
				t.Error("ReadMessage()",err)
				return
			}

			if !reflect.DeepEqual(mt,mtSend){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", mt, mtSend)
			}

			if !reflect.DeepEqual(data,tt.args.data){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", data, len(dataSend))
			}

			t.Logf("sendFrame_and_readFrames() = %v,%v, want %v,%vsuccess", mt,len(data), mtSend,len(dataSend))

		})
	}
}

func TestConn_ClientWriteMessage_and_ServerReadMessage(t *testing.T) {
	type args struct {
		mt   MessageType
		data []byte
	}
	client:=newField()
	client.isServer=false
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "TextMessage",
			fields:client,
			args:args{mt: TextMessage,data: []byte("hello")},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			if err := c.WriteMessage(tt.args.mt, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			//服务器接收
			c.isServer=true
			mt,data,err:=c.ReadMessage()
			if err!=nil{
				t.Error("ReadMessage()",err)
			}

			if !reflect.DeepEqual(mt,tt.args.mt){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", mt, tt.args.mt)
			}

			if !reflect.DeepEqual(data,tt.args.data){
				t.Errorf("sendFrame_and_readFrames() = %v, want %v", data, tt.args.data)
			}

			t.Logf("sendFrame_and_readFrames() = %v,%v, want %v,%v", mt,string(data), tt.args.mt,string(tt.args.data))

		})
	}
}


func TestConn_Ping(t *testing.T) {
	client:=newField()
	client.isServer=false
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "test1",
			fields:  client,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			if err := c.Ping(); (err != nil) != tt.wantErr {
				t.Errorf("Ping() error = %v, wantErr %v", err, tt.wantErr)
			}

			//服务器
			c.isServer=true
			mt,data,err:=c.ReadMessage()
			if err!=nil{
				t.Error(err)
			}

			if mt!=MessageType(opCodePing)||string(data)!="Ping"{
				t.Error(mt,string(data))
				return
			}
			t.Log(mt,string(data),"success")
		})
	}
}

func TestConn_Close(t *testing.T) {
	client:=newField()
	client.isServer=false
	tests := []struct {
		name   string
		fields *fields
	}{
		{
			name: "test1",
			fields: &client,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}

			fmt.Println(c.State)
		})

	}
}

func TestConn_close(t *testing.T) {
	client:=newField()
	client.isServer=false
	type args struct {
		closeCode int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			fields: client,
			args:args{closeCode: CloseAbnormalClosure} ,
			wantErr:false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
			}
			if err := c.close(tt.args.closeCode); (err != nil) != tt.wantErr {
				t.Errorf("close() error = %v, wantErr %v", err, tt.wantErr)
			}
			t.Log(c.State)
		})
	}
}

func TestConn_HeartBeat(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "test1",
			fields:newField() ,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conn{
				conn:           tt.fields.conn,
				bufR:           tt.fields.bufR,
				bufW:           tt.fields.bufW,
				isServer:       tt.fields.isServer,
				State:          tt.fields.State,
				readBufferSize: tt.fields.readBufferSize,
				pingTimes:      tt.fields.pingTimes,
			}
			c.HeartBeat()
			fmt.Println(c.State)
		})
	}
}