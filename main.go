package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/imdario/mergo"

	"golang.org/x/crypto/ssh"
)

// Config type is a map for storing config values
type Config struct {
	SvrAddr          string            `json:"server.addr"`
	SvrPort          int               `json:"server.port"`
	SvrAllowRndUser  bool              `json:"server.allowRandomUser"`
	SvrVer           string            `json:"server.ident"`
	SvrMaxTries      int               `json:"server.maxTries"`
	SvrMaxConn       int               `json:"server.maxConnections"`
	SvrUserList      map[string]string `json:"server.userList"`
	SvrLogFilename   string            `json:"server.logFilename"`
	SvrTimeout       time.Duration     `json:"server.Timeout"`
	AcinemaLogPrefix string            `json:"asciinema.logfileprefix"`
	AcinemaAPIEndPt  string            `json:"asciinema.apiEndpoint"`
	AcinemaAPIKey    string            `json:"asciinema.apiKey"`
}

func main() {

	defaultCfg := Config{
		SvrAddr:         "0.0.0.0",
		SvrPort:         22,
		SvrAllowRndUser: true,
		SvrVer:          "SSH-2.0-OpenSSH_6.8p1",
		SvrMaxTries:     3,
		SvrMaxConn:      20,
		SvrUserList: map[string]string{
			"testuser": "tiger",
		},
		SvrLogFilename:   "test.log",
		SvrTimeout:       time.Duration(time.Minute * 10),
		AcinemaLogPrefix: "test",
		AcinemaAPIEndPt:  "https://asciinema.org",
	}

	// Read config
	config := loadConfiguration("config.json")
	mergo.Merge(&config, defaultCfg)

	// Read banner
	bannerFile, err := ioutil.ReadFile("banner.txt")
	if err != nil {
		bannerFile = []byte{}
	}

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	sshConfig := &ssh.ServerConfig{

		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			log.Printf("User [%v] trying to login with password \"%v\"", c.User(), string(pass))
			if stpass, exists := config.SvrUserList[c.User()]; exists && (stpass == string(pass) || stpass == "*") || config.SvrAllowRndUser {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"permit-agent-forwarding": "no",
					},
				}, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		ServerVersion: config.SvrVer,

		BannerCallback: func(c ssh.ConnMetadata) string {
			return string(bannerFile)
		},
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	sshConfig.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be
	// accepted.

	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", config.SvrAddr, config.SvrPort))
	defer listener.Close()
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}

	for {
		nConn, err := listener.Accept()
		defer nConn.Close()
		if err != nil {
			log.Printf("failed to accept incoming connection: %v", err)
			continue
		}

		sshSession, err := NewSSHSession(nConn, sshConfig, config)
		if err != nil {
			log.Printf("Error establising SSH connection")
		}
		go sshSession.handleNewConn()
	}

}

func loadConfiguration(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		log.Println(err.Error())
	}

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return config
}