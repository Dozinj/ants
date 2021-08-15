package ants

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

)

//Sec-WebSocket-Version can not be less than 13
var DefaultWebsocketVersion = "13"

type Dialer struct {
	//websocket 子协议
	subProtocols     []string
	Timeout time.Duration
}

var DefaultDialer =&Dialer{
	subProtocols: []string{"chat"},
	Timeout: 10*time.Second,
}

//Dial 完成http升级为websocket协议握手阶段
func (d *Dialer)Dial(urlStr string)(*Conn,*http.Response,error) {
	return d.DialWithContext(context.Background(), urlStr)
}

//DialWithContext 完成http升级为websocket协议握手阶段(支持上下文形式)
func (d *Dialer)DialWithContext(ctx context.Context,URL string)(*Conn,*http.Response,error) {
	//限定握手时间
	if d.Timeout != 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, d.Timeout)
		defer cancel()
	}

	//解析url
	op, err := parseUrl(URL)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, nil, err
	}

	//客户端随机生成16字节握手密钥
	secKey, err := generateChallengeKey()
	if err != nil {
		return nil, nil, err
	}

	req.Header["Upgrade"] = []string{"websocket"}
	req.Header["Connection"] = []string{"Upgrade"}
	req.Header["Sec-WebSocket-Key"] = []string{secKey}
	req.Header["Sec-WebSocket-Version"] = []string{DefaultWebsocketVersion}
	req.Header["Sec-WebSocket-Protocol"] = d.subProtocols
	if len(d.subProtocols) > 1 { //请求头多个字段之间用`,`隔开
		req.Header["Sec-WebSocket-Protocol"] = []string{strings.Join(d.subProtocols, ",")}
	}

	//建立tcp连接
	netConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", op.host, op.port))
	if err != nil {
		return nil, nil, err
	}

	//封装netConn
	conn := newConn(netConn, false)

	//Write 以wire格式写入 HTTP/1.1 请求，即标头和正文。
	if err := req.WithContext(ctx).Write(conn.bufW); err != nil {
		return nil, nil, err
	}

	//清空缓冲区
	_ = conn.bufW.Flush()

	resp, err := http.ReadResponse(conn.bufR, req)
	if err != nil {
		return nil, nil, err
	}

	if err = checkRespHand(resp, secKey); err != nil {
		return nil, resp, err
	}

	//更新连接状态
	conn.State = Connected
	return conn, resp, nil
}

//checkHand 检验握手结果
func checkRespHand(resp *http.Response,secKey string)error {
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("invalid statusCode=%d", resp.StatusCode)
	}

	if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") {
		return fmt.Errorf("invalid Upgrade=%s", resp.Header.Get("Upgrade"))
	}

	if !strings.EqualFold(resp.Header.Get("Connection"), "Upgrade") {
		return fmt.Errorf("invalid Connection=%s", resp.Header.Get("Connection"))
	}

	enSecKey := encryptionkey(secKey)
	if enSecKey != resp.Header.Get("Sec-WebSocket-Accept") {
		return errors.New("encryption key comparison failed")
	}

	return nil
}


type options struct {
	host string

	port string

	path string

	rawQuery string

	scheme string
}

func parseUrl(URL string)(*options,error) {
	u, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}

	op := &options{
		host:     u.Hostname(),
		port:     u.Port(),
		path:     u.Path,
		rawQuery: u.RawQuery,
		scheme:   u.Scheme,
	}

	//握手阶段采用http 协议
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return nil, errors.New("only support ws or wss url scheme")
	}

	//补全默认省略端口
	if op.port == "" {
		switch u.Scheme {
		case "http":
			op.port = "80"
		case "https":
			op.port = "443"
		}
	}
	return op, nil
}

