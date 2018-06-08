package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	colorable "github.com/mattn/go-colorable"
	honeyos "github.com/mkishere/sshsyrup/os"
	_ "github.com/mkishere/sshsyrup/os/command"
	"github.com/mkishere/sshsyrup/util"
	"github.com/mkishere/sshsyrup/virtualfs"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

const (
	logTimeFormat string = "20060102"
)

var (
	vfs        afero.Fs
	configPath string
	ipConnCnt  struct {
		sync.RWMutex
		m map[string]int
	}
)

func init() {
	pflag.StringVarP(&configPath, "config", "c", ".", "Specify the working directory")

	viper.SetDefault("server.addr", "0.0.0.0")
	viper.SetDefault("server.port", 2222)
	viper.SetDefault("server.allowRandomUser", true)
	viper.SetDefault("server.ident", "SSH-2.0-OpenSSH_6.8p1")
	viper.SetDefault("server.maxTries", 3)
	viper.SetDefault("server.maxConnections", 10)
	viper.SetDefault("server.maxConnPerHost", 2)
	viper.SetDefault("server.timeout", time.Duration(time.Minute*10))
	viper.SetDefault("server.speed", 0)
	viper.SetDefault("server.processDelay", 0)
	viper.SetDefault("server.hostname", "spr1139")
	viper.SetDefault("server.commandList", "commands.txt")
	viper.SetDefault("server.sessionLogFmt", "asciinema")
	viper.SetDefault("server.banner", "banner.txt")
	viper.SetDefault("server.privateKey", "id_rsa")
	viper.SetDefault("server.portRedirection", "disable")
	viper.SetDefault("server.commandOutputDir", "cmdOutput")
	viper.SetDefault("virtualfs.imageFile", "filesystem.zip")
	viper.SetDefault("virtualfs.uidMappingFile", "passwd")
	viper.SetDefault("virtualfs.gidMappingFile", "group")
	viper.SetDefault("virtualfs.savedFileDir", "tempdir")
	viper.SetDefault("asciinema.apiEndpoint", "https://asciinema.org")
}

func main() {
	pflag.Parse()
	viper.SetEnvPrefix("sshsyrup")
	viper.AddConfigPath(configPath)
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find config file at %v", configPath)
		return
	}

	viper.AutomaticEnv()
	if runtime.GOOS == "windows" {
		log.SetFormatter(&log.TextFormatter{ForceColors: true})
		log.SetOutput(colorable.NewColorableStdout())
	}
	pathMap := lfshook.PathMap{
		log.InfoLevel: "logs/activity.log",
	}
	if _, err = os.Stat("logs"); os.IsNotExist(err) {
		os.MkdirAll("logs/sessions", 0755)
	}
	log.AddHook(lfshook.NewHook(
		pathMap,
		&log.JSONFormatter{},
	))

	// See if logstash is enabled
	if viper.IsSet("elastic.endPoint") {

		hook := util.NewElasticHook(viper.GetString("elastic.endPoint"), viper.GetString("elastic.index"), viper.GetString("elastic.pipeline"))
		if err != nil {
			log.WithError(err).Fatal("Cannot hook with Elastic")
		}
		log.AddHook(hook)
	}
	// Init connection count map
	ipConnCnt = struct {
		sync.RWMutex
		m map[string]int
	}{m: make(map[string]int)}

	// Initalize VFS
	// ID Mapping

	backupFS := afero.NewBasePathFs(afero.NewOsFs(), viper.GetString("virtualfs.savedFileDir"))
	zipfs, err := virtualfs.NewVirtualFS(path.Join(configPath, viper.GetString("virtualfs.imageFile")))
	if err != nil {
		log.Error("Cannot create virtual filesystem")
	}
	vfs = afero.NewCopyOnWriteFs(zipfs, backupFS)
	err = honeyos.LoadUsers(path.Join(configPath, viper.GetString("virtualfs.uidMappingFile")))
	if err != nil {
		log.Errorf("Cannot load user mapping file %v", path.Join(configPath, viper.GetString("virtualfs.uidMappingFile")))
	}

	err = honeyos.LoadGroups(path.Join(configPath, viper.GetString("virtualfs.uidMappingFile")))
	if err != nil {
		log.Errorf("Cannot load group mapping file %v", path.Join(configPath, viper.GetString("virtualfs.uidMappingFile")))
	}
	// Load command list
	honeyos.RegisterFakeCommand(readFiletoArray(path.Join(configPath, viper.GetString("server.commandList"))))
	// Load command output list
	cmdOutputPath := viper.GetString("server.commandOutputDir")
	if dp, err := os.Open(cmdOutputPath); err == nil {
		fileList, err := dp.Readdir(-1)
		if err == nil {
			for _, fi := range fileList {
				if !fi.IsDir() {
					honeyos.RegisterCommandOutput(fi.Name(), path.Join(cmdOutputPath, fi.Name()))
				}
			}
		}
	}
	// Randomize seed
	rand.Seed(time.Now().Unix())

	// Read banner
	bannerFile, err := ioutil.ReadFile(path.Join(configPath, viper.GetString("server.banner")))
	if err != nil {
		bannerFile = []byte{}
	}
	sshConfig := &ssh.ServerConfig{

		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			clientIP, port, _ := net.SplitHostPort(c.RemoteAddr().String())
			log.WithFields(log.Fields{
				"user":       c.User(),
				"srcIP":      clientIP,
				"port":       port,
				"authMethod": "password",
				"password":   string(pass),
			}).Info("User trying to login with password")

			if stpass, exists := honeyos.IsUserExist(c.User()); exists && (stpass == string(pass) || stpass == "*") || viper.GetBool("server.allowRandomUser") {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"permit-agent-forwarding": "yes",
					},
				}, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			clientIP, port, _ := net.SplitHostPort(c.RemoteAddr().String())
			log.WithFields(log.Fields{
				"user":              c.User(),
				"srcIP":             clientIP,
				"port":              port,
				"pubKeyType":        key.Type(),
				"pubKeyFingerprint": base64.StdEncoding.EncodeToString(key.Marshal()),
				"authMethod":        "publickey",
			}).Info("User trying to login with key")
			return nil, errors.New("Key rejected, revert to password login")
		},

		ServerVersion: viper.GetString("server.ident"),

		BannerCallback: func(c ssh.ConnMetadata) string {
			return string(bannerFile)
		},
	}

	privateBytes, err := ioutil.ReadFile(path.Join(configPath, viper.GetString("server.privateKey")))
	if err != nil {
		log.WithError(err).Fatal("Failed to load private key")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse private key")
	}

	sshConfig.AddHostKey(private)

	connChan := make(chan net.Conn)
	// Create pool of workers to handle connections
	for i := 0; i < viper.GetInt("server.maxConnections"); i++ {
		go createSessionHandler(connChan, sshConfig)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%v:%v", viper.GetString("server.addr"), viper.GetInt("server.port")))
	if err != nil {
		log.WithError(err).Fatal("Could not create listening socket")
	}
	defer listener.Close()

	for {
		nConn, err := listener.Accept()
		host, port, _ := net.SplitHostPort(nConn.RemoteAddr().String())
		log.WithFields(log.Fields{
			"srcIP": host,
			"port":  port,
		}).Info("Connection established")
		if err != nil {
			log.WithError(err).Error("Failed to accept incoming connection")
			continue
		}
		ipConnCnt.RLock()
		cnt := ipConnCnt.m[host]
		ipConnCnt.RUnlock()
		if cnt >= viper.GetInt("server.maxConnPerHost") {
			nConn.Close()
			continue
		} else {
			ipConnCnt.Lock()
			ipConnCnt.m[host] = cnt + 1
			ipConnCnt.Unlock()
		}
		tConn := NewThrottledConnection(nConn, viper.GetInt64("server.speed"), viper.GetDuration("server.timeout"))

		connChan <- tConn
	}

}

func loadIDMapping(file string) (m map[int]string) {
	m = map[int]string{0: "root"}
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	defer f.Close()
	if err != nil {
		return
	}
	buf := bufio.NewScanner(f)
	linenum := 1
	for buf.Scan() {
		fields := strings.Split(buf.Text(), ":")
		id, err := strconv.ParseInt(fields[2], 10, 32)
		if err != nil {
			log.Errorf("Cannot parse mapping file %v line %v", file, linenum)
			continue
		}
		m[int(id)] = fields[0]
		linenum++
	}
	return
}

func readFiletoArray(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return []string{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func createDelayFunc(base, r int) func() {
	return func() {
		sleepTime := base - r + rand.Intn(2*r)
		time.Sleep(time.Millisecond * time.Duration(sleepTime))
	}
}
