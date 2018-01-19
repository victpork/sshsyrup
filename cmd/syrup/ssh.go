package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	os "github.com/mkishere/sshsyrup/os"
	"github.com/mkishere/sshsyrup/util/termlogger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// SSHSession stores SSH session info
type SSHSession struct {
	user          string
	src           net.Addr
	clientVersion string
	activity      chan bool
	sshChan       <-chan ssh.NewChannel
	ptyReq        *ptyRequest
	log           *log.Entry
}

type envRequest struct {
	Name  string
	Value string
}

type ptyRequest struct {
	Term    string
	Width   uint32
	Height  uint32
	PWidth  uint32
	PHeight uint32
	Modes   string
}
type winChgRequest struct {
	Width  uint32
	Height uint32
}

type tunnelRequest struct {
	RemoteHost string
	RemotePort uint32
	LocalHost  string
	LocalPort  uint32
}

// NewSSHSession create new SSH connection based on existing socket connection
func NewSSHSession(nConn net.Conn, sshConfig *ssh.ServerConfig) (*SSHSession, error) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, sshConfig)
	if err != nil {
		return nil, err
	}

	logger := log.WithFields(log.Fields{
		"user":      conn.User(),
		"srcIP":     conn.RemoteAddr().String(),
		"clientStr": string(conn.ClientVersion()),
		"sessionId": base64.StdEncoding.EncodeToString(conn.SessionID()),
	})
	logger.Infof("New SSH connection with client")

	activity := make(chan bool)
	go func(activity chan bool) {
		defer nConn.Close()
		for range activity {
			// When receive from activity channel, reset deadline
			nConn.SetReadDeadline(time.Now().Add(config.SvrTimeout))
		}
	}(activity)

	go ssh.DiscardRequests(reqs)
	return &SSHSession{
		user:          conn.User(),
		src:           conn.RemoteAddr(),
		clientVersion: string(conn.ClientVersion()),
		activity:      activity,
		sshChan:       chans,
		log:           logger,
	}, nil
}

func (s *SSHSession) handleNewSession(newChan ssh.NewChannel) {

	channel, requests, err := newChan.Accept()
	if err != nil {
		s.log.WithError(err).Error("Could not accept channel")
		return
	}
	var sh *os.Shell
	go func(in <-chan *ssh.Request, channel ssh.Channel) {
		quitSignal := make(chan int, 1)
		for {
			select {
			case req := <-in:
				switch req.Type {
				case "winadj@putty.projects.tartarus.org", "simple@putty.projects.tartarus.org":
					//Do nothing here
				case "pty-req":
					// Of coz we are not going to create a PTY here as we are honeypot.
					// We are creating a pseudo-PTY
					var ptyreq ptyRequest
					if err := ssh.Unmarshal(req.Payload, &ptyreq); err != nil {
						s.log.WithField("reqType", req.Type).WithError(err).Errorln("Cannot parse user request payload")
						req.Reply(false, nil)
					} else {
						s.log.WithField("reqType", req.Type).Infof("User requesting pty(%v %vx%v)", ptyreq.Term, ptyreq.Width, ptyreq.Height)
						s.ptyReq = &ptyreq
						req.Reply(true, nil)
					}
				case "env":
					var envReq envRequest
					if err := ssh.Unmarshal(req.Payload, &envReq); err != nil {
						req.Reply(false, nil)
					} else {
						s.log.WithFields(log.Fields{
							"reqType":     req.Type,
							"envVarName":  envReq.Name,
							"envVarValue": envReq.Value,
						}).Infof("User sends envvar:%v=%v", envReq.Name, envReq.Value)
						req.Reply(true, nil)
					}
				case "shell":
					s.log.WithField("reqType", req.Type).Info("User requesting shell access")
					if s.ptyReq == nil {
						s.ptyReq = &ptyRequest{
							Width:  80,
							Height: 24,
							Term:   "vt100",
						}
					}
					// The need of a goroutine here is that PuTTY will wait for reply before acknowledge it enters shell mode
					var hook termlogger.LogHook
					if config.SessionLogFmt == "asciinema" {
						asciiLogParams := map[string]string{
							"TERM": s.ptyReq.Term,
							"USER": s.user,
							"SRC":  s.src.String(),
						}
						hook, err = termlogger.NewAsciinemaHook(int(s.ptyReq.Width), int(s.ptyReq.Height),
							config.AcinemaAPIEndPt, config.AcinemaAPIKey, asciiLogParams)

					} else if config.SessionLogFmt == "uml" {
						hook, err = termlogger.NewUMLHook(0, fmt.Sprintf("logs/sessions/%v-%v.ulm.log", s.user, time.Now().Format(logTimeFormat)))
					} else {
						log.Errorf("Session Log option %v not recognized", config.SessionLogFmt)
					}
					if err != nil {
						log.Errorf("Cannot create %v log file", config.SessionLogFmt)
					}
					tLog := termlogger.NewLogger(hook, channel)
					defer tLog.Close()
					sh = os.NewShell(tLog, vfs, int(s.ptyReq.Width), int(s.ptyReq.Height), s.user, s.src.String(), s.log, quitSignal)
					go sh.HandleRequest()
					req.Reply(true, nil)
				case "subsystem":
					subsys := string(req.Payload[4:])
					s.log.WithFields(log.Fields{
						"reqType":   req.Type,
						"subSystem": subsys,
					}).Infof("User requested subsystem %v", subsys)
					req.Reply(true, nil)
				case "window-change":
					s.log.WithField("reqType", req.Type).Info("User shell window size changed")
					if sh != nil {
						var winChg *winChgRequest
						if err := ssh.Unmarshal(req.Payload, winChg); err != nil {
							req.Reply(false, nil)
						}
						sh.SetSize(int(winChg.Width), int(winChg.Height))
					}
				case "exec":
					cmd := string(req.Payload[4:])
					s.log.WithFields(log.Fields{
						"reqType": req.Type,
						"cmd":     cmd,
					}).Info("User request remote exec")
					channel.Write([]byte(fmt.Sprintf("%v: command not found\n", cmd)))
					quitSignal <- 0
					req.Reply(true, nil)
				default:
					s.log.WithField("reqType", req.Type).Infof("Unknown channel request type %v", req.Type)
				}
			case ret := <-quitSignal:
				defer closeChannel(channel, ret)
				return
			}
		}
	}(requests, channel)
}

func (s *SSHSession) handleNewConn() {
	// Service the incoming Channel channel.
	for newChannel := range s.sshChan {
		s.log.WithField("chanType", newChannel.ChannelType()).Info("User created new session channel")
		switch newChannel.ChannelType() {
		case "direct-tcpip", "forwarded-tcpip":
			var treq tunnelRequest
			err := ssh.Unmarshal(newChannel.ExtraData(), &treq)
			if err != nil {
				s.log.WithError(err).Error("Cannot unmarshal port forwarding data")
				newChannel.Reject(ssh.UnknownChannelType, "Corrupt payload")
			}
			s.log.WithFields(log.Fields{
				"remoteHost": fmt.Sprintf("%v:%v", treq.RemoteHost, treq.RemotePort),
				"localHost":  fmt.Sprintf("%v:%v", treq.LocalHost, treq.LocalPort),
				"chanType":   newChannel.ChannelType(),
			}).Info("Trying to establish connection with port forwarding")
			newChannel.Reject(ssh.Prohibited, "Port forwarding disabled")
		case "session":
			go s.handleNewSession(newChannel)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			s.log.WithField("chanType", newChannel.ChannelType()).Infof("Unknown channel type %v", newChannel.ChannelType())
			continue
		}
	}
}

func createSessionHandler(c <-chan net.Conn, sshConfig *ssh.ServerConfig) {
	for conn := range c {
		sshSession, err := NewSSHSession(conn, sshConfig)
		if err != nil {
			log.WithField("srcIP", conn.RemoteAddr().String()).WithError(err).Error("Error establishing SSH connection")
		}
		sshSession.handleNewConn()
		conn.Close()
	}
}

func closeChannel(ch ssh.Channel, signal int) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(signal))
	ch.SendRequest("exit-status", false, b)
	ch.Close()
}
