// Copyright 2015 Nevio Vesic
// Please check out LICENSE file for more information about what you CAN and what you CANNOT do!
// Basically in short this is a free software for you to do whatever you want to do BUT copyright must be included!
// I didn't write all of this code so you could say it's yours.
// MIT License

package zyesl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Client - In case you need to do inbound dialing against freeswitch server in order to originate call or see
// sofia statuses or whatever else you came up with
type Client struct {
	SocketConnection

	Proto   string `json:"freeswitch_protocol"`
	Addr    string `json:"freeswitch_addr"`
	Passwd  string `json:"freeswitch_password"`
	Timeout int    `json:"freeswitch_connection_timeout"`
}

// EstablishConnection - Will attempt to establish connection against freeswitch and create new SocketConnection
func (c *Client) EstablishConnection() error {
	conn, err := c.Dial(c.Proto, c.Addr, time.Duration(c.Timeout*int(time.Second)))
	if err != nil {
		return err
	}
	c.SocketConnection = SocketConnection{
		Conn: conn,
		err:  make(chan error),
		m:    make(chan *Message),
	}
	return nil
}

// Authenticate - Method used to authenticate client against freeswitch. In case of any errors durring so
// we will return error.
func (c *Client) Authenticate() error {
	m, err := NewMessage(bufio.NewReaderSize(c, ReadBufferSize), false)
	if err != nil {
		logrus.Error(ECouldNotCreateMessage, err)
		return err
	}

	cmr, err := m.tr.ReadMIMEHeader()
	if err != nil && err.Error() != "EOF" {
		logrus.Error(ECouldNotReadMIMEHeaders, err)
		return err
	}
	if cmr.Get("Content-Type") != "auth/request" {
		logrus.Error(EUnexpectedAuthHeader, cmr.Get("Content-Type"))
		return fmt.Errorf(EUnexpectedAuthHeader, cmr.Get("Content-Type"))
	}

	s := "auth " + c.Passwd + "\r\n\r\n"
	_, err = io.WriteString(c, s)
	if err != nil {
		return err
	}

	am, err := m.tr.ReadMIMEHeader()
	if err != nil && err.Error() != "EOF" {
		logrus.Error(ECouldNotReadMIMEHeaders, err)
		return err
	}

	if am.Get("Reply-Text") != "+OK accepted" {
		logrus.Error(EInvalidPassword, c.Passwd)
		return fmt.Errorf(EInvalidPassword, c.Passwd)
	}

	return nil
}

// NewClient - Will initiate new client that will establish connection and attempt to authenticate
// against connected freeswitch server
func NewClient(host string, port uint, passwd string, timeout int) (*Client, error) {
	client := Client{
		Proto:   "tcp", // Let me know if you ever need this open up lol
		Addr:    net.JoinHostPort(host, strconv.Itoa(int(port))),
		Passwd:  passwd,
		Timeout: timeout,
	}
	err := client.EstablishConnection()
	if err != nil {
		return nil, err
	}
	err = client.Authenticate()
	if err != nil {
		client.Close()
		return nil, err
	}
	return &client, nil
}

// NewClient - Will initiate new client that will establish connection and attempt to authenticate
// against connected freeswitch server
func NewUnixClient(path string) (*Client, error) {
	client := Client{
		Proto: "unix", // Let me know if you ever need this open up lol
		Addr:  path,
	}
	err := client.EstablishConnection()
	if err != nil {
		return nil, err
	}
	// err = client.Authenticate()
	// if err != nil {
	// 	client.Close()
	// 	return nil, err
	// }
	return &client, nil
}

// Main connection against ESL - Gotta add more description here
type SocketConnection struct {
	net.Conn
	err chan error
	m   chan *Message
}

// Dial - Will establish timedout dial against specified address. In this case, it will be freeswitch server
func (c *SocketConnection) Dial(network string, addr string, timeout time.Duration) (net.Conn, error) {
	if network == "unix" {
		return net.Dial("unix", addr)
	} else {
		return net.DialTimeout(network, addr, timeout)
	}
}

// Send - Will send raw message to open net connection
func (c *SocketConnection) Send(cmd string) error {
	if strings.Contains(cmd, "\r\n") {
		return fmt.Errorf(EInvalidCommandProvided, cmd)
	}
	_, err := io.WriteString(c, cmd)
	if err != nil {
		return err
	}
	_, err = io.WriteString(c, "\r\n\r\n")
	if err != nil {
		return err
	}
	return nil
}

// SendEvent - Will loop against passed event headers
func (c *SocketConnection) SendEvent(eventHeaders []string) error {
	if len(eventHeaders) <= 0 {
		return fmt.Errorf(ECouldNotSendEvent, len(eventHeaders))
	}
	_, err := io.WriteString(c, "sendevent ")
	if err != nil {
		return err
	}
	for _, eventHeader := range eventHeaders {
		_, err := io.WriteString(c, eventHeader)
		if err != nil {
			return err
		}
		_, err = io.WriteString(c, "\r\n")
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(c, "\r\n")
	if err != nil {
		return err
	}
	return nil
}

// Execute - Helper fuck to execute commands with its args and sync/async mode
func (c *SocketConnection) Execute(command, args string, sync bool) (err error) {
	return c.SendMsg(map[string]string{
		"call-command":     "execute",
		"execute-app-name": command,
		"execute-app-arg":  args,
		"event-lock":       strconv.FormatBool(sync),
	}, "", "")
}

// ExecuteUUID - Helper fuck to execute uuid specific commands with its args and sync/async mode
func (c *SocketConnection) ExecuteUUID(uuid string, command string, args string, sync bool) (err error) {
	return c.SendMsg(map[string]string{
		"call-command":     "execute",
		"execute-app-name": command,
		"execute-app-arg":  args,
		"event-lock":       strconv.FormatBool(sync),
	}, uuid, "")
}

// SendMsg - Basically this func will send message to the opened connection
func (c *SocketConnection) SendMsg(msg map[string]string, uuid, data string) (err error) {
	b := bytes.NewBufferString("sendmsg")
	if uuid != "" {
		if strings.Contains(uuid, "\r\n") {
			return fmt.Errorf(EInvalidCommandProvided, msg)
		}
		b.WriteString(" " + uuid)
	}
	b.WriteString("\n")
	for k, v := range msg {
		if strings.Contains(k, "\r\n") {
			return fmt.Errorf(EInvalidCommandProvided, msg)
		}
		if v != "" {
			if strings.Contains(v, "\r\n") {
				return fmt.Errorf(EInvalidCommandProvided, msg)
			}
			b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	b.WriteString("\n")
	if msg["content-length"] != "" && data != "" {
		b.WriteString(data)
	}
	// lock mutex
	_, err = b.WriteTo(c)
	return err
}

// ReadMessage - Will read message from channels and return them back accordingy.
//  If error is received, error will be returned. If not, message will be returned back!
func (c *SocketConnection) ReadMessage() (*Message, error) {
	// Debug("Waiting for connection message to be received ...")
	select {
	case err := <-c.err:
		return nil, err
	case msg := <-c.m:
		return msg, nil
	}
}

// Handle - Will handle new messages and close connection when there are no messages left to process
func (c *SocketConnection) Handle() {
	rbuf := bufio.NewReaderSize(c, ReadBufferSize)
	for {
		msg, err := NewMessage(rbuf, true)
		// Debug("handleMsg====> %v", msg)
		if err != nil {
			c.err <- err
			break
		}
		c.m <- msg
	}
	// Closing the connection now as there's nothing left to do ...
	c.Close()
}

// Close - Will close down net connection and return error if error happen
func (c *SocketConnection) Close() error {
	return c.Conn.Close()
}

// BgApi - Helper designed to attach api in front of the command so that you do not need to write it
func (sc *SocketConnection) Api(command string) error {
	return sc.Send("api " + command)
}

// BgApi - Helper designed to attach bgapi in front of the command so that you do not need to write it
func (sc *SocketConnection) BgApi(command string) error {
	return sc.Send("bgapi " + command)
}

// Connect - Helper designed to help you handle connection. Each outbound server when handling needs to connect e.g. accept
// connection in order for you to do answer, hangup or do whatever else you wish to do
func (sc *SocketConnection) Connect() error {
	return sc.Send("connect")
}

// Exit - Used to send exit signal to ESL. It will basically hangup call and close connection
func (sc *SocketConnection) Exit() error {
	return sc.Send("exit")
}
