package tcp

import (
	"context"
	"errors"
	"github.com/project-flogo/core/data/metadata"
	"github.com/project-flogo/core/support/log"
	"github.com/project-flogo/core/trigger"
	"net"
	"strings"
	"sync"
	"time"
)

var triggerMd = trigger.NewMetadata(&Settings{}, &HandlerSettings{}, &Output{}, &Reply{})

func init() {
	_ = trigger.Register(&Trigger{}, &Factory{})
}

// Factory is a trigger factory
type Factory struct {
}

// Metadata implements trigger.Factory.Metadata
func (*Factory) Metadata() *trigger.Metadata {
	return triggerMd
}

// New implements trigger.Factory.New
func (*Factory) New(config *trigger.Config) (trigger.Trigger, error) {
	s := &Settings{}
	err := metadata.MapToStruct(config.Settings, s, true)
	if err != nil {
		return nil, err
	}

	return &Trigger{settings: s}, nil
}

// Trigger is a trigger
type Trigger struct {
	settings    *Settings
	handlers    []trigger.Handler
	listener    net.Listener
	logger      log.Logger
	connections []*connection
	lock        sync.Mutex
}

type connection struct {
	Conn net.Conn
	Frm  *Frame
}

// Initialize initializes the trigger
func (t *Trigger) Initialize(ctx trigger.InitContext) error {

	host := t.settings.Host
	port := t.settings.Port
	t.handlers = ctx.GetHandlers()
	t.logger = ctx.Logger()

	if port == "" {
		return errors.New("Valid port must be set")
	}

	listener, err := net.Listen(t.settings.Network, host+":"+port)
	if err != nil {
		return err
	}

	t.listener = listener

	return err
}

// Start starts the trigger
func (t *Trigger) Start() error {

	go t.waitForConnection()
	t.logger.Infof("Started listener on - %s:%s, Network - %s",
		t.settings.Host, t.settings.Port, t.settings.Network)
	return nil
}

func (t *Trigger) waitForConnection() {
	for {
		// Listen for an incoming connection.
		conn, err := t.listener.Accept()
		if err != nil {
			errString := err.Error()
			if !strings.Contains(errString, "use of closed network connection") {
				t.logger.Error("Error accepting connection: ", err.Error())
			}
			return
		} else {
			t.logger.Debugf("Handling new connection from client - %s", conn.RemoteAddr().String())
			// Handle connections in a new goroutine.
			go t.handleNewConnection(conn)
		}
	}
}

func (t *Trigger) handleNewConnection(conn net.Conn) {
	connDesc := conn.RemoteAddr().String()
	defer func() {
		t.lock.Lock()
		for idx, item := range t.connections {
			if item.Conn == conn {
				t.connections = append(t.connections[:idx], t.connections[idx+1:]...)
				break
			}
		}
		t.lock.Unlock()
		_ = conn.Close()
		t.logger.Debugf("[%s] disconnect from client", connDesc)
	}()

	frm := &Frame{
		FrameMaxSize: 1024 * 1024 * 10,
		HeaderSize:   4,
		GetPayloadSize: func(header []byte) int {
			// Big endian format
			var dataLength uint32
			dataLength = 0
			for _, v := range header {
				dataLength = (dataLength << 8) | (uint32(v) & 0x000000FF)
			}
			return int(dataLength)
		},
	}

	{
		con := &connection{
			Conn: conn,
			Frm:  frm,
		}
		_ = con.Frm.Initialize()
		t.lock.Lock()
		t.connections = append(t.connections, con)
		t.lock.Unlock()
	}

	for {
		if t.settings.TimeOut > 0 {
			conn.SetDeadline(time.Now().Add(time.Duration(t.settings.TimeOut) * time.Millisecond))
		}

		buf := make([]byte, 4096)
		rlen, err := conn.Read(buf[:])
		if err != nil {
			if nerr, ok := err.(*net.OpError); ok && nerr.Timeout() {
				// timeout
			} else {
				t.logger.Warnf("[%s] Read with error %s", connDesc, err.Error())
				return
			}
		} else if rlen > 0 {
			err = frm.ByteToFrame(buf[:rlen], func(header []byte, payload []byte) {
				output := &Output{}
				output.Data = payload
				for i := 0; i < len(t.handlers); i++ {
					_, err := t.handlers[i].Handle(context.Background(), output)
					if err != nil {
						t.logger.Warnf("[%s] Error invoking action : ", connDesc, err.Error())
						continue
					}
				}
			})
			if err != nil {
				t.logger.Warnf("[%s] framing with error %s", connDesc, err.Error())
				return
			}
		}
	}
}

// Stop implements ext.Trigger.Stop
func (t *Trigger) Stop() error {

	for i := 0; i < len(t.connections); i++ {
		t.connections[i].Conn.Close()
		t.connections[i].Frm.Destroy()
	}

	t.connections = nil

	if t.listener != nil {
		t.listener.Close()
	}

	t.logger.Info("Stopped listener")

	return nil
}
