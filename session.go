package kernel

type Session struct {
	uuid   string
	client *Client
}

func NewSession(uuid string, client *Client) *Session {
	return &Session{client: client, uuid: uuid}
}

func (s *Session) Set(key string, args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "set", key+"="+args, sync)
}

func (s *Session) Bridge(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "bridge", args, sync)
}
func (s *Session) Transfer(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "uuid_transfer", args, sync)
}

func (s *Session) Answer(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "answer", args, sync)
}

func (s *Session) QueueDtmf(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "queue_dtmf", args, sync)
}
func (s *Session) Eavesdrop(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "eavesdrop", args, sync)
}
func (s *Session) Hangup(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "hangup", args, sync)
}
func (s *Session) Playback(args FSound, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "playback", "$${sounds_dir}"+string(args), sync)
}
func (s *Session) PlaybackFile(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "playback", args, sync)
}
func (s *Session) Record(args string, sync bool) error {
	return s.client.ExecuteUUID(s.uuid, "record_session", args, sync)
}

func (s *Session) Break() error {
	return s.client.ExecuteUUID(s.uuid, "break", "", true)
}
func (s *Session) ExecuteUUID(uuid string, app string, args string, sync bool) error {
	return s.client.ExecuteUUID(uuid, app, args, sync)
}

func (s *Session) Execute(app string, args string, sync bool) error {
	return s.client.Execute(app, args, sync)
}
