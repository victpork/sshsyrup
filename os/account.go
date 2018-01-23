package os

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type User struct {
	UID      int
	GID      int
	Name     string
	Password string
	Homedir  string
	Info     string
	Shell    string
}

type Group struct {
	GID      int
	Name     string
	Userlist []string
}

var (
	users  = make(map[int]User)
	groups = make(map[int]Group)
)

func LoadUsers(userFile string) error {
	f, err := os.OpenFile(userFile, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			return err
		}
		users[uid] = User{
			UID:      uid,
			GID:      gid,
			Name:     fields[0],
			Password: fields[1],
			Info:     fields[4],
			Homedir:  fields[5],
			Shell:    fields[6],
		}
	}

	return nil
}

func LoadGroups(groupFile string) error {
	f, err := os.OpenFile(groupFile, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			return err
		}
		groups[gid] = Group{
			GID:  gid,
			Name: fields[0],
		}
	}

	return nil
}
