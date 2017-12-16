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

// Config is the struct to store cofiguations
type Config struct {
	AllowRandomUser bool
	Addr            string
	Port            int
	ServerIdent     string
	MaxTries        int
	UserList        map[string]string
	Logfile         string
	MaxConn         int
	Timeout         time.Duration
}

func main() {

	config := Config{
		Addr:            "0.0.0.0",
		Port:            22,
		AllowRandomUser: true,
		ServerIdent:     "SSH-2.0-Beague_1.0.0",
		MaxTries:        3,
		MaxConn:         20,
		UserList:        make(map[string]string),
		Logfile:         "test.log",
		Timeout:         time.Duration(time.Minute * 10),
	}
	config.UserList["testuser"] = "tiger"

	// Read config
	if _, err := os.Stat("config.json"); !os.IsNotExist(err) {
		configFile := loadConfiguration("config.json")
		if err = mergo.Merge(&config, configFile); err != nil {
			log.Fatalf("Cannot load configuration file!")
		}
	}
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
			if stpass, exists := config.UserList[c.User()]; exists && (stpass == string(pass) || stpass == "*") || config.AllowRandomUser {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"permit-agent-forwarding": "no",
					},
				}, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		ServerVersion: config.ServerIdent,

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

	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", config.Addr, config.Port))
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

/* func (term *terminal.Terminal) WriteString(s string) (int, error) {
	return term.Write([]byte(s))
} */

func loadConfiguration(file string) Config {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return config
}
