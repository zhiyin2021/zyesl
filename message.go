// Copyright 2015 Nevio Vesic
// Please check out LICENSE file for more information about what you CAN and what you CANNOT do!
// Basically in short this is a free software for you to do whatever you want to do BUT copyright must be included!
// I didn't write all of this code so you could say it's yours.
// MIT License

package kernel

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/zhiyin2021/zyesl/event"
)

var (
	AvailableMessageTypes = map[string]interface{}{
		"auth/request":           "",
		"text/disconnect-notice": "",
		"text/event-json":        "",
		"text/event-plain":       "",
		"api/response":           "",
		"command/reply":          "",
	}
	// Size of buffer when we read from connection.
	// 1024 << 6 == 65536
	ReadBufferSize = 1024 << 6

	// Freeswitch events that we can handle (have logic for it)

	EInvalidCommandProvided  = "Invalid command provided. Command cannot contain \\r and/or \\n. Provided command is: %s"
	ECouldNotReadMIMEHeaders = "Error while reading MIME headers: %s"
	EInvalidContentLength    = "Unable to get size of content-length: %s"
	EUnsuccessfulReply       = "Got error while reading from reply command: %s"
	ECouldNotReadyBody       = "Got error while reading reader body: %s"
	EUnsupportedMessageType  = "Unsupported message type! We got '%s'. Supported types are: %v "
	ECouldNotDecode          = "Could not decode/unescape message: %s"
	ECouldNotStartListener   = "Got error while attempting to start listener: %s"
	EListenerConnection      = "Listener connection error: %s"
	EInvalidServerAddr       = "Please make sure to pass along valid address. You've passed: \"%s\""
	EUnexpectedAuthHeader    = "Expected auth/request content type. Got %s"
	EInvalidPassword         = "Could not authenticate against freeswitch with provided password: %s"
	ECouldNotCreateMessage   = "Error while creating new message: %s"
	ECouldNotSendEvent       = "Must send at least one event header, detected `%d` header"
)

// Message - Freeswitch Message that is received by GoESL. Message struct is here to help with parsing message
// and dumping its contents. In addition to that it's here to make sure received message is in fact message we wish/can support
type Message struct {
	Headers         map[string]string
	Body            []byte
	Type            string
	r               *bufio.Reader
	tr              *textproto.Reader
	CallerDirection string
	CallerAddr      string
	UUID            string
	CallerNumber    string
	CallerName      string
	CalleeNumber    string
	State           string // answer-state
	EventName       string
	NetworkIP       string
	NetworkPort     string
	Data            string
}

// String - Will return message representation as string
func (m *Message) String() string {
	return fmt.Sprintf("%v body=%s", m.Headers, m.Body)
}

// GetStr - Will return message header value, or "" if the key is not set.
func (m *Message) GetStr(key string) string {
	return m.Headers[key]
}

// GetInt - Will return message header value, or "" if the key is not set.
func (m *Message) GetInt(key string) int {
	v := m.Headers[key]
	if n, e := strconv.Atoi(v); e == nil {
		return n
	}
	return 0
}

// Parse - Will parse out message received from Freeswitch and basically build it accordingly for later use.
// However, in case of any issues func will return error.
func (m *Message) Parse() error {
	cmr, err := m.tr.ReadMIMEHeader()
	if err != nil && err.Error() != "EOF" {
		logrus.Error(ECouldNotReadMIMEHeaders, err)
		return err
	}

	if cmr.Get("Content-Type") == "" {
		logrus.Debug("Not accepting message because of empty content type. Just whatever with it ...")
		data, err := io.ReadAll(m.r)
		return fmt.Errorf("Parse Err %v => %s", err, data)
	}

	// Will handle content length by checking if appropriate lenght is here and if it is than
	// we are going to read it into body
	if lv := cmr.Get("Content-Length"); lv != "" {
		l, err := strconv.Atoi(lv)

		if err != nil {
			logrus.Error(EInvalidContentLength, err)
			return err
		}

		m.Body = make([]byte, l)

		if _, err := io.ReadFull(m.r, m.Body); err != nil {
			logrus.Error(ECouldNotReadyBody, err)
			return err
		}
	}
	m.Type = cmr.Get("Content-Type")
	// Debug("Got message content (type: %s). Searching if we can handle it ...", m.Type)

	if _, ok := AvailableMessageTypes[m.Type]; !ok {
		return fmt.Errorf(EUnsupportedMessageType, m.Type, AvailableMessageTypes)
	}

	// Assing message headers IF message is not type of event-json
	if m.Type != "text/event-json" {
		for k, v := range cmr {
			k = strings.ToLower(k)
			m.Headers[k] = v[0]
			// Will attempt to decode if % is discovered within the string itself
			if strings.Contains(v[0], "%") {
				m.Headers[k], err = url.QueryUnescape(v[0])
				if err != nil {
					logrus.Error(ECouldNotDecode, err)
					continue
				}
			}
		}
		m.CallerAddr = m.Headers["caller-network-addr"]
		m.UUID = m.Headers["unique-id"]
		m.CallerName = m.Headers["caller-caller-id-name"]
		m.CallerNumber = m.Headers["caller-caller-id-number"]
		// m.CalleeNumber = m.Headers["caller-callee-id-number"]
		m.CalleeNumber = m.Headers["caller-destination-number"]
		m.CallerDirection = m.Headers["caller-direction"]
		m.State = m.Headers["answer-state"]
		m.EventName = m.Headers["event-name"]
		m.NetworkIP = m.Headers["variable_sip_network_ip"]
		m.NetworkPort = m.Headers["variable_sip_network_port"]

		if m.EventName == event.BACKGROUND_JOB {
			m.UUID = m.Headers["job-uuid"]
			if strings.Contains(string(m.Body), "-ERR") {
				m.State = "err"
				m.Data = strings.Trim(string(m.Body)[5:], "\n")
			} else if strings.Contains(string(m.Body), "-OK") {
				m.State = "ok"
				m.Data = strings.Trim(string(m.Body)[4:], "\n")
			}
		}
		// m.SipContact = fmt.Sprintf("%s:%s", m.Headers["variable_sip_contact_host"], m.Headers["variable_sip_contact_port"])
	}

	switch m.Type {
	case "text/disconnect-notice":
		for k, v := range cmr {
			logrus.Debugf("Message (header: %s) -> (value: %v)", k, v)
		}
	case "command/reply":
		reply := cmr.Get("Reply-Text")
		m.parseReply(reply)
	case "api/response":
		m.parseReply(string(m.Body))
	case "text/event-json":
		// OK, what is missing here is a way to interpret other JSON types - it expects string only (understandably
		// because the FS events are generally "string: string") - extract into empty interface and migrate only strings.
		// i.e. Event CHANNEL_EXECUTE_COMPLETE - "variable_DP_MATCH":["a=rtpmap:101 telephone-event/8000","101"]
		var decoded map[string]interface{}

		if err := json.Unmarshal(m.Body, &decoded); err != nil {
			return err
		}

		// Copy back in:
		for k, v := range decoded {
			k = strings.ToLower(k)
			// Debug("%s => (%v) %v", k, reflect.TypeOf(v), v)
			switch v := v.(type) {
			case string:
				{
					m.Headers[k] = v
				}
			default:
				//delete(m.Headers, k)
				// Warning("Removed non-string property (%s),%v", k, v)
			}
		}
		m.CallerAddr = m.Headers["caller-network-addr"]
		m.UUID = m.Headers["unique-id"]
		m.CallerName = m.Headers["caller-caller-id-name"]
		m.CallerNumber = m.Headers["caller-caller-id-number"]
		// m.CalleeNumber = m.Headers["caller-callee-id-number"]
		m.CalleeNumber = m.Headers["caller-destination-number"]
		m.CallerDirection = m.Headers["caller-direction"]
		m.State = m.Headers["answer-state"]
		m.EventName = m.Headers["event-name"]

		m.NetworkIP = m.Headers["variable_sip_network_ip"]
		m.NetworkPort = m.Headers["variable_sip_network_port"]

		// m.SipContact = fmt.Sprintf("%s:%s", m.Headers["variable_sip_contact_host"], m.Headers["variable_sip_contact_port"])
		if v := m.Headers["_body"]; v != "" {
			m.Body = []byte(v)
			delete(m.Headers, "_body")
		} else {
			m.Body = []byte("")
		}

		if m.EventName == event.BACKGROUND_JOB {
			m.UUID = m.Headers["job-uuid"]
			m.parseReply(string(m.Body))
		}
	case "text/event-plain":
		r := bufio.NewReader(bytes.NewReader(m.Body))

		tr := textproto.NewReader(r)

		emh, err := tr.ReadMIMEHeader()

		if err != nil {
			return fmt.Errorf(ECouldNotReadMIMEHeaders, err)
		}

		if vl := emh.Get("Content-Length"); vl != "" {
			length, err := strconv.Atoi(vl)

			if err != nil {
				logrus.Error(EInvalidContentLength, err)
				return err
			}

			m.Body = make([]byte, length)

			if _, err = io.ReadFull(r, m.Body); err != nil {
				logrus.Error(ECouldNotReadyBody, err)
				return err
			}
		}
	}
	return nil
}

// Dump - Will return message prepared to be dumped out. It's like prettify message for output
func (m *Message) Dump() (resp string) {
	var keys []string

	for k := range m.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		resp += fmt.Sprintf("%s: %s\r\n", k, m.Headers[k])
	}
	resp += fmt.Sprintf("BODY: %v\r\n", string(m.Body))
	return
}

func (m *Message) parseReply(reply string) {
	if strings.Contains(reply, "-ERR") {
		m.State = "err"
		if len(reply) > 5 {
			m.Data = strings.Trim(reply[5:], "\n")
		}
	} else if strings.Contains(reply, "-OK") || strings.Contains(reply, "+OK") {
		m.State = "ok"
		if len(reply) > 4 {
			m.Data = strings.Trim(reply[4:], "\n")
		}
	}
	m.UUID = m.GetStr("job-uuid")
}

// newMessage - Will build and execute parsing against received freeswitch message.
// As return will give brand new Message{} for you to use it.
func NewMessage(r *bufio.Reader, autoParse bool) (*Message, error) {
	msg := Message{
		r:       r,
		tr:      textproto.NewReader(r),
		Headers: make(map[string]string),
	}

	if autoParse {
		if err := msg.Parse(); err != nil {
			return &msg, err
		}
	}

	return &msg, nil
}
