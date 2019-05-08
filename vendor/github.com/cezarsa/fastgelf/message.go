package fastgelf

import (
	"github.com/francoispqt/gojay"
)

type Message struct {
	Version  string
	Host     string
	Short    string
	Full     string
	TimeUnix float64
	Level    int32
	Facility string
	Extra    map[string]interface{}
	RawExtra []byte
}

func (m *Message) IsNil() bool {
	return m == nil
}

func (m *Message) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("version", m.Version)
	enc.StringKey("host", m.Host)
	enc.StringKey("short_message", m.Short)
	enc.StringKeyOmitEmpty("full_message", m.Full)
	enc.Float64Key("timestamp", m.TimeUnix)
	enc.Int32KeyOmitEmpty("level", m.Level)
	enc.StringKeyOmitEmpty("facility", m.Facility)
	for k, v := range m.Extra {
		enc.AddInterfaceKey(k, v)
	}
	if len(m.RawExtra) > 1 {
		buf := gojay.EmbeddedJSON(m.RawExtra[1 : len(m.RawExtra)-1])
		enc.AddEmbeddedJSON(&buf)
	}
}
