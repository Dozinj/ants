
package ants

import (
	"encoding/binary"
	"math/rand"
	"strconv"
	"sync"
)

const (
	//状态码:  当收到一个关闭帧的时候，可能附带关闭的状态码
	CloseNormalClosure           = 1000   //正常关闭; 无论为何目的而创建, 该链接都已成功完成任务.
	CloseGoingAway               = 1001	  //终端离开, 可能因为服务端错误, 也可能因为浏览器正从打开连接的页面跳转离开
	CloseProtocolError           = 1002	  //由于协议错误而中断连接
	CloseUnsupportedData         = 1003	  //由于接收到不允许的数据类型而断开连接 (如仅接收文本数据的终端接收到了二进制数据).
	CloseNoStatusReceived        = 1005	  // 表示没有收到预期的状态码.
	CloseAbnormalClosure         = 1006   //用于期望收到状态码时连接非正常关闭 (也就是说, 没有发送关闭帧).
	CloseInvalidFramePayloadData = 1007   //由于收到了格式不符的数据而断开连接 (如文本消息中包含了非 UTF-8 数据)
	ClosePolicyViolation         = 1008   //由于收到不符合约定的数据而断开连接. 这是一个通用状态码, 用于不适合使用 1003 和 1009 状态码的场景.
	CloseMessageTooBig           = 1009   //由于收到过大的数据帧而断开连接.
	CloseMandatoryExtension      = 1010   //客户端期望服务器商定一个或多个拓展, 但服务器没有处理, 因此客户端断开连接
	CloseInternalServerErr       = 1011   //客户端由于遇到没有预料的情况阻止其完成请求, 因此服务端断开连接.
	CloseServiceRestart          = 1012   //服务器由于重启而断开连接.
	CloseTryAgainLater           = 1013   //服务器由于临时原因断开连接, 如服务器过载因此断开一部分客户端连接.
	CloseTLSHandshake            = 1015   //表示连接由于无法完成 TLS 握手而关闭 (例如无法验证服务器证书).
)

// CloseError .
type CloseError struct {
	Code int
	Text string
}

func (e *CloseError) Error() string {
	s := []byte("websocket: close ")
	s = strconv.AppendInt(s, int64(e.Code), 10)
	switch e.Code {
	case CloseNormalClosure:
		s = append(s, " (normal)"...)
	case CloseGoingAway:
		s = append(s, " (going away)"...)
	case CloseProtocolError:
		s = append(s, " (protocol error)"...)
	case CloseUnsupportedData:
		s = append(s, " (unsupported data)"...)
	case CloseNoStatusReceived:
		s = append(s, " (no status)"...)
	case CloseAbnormalClosure:
		s = append(s, " (abnormal closure)"...)
	case CloseInvalidFramePayloadData:
		s = append(s, " (invalid payload data)"...)
	case ClosePolicyViolation:
		s = append(s, " (policy violation)"...)
	case CloseMessageTooBig:
		s = append(s, " (message too big)"...)
	case CloseMandatoryExtension:
		s = append(s, " (mandatory extension missing)"...)
	case CloseInternalServerErr:
		s = append(s, " (internal chat_server error)"...)
	case CloseTLSHandshake:
		s = append(s, " (TLS handshake error)"...)
	}
	if e.Text != "" {
		s = append(s, ": "...)
		s = append(s, e.Text...)
	}
	return string(s)
}

var(
	ErrInvalidFrame = &CloseError{Code: CloseProtocolError, Text: "invalid f: "}
)

// OpCode (4bit) 决定如何解析有效载荷数据
type OpCode uint16

const (
	// 0x0 继续帧，表示消息分片模式 表示当前分片消息不是消息的第一个分片，是一个继续分片
	// 也就是说一条消息有两个或两个以上分片，才能出现Opcode=0的情况。
	opCodeContinuation OpCode = 0

	//消息帧
	opCodeText         OpCode = 1  // 0x1 文本帧，表示文本格式传输
	opCodeBinary       OpCode = 2  // 0x2 表示二进制格式传输

	//控制帧
	opCodeClose        OpCode = 8  // 0x8 关闭帧，表示关闭连接
	opCodePing         OpCode = 9  // 0x9 Ping帧，一般主动发送ping给对方，确认对方状态
	opCodePong         OpCode = 10 // 0xA Pong帧，一般发送了ping给对方,对方就回复pong
)

const (
	finBitLen        = 1
	rsv1BitLen       = 1
	rsv2BitLen       = 1
	rsv3BitLen       = 1
	opcodeBitLen     = 4
	maskBitLen       = 1
	payloadLenBitLen = 7
	// payloadExtendLenBitLen = 0 or 16 or  64
	maskingKeyBitLen = 32

	// headerSize = 3*4B + 2B = 14B = 112 bit (max)
	// headerSize = finBitLen + rsv1BitLen + rsv2BitLen + rsv3BitLen + opcodeBitLen
	//+ maskBitLen + payloadLenBitLen + payloadExtendLenBitLen(16/64) + maskingKeyBitLen

	finOffset        = 15 // 1st bit
	rsv1Offset       = 14 // 2nd bit
	rsv2Offset       = 13 // 3rd bit
	rsv3Offset       = 12 // 4th bit
	opcodeOffset     = 8  // 5th-8th bits
	maskOffset       = 7  // 9th bit
	payloadLenOffset = 0  // 10th - 16th bits

	finMask        = 0x8000 // 1000 0000 0000 0000
	rsv1Mask       = 0x4000 // 0100 0000 0000 0000
	rsv2Mask       = 0x2000 // 0010 0000 0000 0000
	rsv3Mask       = 0x1000 // 0001 0000 0000 0000
	opcodeMask     = 0x0F00 // 0000 1111 0000 0000
	maskMask       = 0x0080 // 0000 0000 1000 0000
	payloadLenMask = 0x007F // 0000 0000 0111 1111
)


//Frame 创建一个包含 WebSocket 基本帧数据的结构，并帮助通过 TCP 字节流组装和读取数据
type Frame struct {
	//FIN表示帧结束   1 bit
	//如果是0，表示这不是消息的最后一个分片。
	//如果是1，表示这是消息的最后一个分片。
	Fin    uint16


	//RSV是预留的空间，正常为0 ,1 bit
	//如果收到一个非0值且没有协商的扩展定义这个非0值的含义，接收端就必须断开连接。
	RSV1   uint16
	RSV2   uint16
	RSV3   uint16

	//opcode是标识数据类型的 4 bits
	OpCode OpCode


	//MASK标识这个数据帧的数据是否使用掩码 1 bit
	//根据websocket定义：
	//客户端发送数据需要进行掩码处理，接收数据无需反掩码操作
	//服务端发送数据无需进行掩码处理，接收数据需要反掩码操作
	Mask   uint16

	// Payload length:  7 bits, 7+16 bits, or 7+64 bits
	//PayloadLen表示数据部分的长度。但是PayloadLen只有7位，换成无符号整型的话只有0到127的取值，
	//这么小的数值当然无法描述较大的数据，因此规定当数据长度小于或等于125时候它才作为数据长度的描述，
	//如果 0-125，这是负载长度。
	//如果 126，之后的2个字节（16 位）表示负载数据长度。
	//如果 127，之后的8个字节（64位）表示负载数据长度。
	PayloadLen       uint16 // 7 bits
	PayloadExtendLen uint64 // 64 bits


	//MaskingKey由4个随机字节组成，储存掩码的实体部分。
	//但是只有在前面的MASK被设置为1时候才存在这个数据
	MaskingKey uint32 // 32 bits

	//数据部分，如果掩码存在，那么所有数据都需要与掩码做一次异或运算，
	Payload    []byte 
}

//autoCalcPayloadLen 要确定负载数据长度，首先先判断第一个字节的值，
//如果>0且<125,那么这个值就是负载数据的长度，为0时候，就代表负载数据长度为0，也就是不包含负载数据。
//这里我用payloadExtendLen 表示data 字节长度
func (f *Frame) autoCalcPayloadLen() {

	var (
		payloadLen       uint16
		payloadExtendLen uint64
	)
	length:=uint64(len(f.Payload))

	// 设置有效载荷长度和有效载荷扩展长度
	if length==1 &&f.Payload[0]<=125 { //0-125
		payloadLen = uint16(f.Payload[0])
		payloadExtendLen = 0
	}else if length<(1<<16){ //后两字节
		payloadLen=126
		payloadExtendLen=uint64(len(f.Payload))
	}else  {
		payloadLen=127
		payloadExtendLen=uint64(len(f.Payload))
	}


	f.PayloadLen = payloadLen
	f.PayloadExtendLen = payloadExtendLen
}

// genMaskingKey 生成4个字节随机大小的掩码
func (f *Frame) genMaskingKey() {
	f.MaskingKey = rand.Uint32()
}

// setPayload  自动屏蔽或取消屏蔽有效载荷数据
func (f *Frame) setPayload(payload []byte) *Frame {
	f.Payload = make([]byte, len(payload))
	copy(f.Payload, payload)


	if f.Mask == 1 {
		//如果已设置掩码，则计算带有有效载荷的掩码密钥
		f.maskPayload()
	}

	//通过实际读取到的payload  确定payload拓展长度
	f.autoCalcPayloadLen()
	return f
}

//genMasks 将4字节的maskKey拆分为长度为4的byte切片,
//将每个字节隔离开来，便于接下来的单位字节操作
func genMasks(maskingKey uint32) [4]byte {
	//？ 应该最左边开始是第一字节
	return [4]byte{
		byte((maskingKey >> 24)& 0x00FF),
		byte((maskingKey >> 16)& 0x00FF),
		byte((maskingKey >> 8) & 0x00FF),
		byte((maskingKey >> 0) & 0x00FF),
	}
}

//maskPayload 对负载数据进行掩码操作
//original-octet-i：为原始数据的第i字节  transformed-octet-i：为转换后的数据的第i字节。
//j：为i mod 4的结果。
//masking-key-octet-j：为mask key第j字节。
//算法描述为： original-octet-i 与 masking-key-octet-j 异或后，得到 transformed-octet-i。
//j = i MOD 4
//transformed-octet-i = original-octet-i XOR masking-key-octet-j
//XOR :如果a、b两个值不相同，则异或结果为1。如果a、b两个值相同，异或结果为0。
func (f *Frame) maskPayload() {
	masks := genMasks(f.MaskingKey)
	for i, v := range f.Payload {
		j := i % 4
		f.Payload[i] = v ^ masks[j]
	}
}

// isFinal .
func (f *Frame) isFinal() bool {
	return f.Fin == 1
}

func (f *Frame) valid() error {
	var err = ErrInvalidFrame
	//这里暂时不支持协议拓展
	if f.RSV1 != 0 || f.RSV2 != 0 || f.RSV3 != 0 {
		err.Text += "reserved bit is not 0"
		return err
	}

	if f.Mask == 1 && f.MaskingKey == 0 {
		err.Text += "mask value is not set"
		return err
	}
	return nil
}


// encodeFrameTo 将数据帧结构体头部序列化为[]byte
func encodeFrameTo(f *Frame) []byte {
	buf := make([]byte, 2, minFrameHeaderSize+8)

	var (
		part1 uint16
	)
	//先写入前两字节数据
	//通过位运算确定数据帧byte中的每个bit
	part1 |= f.Fin << finOffset
	part1 |= f.RSV1 << rsv1Offset

	part1 |= f.RSV2 << rsv2Offset
	part1 |= f.RSV3 << rsv3Offset
	part1 |= uint16(f.OpCode) << opcodeOffset //位运算需换成内置类型
	part1 |= f.Mask << maskOffset
	part1 |= f.PayloadLen << payloadLenOffset

	//byte 将uint16 写入[]byte中
	binary.BigEndian.PutUint16(buf[:2], part1)

	//append payloadExtendLen
	switch f.PayloadLen {
	case 126:
		payloadExtendBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(payloadExtendBuf[:2], uint16(f.PayloadExtendLen))
		buf = append(buf, payloadExtendBuf...)
	case 127:
		payloadExtendBuf := make([]byte, 8)
		binary.BigEndian.PutUint64(payloadExtendBuf[:8], f.PayloadExtendLen)
		buf = append(buf, payloadExtendBuf...)
	}

	//如果掩码存在需要多加4个byte的掩码
	if f.Mask == 1 {
		maskingKeyBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(maskingKeyBuf[:4], f.MaskingKey)
		buf = append(buf, maskingKeyBuf...)
	}

	// 协议头完成，准备填写负载数据
	buf = append(buf, f.Payload...)

	//将frame 放回对象池中
	f.free()
	return buf
}

const (
	// 2B(header) + 4B(maskingKey) = 6B
	minFrameHeaderSize = (finBitLen + rsv1BitLen + rsv2BitLen +
		rsv3BitLen + opcodeBitLen + maskBitLen +
		payloadLenBitLen + maskingKeyBitLen) / 8
)


// parseFrameHeader .head 2个字节,每个bit都有意义
// 通过位运算将数据帧头部反序列化到frame结构体中
func parseFrameHeader(header []byte) *Frame {
	var (
		f   = newFrame() //从对象池中获取
		part1 = binary.BigEndian.Uint16(header[:2])
	)

	f.Fin = (part1 & finMask) >> finOffset
	f.RSV1 = (part1 & rsv1Mask) >> rsv1Offset
	f.RSV2 = (part1 & rsv2Mask) >> rsv2Offset
	f.RSV3 = (part1 & rsv3Mask) >> rsv3Offset
	f.OpCode = OpCode((part1 & opcodeMask) >> opcodeOffset)
	f.Mask = (part1 & maskMask) >> maskOffset
	f.PayloadLen = (part1 & payloadLenMask) >> payloadLenOffset

	return f
}

// fragmentDataFrames 将data拆分为多个数据帧
func fragmentDataFrames(data []byte, hasMask bool, opcode OpCode,frameSize int) []*Frame {
	if frameSize==0{
		return nil
	}
	length := len(data)
	start, end, n := 0, 0, length/frameSize

	frames := make([]*Frame, 0, n+1)
	for i := 1; i <=n; i++ {
		start, end = (i-1)*frameSize, i*frameSize
		f:=constructDataFrame(data[start:end],hasMask, opCodeContinuation)
		frames = append(frames, f)
	}

	if end < length {
		frames = append(frames, constructDataFrame(data[end:], hasMask, opCodeContinuation))
	}

	frames[0].OpCode = opcode
	//将最后一帧的FIN 设为1
	frames[len(frames)-1].Fin=1

	return frames
}

// constructDataFrame 负载数据长度由conn限制,
func constructDataFrame(payload []byte, hasMask bool, opcode OpCode) *Frame {
	//如果操作码为opCodeContinuation，则表示此帧不是最终帧
	final := opcode != opCodeContinuation
	f := constructFrame(opcode, final, hasMask)

	f.setPayload(payload)
	return f
}


func constructControlFrame(opcode OpCode, hasMask bool, payload []byte) *Frame {
	f := constructFrame(opcode, true, hasMask)
	if len(payload) != 0 {
		f.setPayload(payload)
	}
	return f
}


var FramePool = sync.Pool{
	New: func() interface{} { return new(Frame) },
}

func newFrame() *Frame {
	f := FramePool.Get().(*Frame)
	return f
}

func (f *Frame) free() {
	*f=Frame{}
	FramePool.Put(f)
}

//constructFrame 封装一个数据帧格式
func constructFrame(opcode OpCode, final bool, hasMask bool) *Frame {
	f := newFrame()
	if !final {
		f.Fin = 0
	} else {
		f.Fin = 1
	}

	if !hasMask {
		f.Mask = 0
	} else {
		f.Mask = 1
		//进行掩码处理
		f.genMaskingKey()
	}

	f.OpCode = opcode

	return f
}
