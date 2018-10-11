package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"runtime"
	"time"

	colorable "github.com/mattn/go-colorable"
	syrup "github.com/mkishere/sshsyrup"
	honeyos "github.com/mkishere/sshsyrup/os"
	_ "github.com/mkishere/sshsyrup/os/command"
	"github.com/mkishere/sshsyrup/util"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	configPath string
)

func init() {
	pflag.StringVarP(&configPath, "config", "c", ".", "Specify the working directory")

	viper.SetDefault("server.addr", "0.0.0.0")
	viper.SetDefault("server.port", 2222)
	viper.SetDefault("server.allowRandomUser", true)
	viper.SetDefault("server.ident", "SSH-2.0-OpenSSH_6.8p1")
	viper.SetDefault("server.maxTries", 3)
	viper.SetDefault("server.allowRetryLogin", false)
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
		err = os.MkdirAll("logs/sessions", 0755)
		if err != nil {
			os.Exit(1)
		}
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

	key, err := ioutil.ReadFile(path.Join(configPath, viper.GetString("server.privateKey")))
	if err != nil {
		log.WithError(err).Fatal("Failed to load private key")
	}

	syrupServer := syrup.NewServer(configPath, key)

	syrupServer.ListenAndServe()

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
