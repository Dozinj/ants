package ants

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"
)

const (
	Connecting ="connecting"
	Connected ="connected"
	Closing ="closing"
	Closed="closed"
)

var (
	ErrMaskNotSet = errors.New("mask is not set")
	ErrMaskSet    = errors.New("mask is set")
	ErrNilRead =errors.New("conn is closed")
)


type MessageType uint16

const (
	// NoFrame .
	NoFrame MessageType = 0
	// TextMessage .
	TextMessage = MessageType(opCodeText)
	// BinaryMessage .
	BinaryMessage = MessageType(opCodeBinary)

	//CloseMessage 表示关闭控制消息。可选的消息有效负载包含数字代码和文本。
	CloseMessage = MessageType(opCodeClose)
	// PingMessage .
	PingMessage = MessageType(opCodePing)
	// PongMessage .
	PongMessage = MessageType(opCodePong)
)

const defaultReadSize =65535


type Conn struct {
	conn     net.Conn
	bufR     *bufio.Reader
	bufW     *bufio.Writer
	isServer bool

	//conn 连接状态
	State    string

	//read缓冲区长度
	readBufferSize int
	mu sync.Mutex
	pongHandler func(payload string)
}

func newConn(netConn net.Conn,isServer bool)*Conn {
	conn := &Conn{
		conn: netConn,
		//数据帧最长格式最长情况: 2B head +4B maskKey + 8B payload_extend
		bufR: bufio.NewReaderSize(netConn,defaultReadSize+minFrameHeaderSize+8),
		bufW: bufio.NewWriter(netConn),
		isServer: isServer,
		State: Connecting,
		readBufferSize: defaultReadSize,//to fix bug
	}

	return conn
}

//read Peek返回接下来的n个字节，而不推进读取器。
//调用Peek可防止未读字节或未读字节调用成功，直到下一次读取操作
func (c *Conn)read(n int)([]byte,error){
	p,err:=c.bufR.Peek(n)
	if err==io.EOF{
		return nil, err
	}

	if len(p)==0{
		return nil,ErrNilRead
	}

	//缓冲区移除字节
	_,_=c.bufR.Discard(len(p))
	return p,nil
}



//解析websocket 数据帧
func (c *Conn)readFrame()(*Frame,error) {
	//先读取数据帧头部
	p, err := c.read(2)
	//如果没有数据来，这将被阻止
	if err != nil {
		return nil, err
	}

	//获取数据帧头部的前两字节    fin+rev1+rev2+rev3+mask+payloadLen
	frame := parseFrameHeader(p)

	var payloadExtendLen uint64 =0//若PayloadLen<126 则为0

	//PayloadLen
	switch frame.PayloadLen {
	case 126:
		//7+16 bits
		p, err = c.read(2)
		if err != nil {
			return nil, err
		}

		//计算payloadExtendLen
		first:=uint64(p[0])*256
		second:=uint64(p[1])
		payloadExtendLen=first+second

	case 127:
		//当payloadExtendLen>65535 即payload 大于65535 字节时
		// 7+64 bits
		p, err = c.read(8)
		if err != nil {
			return nil, err
		}
		for i:=0;i<len(p);i++{
			//[]byte{8 ,7, 6, 5, 4, 3, 2, 1}   256=1<<8  256*2=1<<16
			//payloadExtendLen = 1 + 2*256 + 3*256*2 +...+ 8*256*7
			payloadExtendLen+=uint64(p[i])*uint64(1<<8*i)
		}
	default:
		//0-125 payloadExtendLen 默认为0
	}
	frame.PayloadExtendLen = payloadExtendLen

	//mask
	if frame.Mask == 1 {
		// only 32bit masking key to read
		p, err = c.read(4)
		if err != nil {
			return nil, err
		}
		frame.MaskingKey = binary.BigEndian.Uint32(p)
	}


	//验证协议基本规范
	if err = frame.valid(); err != nil {
		c.close(CloseProtocolError)
		return nil, err
	}

	//验证掩码
	if err := c.validMask(frame); err != nil {
		return nil, err
	}

	//负载数据 通过数据帧中定义长度分配容量 payload最少也有1Byte
	if payloadExtendLen==0{
		payloadExtendLen=1
	}

	payload := make([]byte, 0, payloadExtendLen)

	for payloadExtendLen > uint64(c.readBufferSize){
		p, err := c.read(c.readBufferSize)
		if err != nil {
			return nil, err
		}
		payload = append(payload, p...)
		payloadExtendLen -= uint64(c.readBufferSize)
	}


	//读取剩余部分
	p, err = c.read(int(payloadExtendLen))
	if err != nil {
		return nil, err
	}
	payload = append(payload, p...)


	//将读取到的负载数据写入结构体  包含掩码解密过程(如果需要)
	frame=frame.setPayload(payload)


	//判断消息类型
	switch frame.OpCode {
	case opCodeText, opCodeBinary, opCodeContinuation:
		//todo 可以做个心跳机制
	case opCodePing:
		err = c.replyPing(frame)
	case opCodePong:
		err = c.replyPong(frame)
	case opCodeClose:
		err = c.handleClose(frame)
	default:
		return nil, errors.New("unsupported frame messageType")
	}

	return frame,nil
}

//sendFrame 发送单个数据帧
func (c *Conn)sendFrame(frame *Frame)error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.Connect() {
		return errors.New("the current connection has been disconnected")
	}

	data := encodeFrameTo(frame)

	//fmt.Println(len(data))
	//当数据长度大于缓冲长度时，如果数据特别大，则会跳过缓冲区copy环节，直接写入。
	_, err := c.bufW.Write(data)
	if err != nil {
		return err
	}
	//如果数据较小则拷贝到缓冲队列,Flush写入内存
	return c.bufW.Flush()
}

//writeDataframe 发送数据帧支持 text,binary 两种格式
func(c *Conn)writeDataframe(data []byte,mt MessageType)error {
	//数据内容过大 分多个数据帧发送
	if len(data) > c.readBufferSize {
		frames := fragmentDataFrames(data, !c.isServer, OpCode(mt), c.readBufferSize)

		if frames==nil{
			return errors.New("fragmentDataFrames construct nil")
		}
		for _, frame := range frames {
			if err := c.sendFrame(frame); err != nil {
				return err
			}
		}
		return nil
	}

	//数据较小仅封装一次
	frame := constructDataFrame( data,!c.isServer,OpCode(mt))
	if err := c.sendFrame(frame); err != nil {
		return err
	}
	return nil
}

//writeControlFrame 发送控制帧 	PING|PONG|CLOSE
func (c *Conn)writeControlFrame(code OpCode,data []byte)error{
	frame:=constructControlFrame(code,!c.isServer,data)
	return c.sendFrame(frame)
}

//读取消息
func(c *Conn)ReadMessage()(mt MessageType,data []byte,err error) {
	if !c.Connect() {
		return 0, nil, errors.New("conn is not connecting")
	}
	frame, err := c.readFrame()
	if err != nil {
		return NoFrame, nil, err
	}
	mt = MessageType(frame.OpCode)

	//建立缓冲区读取数据
	buf := bytes.NewBuffer(nil)
	buf.Write(frame.Payload)

	//判断是不是最后一个数据分片
	for !frame.isFinal() {
		frame, err = c.readFrame() //Fix err
		if err != nil {
			return NoFrame, nil, err
		}

		buf.Write(frame.Payload)
	}

	data = buf.Bytes()

	frame.free() //放回对象池
	return mt, data, nil
}

//WriteMessage 支持text,binary
func (c *Conn)WriteMessage(mt MessageType,data []byte) error{
	if !c.Connect(){
		return errors.New("conn is not connecting")
	}
	if mt!=TextMessage&&mt!=BinaryMessage{
		return errors.New("only TextMessage type are supported")
	}
	return c.writeDataframe(data,mt)
}

func (c *Conn)SendFile(r io.Reader)error{
	if !c.Connect(){
		return errors.New("conn is not connecting")
	}
	data,err:=ioutil.ReadAll(r)
	if err!=nil{
		return err
	}
	return c.writeDataframe(data,BinaryMessage)
}

func (c *Conn)AcceptFile(filepath string)error{
	fd, err:= os.OpenFile(filepath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err!=nil{
		return err
	}
	return c.acceptFile(fd)
}

func (c *Conn)acceptFile(fd *os.File)error{
	frame, err := c.readFrame()
	if err != nil {
		return  err
	}
	mt := MessageType(frame.OpCode)
	if mt!=BinaryMessage{
		return errors.New("messageType is ont BinaryMessage")
	}

	_,err=fd.Write(frame.Payload)
	if err!=nil{
		return err
	}

	//判断是不是最后一个数据分片
	for  !frame.isFinal() {
		frame, err = c.readFrame() //fix bug
		if err != nil {
			return  err
		}
		_,err=fd.Write(frame.Payload)
		if err!=nil{
			return err
		}
	}
	return nil
}


func (c *Conn) handleClose(frm *Frame) error {
	var err = &CloseError{
		Code: CloseNormalClosure,
	}

	if frm.PayloadLen >= 2 {
		code := binary.BigEndian.Uint16(frm.Payload[:2])
		message := frm.Payload[2:]
		err.Code = int(code)
		err.Text = string(message)
	}
	fmt.Printf("c.handleClose got a frame with closeError=%v", err)

	c.close(err.Code)
	return err
}

// Ping conn send a ping packet to another side.
func (c *Conn) Ping() (err error) {
	return c.writeControlFrame(opCodePing, []byte("ping"))
}

// replyPing work for Conn to reply ping packet. frame MUST contains 125 Byte or-
// less payload.
func (c *Conn) replyPing(frm *Frame) (err error) {
	return c.pong(frm.Payload)
}

// pong .
func (c *Conn) pong(pingPayload []byte) (err error) {
	return c.writeControlFrame(opCodePong, pingPayload)
}

// replyPong frame MUST contains same payload with PING frame payload
func (c *Conn) replyPong(frm *Frame) (err error) {
	// if receive pong frame, try to call pongHandler
	if c.pongHandler != nil {
		c.pongHandler(string(frm.Payload))
	}

	return nil
}

// SetPongHandler handler would be called while the Conn
func (c *Conn) SetPongHandler(handler func(s string)) {
	c.pongHandler = handler
}

// Close .
func (c *Conn) Close() {
	c.State = Closing
	if err := c.close(CloseAbnormalClosure); err != nil {
		fmt.Printf("Conn.Close failed to close, err=%v", err)
	}
}

func (c *Conn) close(closeCode int) error {
	p := make([]byte, 2, 16)
	closeErr := &CloseError{Code: closeCode}
	binary.BigEndian.PutUint16(p[:2], uint16(closeCode))
	p = append(p, []byte(closeErr.Error())...)


	if err := c.writeControlFrame(opCodeClose, p); err != nil {
		return err
	}

	if c.conn != nil {
		// close underlying TCP connection
		defer func() { _ = c.conn.Close() }()
	}
	// update Conn's State to 'Closed'
	c.State = Closed
	return nil
}


func (c *Conn) validMask(frm *Frame) error {
	if c.isServer {
		// 接受客户端发送来的数据帧 -> 需要掩码
		if frm.Mask != 1 {
			return ErrMaskNotSet
		}
	} else {
		// 接受服务端发送来的数据帧 -> 不需要掩码
		if frm.Mask != 0 {
			return ErrMaskSet
		}
	}
	return nil
}

//Connect 判断当前是否在连接中 是则返回true
func (c *Conn)Connect()bool{
	return c.State==Connected
}

func (c *Conn)RemoteAddr()net.Addr{
	return c.conn.RemoteAddr()
}

func (c *Conn)LocalAddr()net.Addr{
	return c.conn.LocalAddr()
}

func(c *Conn)SetWriteDeadline(t time.Time)error{
	return c.conn.SetWriteDeadline(t)
}

func(c *Conn)SetReadDeadline(t time.Time)error {
	return c.conn.SetReadDeadline(t)
}

func(c *Conn)SetDeadline(t time.Time)error {
	return c.conn.SetDeadline(t)
}

