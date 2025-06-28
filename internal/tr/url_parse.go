package tr

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

type endpoint struct {
	host   string // без порта
	https  bool
	port   uint16 // 0 ⇒ по умолчанию (9091)
	rpcURI string // с ведущим "/"
}

func parseTRURL(raw string) (ep endpoint, err error) {
	ep.rpcURI = "/transmission/rpc"

	// если пользователь написал просто "192.168.1.2" или "torrent.local"
	// — добавляем схему, чтобы net/url разобрал
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return
	}

	// https ?
	ep.https = strings.EqualFold(u.Scheme, "https")

	// host / port
	host, portStr, _ := net.SplitHostPort(u.Host)
	if host == "" { // порт не указан
		host = u.Host
	} else {
		if p, _ := strconv.Atoi(portStr); p > 0 {
			ep.port = uint16(p)
		}
	}
	ep.host = host

	// rpc path
	path := strings.Trim(u.Path, "/")
	if path != "" {
		ep.rpcURI = "/" + path
	}
	return
}
