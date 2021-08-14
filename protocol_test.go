package ants

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func Test_genMasks(t *testing.T) {
	type args struct {
		maskingKey uint32
	}
	tests := []struct {
		name string
		args args
		want [4]byte
	}{
		{
			name: "test1",
			args: args{maskingKey: 0x9acb0442},
			want: [4]byte{0x9a, 0xcb, 0x04, 0x42},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genMasks(tt.args.maskingKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("genMasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_constructDataFrame(t *testing.T) {
	type args struct {
		payload []byte
		hasMask bool
		opcode  OpCode
	}
	tests := []struct {
		name string
		args args
		want *Frame
	}{
		{
			name: "test1",
			args: args{opcode: opCodeText, hasMask: false, payload: []byte("LaLaLa")},
			want: &Frame{OpCode: opCodeText, Fin: 1, Mask: 0, Payload: []byte("LaLaLa")},
		},
		{
			name: "test2",
			args: args{opcode: opCodeContinuation, hasMask: false, payload: []byte("HaHaHa")},
			want: &Frame{OpCode: opCodeContinuation, Fin: 0, Mask: 0, Payload: []byte("HaHaHa")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constructDataFrame(tt.args.payload, tt.args.hasMask, tt.args.opcode)
			got=&Frame{Fin: got.Fin,Mask: got.Mask,OpCode: got.OpCode,Payload: got.Payload}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("constructDataFrame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_constructControlFrame(t *testing.T) {
	type args struct {
		opcode  OpCode
		hasMask bool
		payload []byte
	}
	tests := []struct {
		name string
		args args
		want *Frame
	}{
		{
			name: "test1",
			args: args{opcode: opCodePing, hasMask: false, payload: []byte("ping")},
			want: &Frame{OpCode: opCodePing, Fin: 1, Mask: 0, Payload: []byte("ping")},
		},
		{
			name: "test2",
			args: args{opcode: opCodeClose, hasMask: false, payload: []byte("err: xxx")},
			want: &Frame{OpCode: opCodeClose, Fin: 1, Mask: 0, Payload: []byte("err: xxx")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constructControlFrame(tt.args.opcode, tt.args.hasMask, tt.args.payload)
			got = &Frame{Fin: got.Fin, Mask: got.Mask, OpCode: got.OpCode, Payload: got.Payload}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("constructControlFrame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_constructFrame(t *testing.T) {
	type args struct {
		opcode  OpCode
		final   bool
		hasMask bool
	}
	tests := []struct {
		name string
		args args
		want *Frame
	}{
		{
			name: "test1",
			args: args{opcode: opCodeText, final: false, hasMask: false,},
			want: &Frame{Fin: 0, Mask: 0, OpCode: opCodeText},
		},
		{
			name: "test2",
			args: args{opcode: opCodeBinary, final: true, hasMask: true},
			want: &Frame{Fin: 1, Mask: 1, OpCode: opCodeBinary},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constructFrame(tt.args.opcode, tt.args.final, tt.args.hasMask)
			got = &Frame{Fin: got.Fin, Mask: got.Mask, OpCode: got.OpCode}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("constructFrame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFrame_setPayload(t *testing.T) {
	type fields struct {
		Mask             uint16
		PayloadLen       uint16
		PayloadExtendLen uint64
		Payload          []byte
	}
	type args struct {
		payload []byte
	}

	payloadTest2:=strings.Repeat("s",65535)
	payloadTest3:=strings.Repeat("s",65535+10)
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Frame
	}{
		{
			name:   "test1",
			fields: fields{Mask: 0},
			args:   args{payload: []byte{125}},
			want:   &Frame{Payload: []byte{125}, PayloadLen: 125, PayloadExtendLen: 0},
		},
		{
			name: "test2",
			fields: fields{Mask: 0},
			args: args{payload: []byte(payloadTest2)},
			want: &Frame{Payload:[]byte(payloadTest2),PayloadLen: 126,PayloadExtendLen: uint64(len(payloadTest2))},
		},
		{
			name: "test3",
			fields: fields{Mask: 0},
			args: args{payload: []byte(payloadTest3)},
			want: &Frame{Payload:[]byte(payloadTest3),PayloadLen: 127,PayloadExtendLen: uint64(len(payloadTest3))},
		},

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Frame{
				Mask:             tt.fields.Mask,
				PayloadLen:       tt.fields.PayloadLen,
				PayloadExtendLen: tt.fields.PayloadExtendLen,
				Payload:          tt.fields.Payload,
			}
			got := f.setPayload(tt.args.payload)
			got.Payload = nil
			tt.want.Payload = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setPayload() = %v, want %v", got, tt.want)
			}
			t.Logf("setPayload() = %v, want %v", got, tt.want)
		})
	}
}

func Test_fragmentDataFrames(t *testing.T) {
	type args struct {
		data      []byte
		hasMask   bool
		opcode    OpCode
		frameSize int
	}

	dataTest1 := make([]byte, 0, 65535*2+20)
	part1 := strings.Repeat("a", 65535)
	part2 := strings.Repeat("b", 65535)
	part3 := strings.Repeat("c", 20)

	dataTest1 = append(dataTest1, part1...)
	dataTest1 = append(dataTest1, part2...)
	dataTest1 = append(dataTest1, part3...)

	tests := []struct {
		name string
		args args
		want []*Frame
	}{
		{name: "test1",
			args: args{data: dataTest1, hasMask: false, opcode: opCodeText, frameSize: 65535},
			want:[]*Frame{{Fin: 0,OpCode: opCodeText,Payload: []byte(part1)},
				{Fin: 0,OpCode: opCodeContinuation,Payload: []byte(part2)},
				{Fin: 1,OpCode: opCodeContinuation,Payload: []byte(part3)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			 gots := fragmentDataFrames(tt.args.data, tt.args.hasMask, tt.args.opcode, tt.args.frameSize)
			 for k,got:=range gots {
			 	got=&Frame{Fin: got.Fin,OpCode: got.OpCode,Payload: got.Payload}
				 if !reflect.DeepEqual(got, tt.want[k]) {
					 t.Errorf("fragmentDataFrames() = %v, want %v", got, tt.want)
				 }
			 }
		})
	}
}

func Test_encodeFrameTo(t *testing.T) {
	type args struct {
		f *Frame
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "test1",
			args: args{f: &Frame{
				Fin: 1, RSV1: 0, RSV2: 0, RSV3: 0, OpCode: opCodeText,
				Mask: 0, PayloadLen: 0,
				Payload: []byte(""), PayloadExtendLen: 0}},

				//1 0 0 0 0 0 0 1       first byte
				//0 0 0 0 0 0 0 0       second byte
			want:[]byte{129,0},
		},

		{
			name: "test2",
			args: args{f: &Frame{
				Fin: 0, RSV1: 0, RSV2: 0, RSV3: 0, OpCode: opCodeBinary,
				Mask: 0, PayloadLen: 126,
				PayloadExtendLen: 2, Payload: []byte{16, 32}},  //PayloadExtendLen=2 byte
			},

			//0 0 0 0 0 0 1 0       first byte     fin(1 bit) + rsv1(1 bit) + rsv2(1 bit) + rsv3(1 bit) + opCode(4 bit)
			//0 1 1 1 1 1 1 0       second byte    mask(1 bit) + PayloadLen(7 bit)
			//0 0 0 0 0 0 0 0 0 0 0 0 0 0 1 0      PayloadExtendLen(16 bit)
			//0 0 0 1 0 0 0 0       Payload
			//0 0 1 0 0 0 0 0       Payload
			want:[]byte{2,126,0,2,16,32},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeFrameTo(tt.args.f); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("encodeFrameTo() = %v, want %v", got, tt.want)
			}
		})
	}
}


func Test_parseFrameHeader(t *testing.T) {
	type args struct {
		header []byte
	}
	tests := []struct {
		name string
		args args
		want *Frame
	}{
		{
			name: "test1",
			//1 0 0 0 1 0 0 0       first byte     fin(1 bit) + rsv1(1 bit) + rsv2(1 bit) + rsv3(1 bit) + opCode(4 bit)
			//0 1 1 1 1 1 1 0       second byte    mask(1 bit) + PayloadLen(7 bit)
			args:args{header: []byte{136,126}},
			want: &Frame{
			Fin: 1, RSV1: 0, RSV2: 0, RSV3: 0, OpCode: opCodeClose,
			Mask: 0, PayloadLen: 126},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFrameHeader(tt.args.header); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseFrameHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFrame_autoCalcPayloadLen(t *testing.T) {
	type fields struct {
		PayloadLen       uint16
		PayloadExtendLen uint64
		Payload          []byte
	}

	data1 := strings.Repeat("a", 65535+10)
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "test1",
			fields:fields{Payload: []byte{125}},
		},
		{
			name: "test2",
			fields:fields{Payload: []byte{126}},
		},
		{
			name: "test3",
			fields:fields{Payload: []byte{127}},
		},
		{
			name: "test4",
			fields:fields{Payload: []byte{128}},
		},
		{
			name: "test5",
			fields:fields{Payload: []byte(data1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Frame{
				PayloadLen:       tt.fields.PayloadLen,
				PayloadExtendLen: tt.fields.PayloadExtendLen,
				Payload:          tt.fields.Payload,
			}
			f.autoCalcPayloadLen()
			fmt.Println(f.PayloadLen,f.PayloadExtendLen)
		})
	}
}

func TestFrame_setPayload1(t *testing.T) {
	type field struct {
		Mask             uint16
		MaskingKey       uint32
		Payload          []byte
		PayloadExtendLen uint64
		PayloadLen       uint16
	}
	type args struct {
		payload []byte
	}
	tests := []struct {
		name   string
		fields *field
		args   args
		want   Frame
	}{
		{name: "test1",
			fields: &field{Mask: 1,MaskingKey:rand.Uint32()},
			args:   args{payload: []byte("payload")},
			want:   Frame{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Frame{
				Mask:             tt.fields.Mask,
				MaskingKey:       tt.fields.MaskingKey,
				Payload:          tt.fields.Payload,
				PayloadExtendLen: tt.fields.PayloadExtendLen,
				PayloadLen:       tt.fields.PayloadLen,
			}
			got := f.setPayload(tt.args.payload)
			t.Logf("setPayload() = %v", got)
			t.Log("加密后payload:", string(got.Payload))
		})
	}
}