package tcp

import (
	"github.com/project-flogo/core/data/coerce"
)

type Settings struct {
	Network string `md:"network"`       // The network type
	Host    string `md:"host"`          // The host name or IP for TCP server.
	Port    string `md:"port,required"` // The port to listen on
	TimeOut int    `md:"timeout"`

	SSLCertificateFile string `md:"ssl_certificate_file"`
	SSLPrivateKeyFile  string `md:"ssl_private_key_file"`
	SSLVersion         string `md:"ssl_version"`
}

type HandlerSettings struct {
}

type Output struct {
	Data []byte `md:"data"` // The data received from the connection
}

type Reply struct {
	Reply []byte `md:"reply"` // The reply to be sent back
}

func (o *Output) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"data": o.Data,
	}
}

func (o *Output) FromMap(values map[string]interface{}) error {

	var err error
	o.Data, err = coerce.ToBytes(values["data"])
	if err != nil {
		return err
	}

	return nil
}

func (r *Reply) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"reply": r.Reply,
	}
}

func (r *Reply) FromMap(values map[string]interface{}) error {

	var err error
	r.Reply, err = coerce.ToBytes(values["reply"])
	if err != nil {
		return err
	}

	return nil
}
