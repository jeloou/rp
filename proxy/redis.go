package proxy

import (
	"bufio"
	"fmt"
	"net"

	"strconv"
	"strings"
)

type redisServer struct {
	Addr    string
	Handler func(string) (string, error)
}

func readLen(r *bufio.Reader) (int, error) {
	// check if first byte is not *
	b, _ := r.ReadByte()
	if b != 0x2a {
		return 0, nil
	}

	// we read until we get a \n
	s, _ := r.ReadString('\n')
	i := strings.Index(s, "\r")
	n, _ := strconv.Atoi(s[:i])
	return n, nil
}

func readVal(r *bufio.Reader) (string, error) {
	// check if first byte is not $
	b, _ := r.ReadByte()
	if b != 0x24 {
		return "", nil
	}

	// read the len
	s, _ := r.ReadString('\n')
	i := strings.Index(s, "\r")

	// read the val
	s, _ = r.ReadString('\n')
	i = strings.Index(s, "\r")

	return strings.ToLower(s[:i]), nil
}

func (r *redisServer) handle(conn net.Conn) {
	reader := bufio.NewReader(conn)
	l, _ := readLen(reader)

	args := make([]string, l)
	for i := 0; i < l; i++ {
		v, _ := readVal(reader)
		args[i] = v
	}

	var res string

	cmd := args[0]
	switch cmd {
	case "get":
		if l > 2 {
			res = fmt.Sprintf("-err wrong number of arguments for '%s' command\r\n", cmd)
			break
		}

		s, err := r.Handler(args[1])
		if err != nil {
			res = "$-1\r\n"
			break
		}

		res = fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
	default:
		res = fmt.Sprintf("-err unknown command '%s'\r\n", cmd)
	}

	conn.Write([]byte(res))
}

func (r *redisServer) ListenAndServe() error {
	ln, err := net.Listen("tcp", r.Addr)
	if err != nil {
		return err
	}

	for {
		conn, _ := ln.Accept()
		go r.handle(conn)
	}
}
