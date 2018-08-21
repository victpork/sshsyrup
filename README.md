[![Build Status](https://travis-ci.org/mkishere/sshsyrup.svg?branch=master)](http://travis-ci.org/mkishere/sshsyrup) [![Build status](https://ci.appveyor.com/api/projects/status/iy271guyn7ig81yn/branch/master?svg=true)](https://ci.appveyor.com/project/mkishere/sshsyrup/branch/master)
# Syrup
A SSH honeypot with rich features written in Go

### Features
- SSH self-defined accounts and passwords, also allow any logins
- Fake shell. Records shell sessions and upload to [asciinema.org](https://asciinema.org) (Or, if you wish, can log as [UML-compatible](http://user-mode-linux.sourceforge.net/old/tty_logging.html) format)
- Virtual Filesystem for browsing and fooling intruder
- SFTP/SCP support for uploading/downloading files
- Logs client key fingerprints
- Logs in JSON format for easy parsing
- Push activities to [ElasticSearch](https://www.elastic.co) for analysis and storage
- Record local and remote host when client attempt to create port redirection
- Structure allows [extending command sets](https://github.com/mkishere/sshsyrup/wiki/Writing-new-commands) with ease

### See Recorded Session in Action!
[![asciicast](https://asciinema.org/a/yu8fdSXn6v9EV0ozdSjNNN5NJ.png)](https://asciinema.org/a/yu8fdSXn6v9EV0ozdSjNNN5NJ)

### Requirements
- Linux, Mac or Windows (I've only tested in Windows/WSL/Linux on ARMv7, suppose the other platforms should work as expected)
- Go 1.9 or up (For building)
- [dep](https://github.com/golang/dep) (For building)

### Download
You may find the pre-build packages for various platform on the [release](https://github.com/mkishere/sshsyrup/releases) tab. If you find the platform you need is not on the list, you can follow the building procedure in the next section.

### Building
Run the following command in shell to get latest package and build it
```
go get -u github.com/mkishere/sshsyrup
cd ~/go/src/github.com/mkishere/sshsyrup
dep ensure
go build -ldflags "-s -w" -o sshsyrup ./cmd/syrup
go build -ldflags "-s -w" -o createfs ./cmd/createfs
```

### Setting up for the first run
* Modify _config.yaml_. Here is a sample configuration
    ```yaml
    server:
        addr: 0.0.0.0           # Host IP
        port: 22                # Port listen to
        allowRandomUser: false  # Allow random user
        speed: 0                # Connection max speed in kb/s
        processDelay: 0         # Artifical delay after server returns responses in ms
        timeout: 0              # Connection timeout, 0 for none
    ```
* Prepare the virtual filesystem image by downloading the filesystem.zip from master branch or create your own by running
   ```
   ./createfs -p / -o filesystem.zip
   ```

   Since we'll need to read every file from the directory, it will take some time to load.
   _For Windows, since there are no user/group information, the file/directory owner will always be root._

   Alternatively, you can create your own image file by using `zip` in Linux (or any compatible zip utility file that is capable preserving _uid_/_gid_, symbolic links and timestamps in zip file). After all the image created is a standard zip file. Theoretically you can zip your entire filesystem into a zip file and hosted in Syrup, but remember to exclude sensitive files like `/etc/passwd`

* Prepare user and passwd file
Put _passwd_ and _group_ file in the same directory as config.json. The format of both files are the same as their [real-life counterpart](http://www.linfo.org/etc_passwd.html) in _/etc_, except that passwd also stores the password in the second field of each line, and asterisk(*) in password field can be used to denote matching any password.
* Generate SSH private key and renamed as _id\_rsa_ and put it in the same directory
   ```
   ssh-keygen -t rsa
   ```
* Start the server
   ```
   ./sshsyrup
   ```

### Running from a Docker instance

A Docker image based on the latest build:
```
  docker pull mkishere/sshsyrup
```

The current Dockerfile is a two-stage Dockerfile that will first compile sshsyrup and generate/copy the required files (`id_rsa`, `filesystem.zip`, `config.json`, sample `group` and `passwd` into a clean Docker image (based on [scratch](https://hub.docker.com/_/scratch/), so really lightweight (doesn't even have /bin/sh! :-)

This will generate a new image in your local computer repository. To run it, you will need to know first on which
port you want your instance to listen. By default (`config.json`),
the internal sshsyrup listens on 22. You do not need to change this. Just use the `-p` docker option to change
the externally listening port :

```sh
# Map the Syrup container port 22 to external port 9999
# But you may want to map to port 22 to make your honeypot easier to find
docker run -d -p 9999:22 sshsyrup
```

If you want to see what happens (logs) in the Docker instance, get the instance id (`docker ps`) and then
run `docker logs -f YOUR_INSTANCE_ID`.

### Configuration parameters
Check out [config.yaml](https://github.com/mkishere/sshsyrup/blob/master/config.yaml)
### Logging
By default Syrup will create a logging file in _logs/_ directory with file name _activity.log_ in JSON format.

Please note that Syrup will no longer append dates to log files. Use a proper log rotation tool (e.g. _logrotate_) to do the work.

Also, each terminal session (the shell) will be logged into a separate file under logs/sessions in [asciinema v2 format](https://github.com/asciinema/asciinema/blob/develop/doc/asciicast-v2.md).

### Extending Syrup
Syrup comes with a framework that helps to implement command easier. By implementing the [Command](https://github.com/mkishere/sshsyrup/blob/dfd91b14bd64f43e8100e3e0fbd6357f29b1708b/os/sys.go#L37) interface you can create your own command and being executed by intruders connecting to your honeypot. For more details refer to the [wiki](https://github.com/mkishere/sshsyrup/wiki/Writing-new-commands).

If your command prints static output every time, you can put the output in _cmdOutput/_, and Syrup will print that when client type the command in terminal.

### Contributing
Feel free to submit feature request/bug report via the GitHub issue tracker.

For submitting PR, do the following steps:
1. Fork
2. Create a branch for the feature/bugfix containing your changes on your fork
3. Submit PR with your branch

It is advised that creating an issue to discuss the matter in advance if your change is large :)

### TODO
- Minimal set of POSIX commands/utilities
- Port redirection
- Shell parser
