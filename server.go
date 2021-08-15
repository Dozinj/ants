package ants

import (
	"context"
	"errors"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type HandshakeError struct {
	Text string
}

func (h HandshakeError)Error()string{return h.Text}

func newHandshakeError(reason string) HandshakeError {
	return HandshakeError{Text: "websocket: the client request does not meet the upgrade specification: " + reason}
}

// returnError . 将错误写入 HTTP 响应并将错误返回给 http.Handler
func (u *Upgrader) returnError(w http.ResponseWriter, statusCode int, reason string) error {
	err := errors.New(reason)
	http.Error(w, reason, statusCode)
	// w.WriteHeader(statusCode)
	// w.Write([]byte(reason))
	return err
}

type Upgrader struct {
	//握手时间
	Timeout time.Duration

	//websocket 子协议
	SubProtocols  []string

	//跨域请求
	CheckOrigin func(*http.Request)bool
}

var DefaultUpgrader =&Upgrader{
	CheckOrigin: func(request *http.Request) bool {
		return true
	},
	Timeout: defaultUpgradeTimeout,
	SubProtocols: []string{"chat"},
}

const defaultUpgradeTimeout = 10 * time.Second


//selectSubProtocol 服务器选择自身支持的websocket子协议
func (u *Upgrader)selectSubProtocol(req *http.Request)string{
	for _,reqV:=range req.Header["Sec-WebSocket-Protocol"]{
		for _,respV:=range u.SubProtocols{
			if reqV==respV{
				return respV
			}
		}
	}
	return ""
}

//Upgrade http升级为websocket
func (u *Upgrader)Upgrade(w http.ResponseWriter, req *http.Request,fn func(conn *Conn)) error {
	if u.Timeout == 0 {
		u.Timeout = defaultUpgradeTimeout
	}
	if u.SubProtocols == nil {
		u.SubProtocols = []string{"chat"}
	}

	ctx, cancel := context.WithTimeout(req.Context(), u.Timeout)
	defer cancel()
	req = req.WithContext(ctx)

	if status, reason := checkReqHand(req); reason != "" {
		return u.returnError(w, status, newHandshakeError(reason).Error())
	}

	protocol := u.SubProtocols[0]
	if u.CheckOrigin == nil {
		u.CheckOrigin = func(req *http.Request) bool {
			if len(req.Header["Origin"]) == 0 {
				return true
			}
			return req.Header["Origin"][0] == req.URL.Host
		}
	}
	if !u.CheckOrigin(req) {
		return u.returnError(w, http.StatusBadRequest, newHandshakeError("only supports get requests method)").Error())
	}

	//在HTTP1.X中，一个请求和回复对应在一个tcp连接上，在websocket握手结束后，该tcp链接升级为websocket协议。
	//而在HTTP/2中，多个请求和回复会复用一个tcp链接，无法实现上述的过程。
	//其会在握手阶段将http.ResponseWriter断言为http.Hijacker接口并调用其中的Hijack()方法，拿到原始tcp链接对象并进行接管。
	//而在使用HTTP/2时，http.ResponseWriter无法断言为http.Hijacker
	//调用 Hijack 后，原来的 Request.Body 应该不被使用。
	// Golang 的内置 HTTP 库和 HTTPServer 库将不会管理这个 Socket 连接的生命周期，
	//这个生命周期已经划给 Hijacker 了，在 Hijacker 的代码注释里面是这么描述的
	//Hijacker 接口由 ResponseWriters 实现，允许 HTTP 处理程序接管连接。
	h, ok := w.(http.Hijacker)
	if !ok {
		return u.returnError(w, http.StatusInternalServerError, "http hijacker failed")
	}

	//管理和关闭连接成为调用者的责任。
	netConn, brw, err := h.Hijack()
	if err != nil {
		return u.returnError(w, http.StatusInternalServerError, err.Error())
	}

	if brw.Reader.Buffered() > 0 {
		//返回的 bufio.Reader 可能包含来自客户端的未处理的缓冲数据。
		//握手期间不能传输数据
		netConn.Close()
		return errors.New("websocket: chat_client sent data before handshake is complete")
	}

	//Hijack之后不能再对w http.responsewriter里面的w写入数据；
	secKey := req.Header.Get("Sec-WebSocket-Key")
	p := make([]byte, 0, 1024)
	p = append(p, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "...)
	p = append(p, encryptionkey(secKey)...)
	p = append(p, "\r\nSec-WebSocket-Protocol: "...)
	p = append(p, protocol...)
	p = append(p, "\r\n\r\n"...) //请求头与请求体之间需要空一行

	if _, err = netConn.Write(p); err != nil {
		netConn.Close()
		return err
	}

	conn := newConn(netConn, true)
	conn.State = Connected


	go func() {
		defer func() {
			//一旦某一个协程发生了panic而没有被捕获，那么导致整个go程序都会终止
			if err, ok := recover().(error); ok {
				log.Println(err)
				debug.PrintStack()
			}
		}()

		fn(conn)
	}()
	return nil
}

func checkHeader( req *http.Request,key,value string)bool {
	if len(req.Header[key]) != 1 {
		return false
	}
	return strings.EqualFold(req.Header[key][0], value)
}

func checkReqHand(req *http.Request) (status int, reason string) {
	//验证请求方法
	if req.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, "not get method"
	}
	//检查请求头信息
	if !checkHeader(req, "Upgrade", "websocket") {
		return http.StatusBadRequest, "invalid Upgrade field which should be websocket"
	}

	if !checkHeader(req, "Connection", "Upgrade") {
		return http.StatusBadRequest, "invalid Connection field which should be Upgrade "
	}

	if !checkHeader(req, "Sec-Websocket-Version", DefaultWebsocketVersion) {
		return http.StatusBadRequest, "invalid Sec-WebSocket-Version field which should be 13 "
	}

	if req.Header.Get("Sec-WebSocket-Key") == "" {
		return http.StatusBadRequest, "webSocket key is not allowed to be empty"
	}
	return 0, ""
}

