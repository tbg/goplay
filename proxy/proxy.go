package proxy

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgproto3/v2"
)

type Options struct {
	IncomingTLSConfig *tls.Config
	OutgoingTLSConfig *tls.Config

	// TODO(tbg): this is unimplemented and exists only to check which clients
	// allow use of SNI. Should always return ("", nil).
	OutgoingAddrFromSNI    func(serverName string) (addr string, clientErr error)
	OutgoingAddrFromParams func(map[string]string) (addr string, clientErr error)

	_ struct{} // force explicit init of this struct
}

func sendErr(conn net.Conn, msg string) {
	_, _ = conn.Write((&pgproto3.ErrorResponse{
		Severity: "FATAL",
		Code:     "08004", // rejected connection
		Message:  msg + ", see http://cloud.todo.com/topics/connection-failed",
	}).Encode(nil))
}

func Proxy(conn net.Conn, opts Options) error {
	{
		m, err := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn).ReceiveStartupMessage()
		if err != nil {
			return errors.Wrap(err, "while receiving startup message")
		}
		_, ok := m.(*pgproto3.SSLRequest)
		if !ok {
			sendErr(conn, "server requires encryption")
			return errors.Newf("unsupported startup message: %T", m)
		}

		_, err = conn.Write([]byte("S"))
		if err != nil {
			return errors.Wrap(err, "allowing SSLRequest")
		}

		cfg := opts.IncomingTLSConfig.Clone()
		var sniServerName string
		cfg.GetConfigForClient = func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			sniServerName = h.ServerName
			return nil, nil
		}
		if opts.OutgoingAddrFromSNI != nil {
			addr, clientErr := opts.OutgoingAddrFromSNI(sniServerName)
			if clientErr != nil {
				sendErr(conn, clientErr.Error()) // won't actually be interpreted by most clients
				return errors.Wrap(err, "rejected by OutgoingAddrFromSNI")
			}
			if addr != "" {
				return errors.Newf("OutgoingAddrFromSNI is unimplemented")
			}
		}
		conn = tls.Server(conn, cfg)
	}

	m, err := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn).ReceiveStartupMessage()
	if err != nil {
		return errors.Wrap(err, "receiving post-TLS startup message")
	}
	msg, ok := m.(*pgproto3.StartupMessage)
	if !ok {
		return errors.Newf("unsupported post-TLS startup message: %T", m)
	}

	outgoingAddr, clientErr := opts.OutgoingAddrFromParams(msg.Parameters)
	if clientErr != nil {
		sendErr(conn, clientErr.Error())
		return errors.Wrap(clientErr, "rejected by OnClientInfo")
	}

	crdbConn, err := net.Dial("tcp", outgoingAddr)
	if err != nil {
		sendErr(conn, "unable to reach backend SQL server")
		return errors.Wrap(err, "dialing target server")
	}

	// Send SSLRequest.
	if err := binary.Write(crdbConn, binary.BigEndian, []int32{8, 80877103}); err != nil {
		return errors.Wrap(err, "sending SSLRequest to target server")
	}

	response := make([]byte, 1)
	if _, err = io.ReadFull(crdbConn, response); err != nil {
		return errors.Wrap(err, "reading response to SSLRequest")
	}

	if response[0] != 'S' {
		return errors.Newf("target server refused TLS connection")
	}

	crdbConn = tls.Client(crdbConn, opts.OutgoingTLSConfig)

	if _, err := crdbConn.Write(msg.Encode(nil)); err != nil {
		return errors.Wrap(err, "relaying StartupMessage to target server")
	}

	errOutgoing := make(chan error)
	errIncoming := make(chan error)

	go func() {
		_, err := io.Copy(crdbConn, conn)
		errOutgoing <- err
	}()
	go func() {
		_, err := io.Copy(conn, crdbConn)
		errIncoming <- err
	}()

	select {
	// NB: when using pgx, we see a nil errIncoming first on clean connection
	// termination. Using psql I see a nil errOutgoing first. I think the PG
	// protocol stipulates sending a message to the server at which point
	// the server closes the connection (errIncoming), but presumably the
	// client gets to close the connection once it's sent that message,
	// meaning either case is possible.
	case err := <-errIncoming:
		return errors.Wrap(err, "copying from target server to client")
	case err := <-errOutgoing:
		// The incoming connection got closed.
		return errors.Wrap(err, "copying from target server to client")
	}
}
