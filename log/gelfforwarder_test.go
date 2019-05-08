package log

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/cezarsa/fastgelf"
	"github.com/tsuru/bs/bslog"
)

func newBackend(t testing.TB) *gelfBackend {
	bslog.Logger = log.New(ioutil.Discard, "", log.LstdFlags)
	b := &gelfBackend{}
	b.setup()
	return b
}

func BenchmarkGelfBackendParseFieldsNoFields(b *testing.B) {
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "no field to parse with a modest size log message half full of stuff in it",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}

func BenchmarkGelfBackendInvalidFields(b *testing.B) {
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "invalid fields to parse invalid1=a invalid2=b invalid3=c in4=d inva=EMERG",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}

func BenchmarkGelfBackendValidFields(b *testing.B) {
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "w fields request_id=a request_time=1 request_uri=/ status=200 level=EMERG",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}

func BenchmarkGelfBackendHugeValidFields(b *testing.B) {
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields request_id=a request_time=1 request_uri=/ status=200 method=GET " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields ",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}

func BenchmarkGelfBackendHugeValidFieldsBigWhitelist(b *testing.B) {
	os.Setenv("LOG_GELF_FIELDS_WHITELIST", "request_id,request_time,request_uri,status,method,uri,type,uid,request_size,a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t")
	defer os.Unsetenv("LOG_GELF_FIELDS_WHITELIST")
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields request_id=a request_time=1 request_uri=/ status=200 method=GET " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields " +
			"my big message with many fields my big message with many fields my big message with many fields ",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}

func BenchmarkGelfBackendHugeInvalid(b *testing.B) {
	os.Setenv("LOG_GELF_FIELDS_WHITELIST", "request_id,request_time,request_uri,status,method,uri,type,uid,request_size,a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t")
	defer os.Unsetenv("LOG_GELF_FIELDS_WHITELIST")
	b.StopTimer()
	backend := newBackend(b)
	msg := &fastgelf.Message{
		Short: "mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"mybigmessagewithmanyfieldsmybigmessagewithmanyfieldsmybigmessagewithmanyfields" +
			"a=a",
		Extra: map[string]interface{}{},
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		backend.parseFields(msg)
	}
	b.StopTimer()
}
